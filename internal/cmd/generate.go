package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aaronlippold/go-split/internal/analyzer"
)

// GenerateResult holds generation results for JSON output.
type GenerateResult struct {
	SourceFile       string          `json:"source_file"`
	OutputDir        string          `json:"output_dir"`
	DryRun           bool            `json:"dry_run"`
	TestFile         string          `json:"test_file,omitempty"`
	Files            []GeneratedFile `json:"files"`
	ValidationPassed bool            `json:"validation_passed,omitempty"`
	ValidationError  string          `json:"validation_error,omitempty"`
}

// GeneratedFile describes a generated file.
type GeneratedFile struct {
	Name      string `json:"name"`
	Lines     int    `json:"lines"`
	Status    string `json:"status"` // "created", "failed", "skipped"
	Error     string `json:"error,omitempty"`
	TestCount int    `json:"test_count,omitempty"` // Number of tests in file (for test files)
}

// SplitPlan represents the AI's plan for splitting source and tests together.
type SplitPlan struct {
	Files []SplitFile `json:"files"`
}

// SplitFile represents a planned output file with its content assignment.
type SplitFile struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Functions   []string `json:"functions,omitempty"`
	Types       []string `json:"types,omitempty"`
	Tests       []string `json:"tests,omitempty"` // Test functions that belong with this file
}

// generateConfig holds generate-specific configuration.
type generateConfig struct {
	SkipTests      bool
	SkipValidation bool
}

var genCfg = &generateConfig{}

// newGenerateCmd creates the generate command.
func newGenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate <file>",
		Short: "Generate split files from a Go file",
		Long: `Generate split files based on AI analysis. The AI will determine
how to best split the file and generate the new files.

When a test file exists, the AI receives both source and tests together
to plan splits that maintain test coverage. When no tests exist, the AI
generates test stubs for each output file.`,
		Args: cobra.ExactArgs(1),
		RunE: runGenerate,
	}

	cmd.Flags().BoolVar(&genCfg.SkipTests, "skip-tests", false, "Skip test file splitting/generation")
	cmd.Flags().BoolVar(&genCfg.SkipValidation, "skip-validation", false, "Skip running go test after split")

	return cmd
}

func runGenerate(cmd *cobra.Command, args []string) error {
	ui := NewUI(cmd.OutOrStdout(), cfg.JSON)

	filename := args[0]
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", filename)
	}

	content, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	info, err := analyzer.ParseGoFile(filename)
	if err != nil {
		return fmt.Errorf("parsing file: %w", err)
	}

	outDir := cfg.OutputDir
	if outDir == "" {
		outDir = filepath.Dir(filename)
	}

	result := GenerateResult{
		SourceFile: filepath.Base(filename),
		OutputDir:  outDir,
		DryRun:     cfg.DryRun,
		Files:      []GeneratedFile{},
	}

	// Check for associated test file
	testFilePath := findTestFile(filename)
	var testContent []byte
	hasTests := false
	if testFilePath != "" && !genCfg.SkipTests {
		result.TestFile = filepath.Base(testFilePath)
		testContent, err = os.ReadFile(testFilePath)
		if err != nil {
			testContent = nil
		} else {
			hasTests = true
		}
	}

	ui.Header(fmt.Sprintf("📄 Splitting %s (%d lines)", filepath.Base(filename), info.Lines))

	if hasTests {
		testInfo, _ := analyzer.ParseGoFile(testFilePath)
		if testInfo != nil {
			testCount := countTestFunctions(testInfo)
			ui.Info(fmt.Sprintf("Found test file: %s (%d lines, %d tests) - will split alongside source", result.TestFile, testInfo.Lines, testCount))
		}
	} else if !genCfg.SkipTests {
		ui.Info("No test file found - will generate test stubs")
	}

	ui.StartSpinner("Planning split...")

	client := newAPIClient()

	// Build planning prompt with BOTH source and tests if available
	var planPrompt string
	if hasTests {
		planPrompt = fmt.Sprintf(`Analyze this Go source file AND its test file together.
Return ONLY a JSON array of source filenames to create (not test files - those will be generated to match).

Example response: ["types.go", "helpers.go", "handlers.go"]

Rules:
- Use descriptive names based on content
- Keep related code together (types with their methods)
- Separate helpers from main logic
- Consider test coverage: functions tested together should stay together
- Each output file should have meaningful, testable units

SOURCE FILE (%s):
%s

TEST FILE (%s):
%s`, filepath.Base(filename), string(content), result.TestFile, string(testContent))
	} else {
		planPrompt = fmt.Sprintf(`Analyze this Go file and return ONLY a JSON array of filenames to create.
Example: ["helpers.go", "handlers.go", "types.go"]

Rules:
- Use descriptive names based on content
- Keep related code together
- Separate helpers from main logic

File content:
%s`, string(content))
	}

	planResult, err := client.Call(planPrompt, 500)
	if err != nil {
		ui.StopSpinnerMsg(false, "Planning failed")
		return fmt.Errorf("planning failed: %w", err)
	}

	filenames := parseFilenames(planResult)
	if len(filenames) == 0 {
		ui.StopSpinnerMsg(false, "Could not determine files to create")
		return fmt.Errorf("could not determine files to create")
	}

	ui.StopSpinnerMsg(true, fmt.Sprintf("Will create: %s", strings.Join(filenames, ", ")))

	if cfg.DryRun {
		for _, fname := range filenames {
			result.Files = append(result.Files, GeneratedFile{
				Name:   fname,
				Status: "skipped",
			})
			if !genCfg.SkipTests {
				testFname := strings.TrimSuffix(fname, ".go") + "_test.go"
				result.Files = append(result.Files, GeneratedFile{
					Name:   testFname,
					Status: "skipped",
				})
			}
		}

		if cfg.JSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		}

		ui.Info("Dry run - no files will be created")
		return nil
	}

	cmd.Println()

	// Generate each file pair (source + test) together
	for i, fname := range filenames {
		testFname := strings.TrimSuffix(fname, ".go") + "_test.go"

		// Generate source and test together in one prompt if tests exist
		if hasTests && !genCfg.SkipTests {
			ui.Step(i+1, len(filenames), fmt.Sprintf("Generating %s + %s", fname, testFname))

			genPrompt := fmt.Sprintf(`You are splitting a Go file and its tests. Generate BOTH files.

OUTPUT FORMAT - Return exactly this JSON structure:
{
  "source": "// source code here",
  "test": "// test code here"
}

SOURCE FILE to split - extract code for %s:
%s

TEST FILE to split - extract tests for %s:
%s

Rules:
- Include package declaration and imports in both files
- Move tests that test functions/types in the source file to the test file
- Maintain test coverage relationships
- Output valid Go code (no markdown)`, fname, string(content), testFname, string(testContent))

			response, err := client.Call(genPrompt, 6000)
			if err != nil {
				result.Files = append(result.Files, GeneratedFile{Name: fname, Status: "failed", Error: err.Error()})
				result.Files = append(result.Files, GeneratedFile{Name: testFname, Status: "failed", Error: err.Error()})
				cmd.Printf(" ✗ (%v)\n", err)
				continue
			}

			sourceCode, testCode := parseSourceAndTest(response)
			if sourceCode == "" {
				result.Files = append(result.Files, GeneratedFile{Name: fname, Status: "failed", Error: "could not parse source from response"})
				cmd.Println(" ✗ (parse error)")
				continue
			}

			// Write source file
			sourceCode = cleanCode(sourceCode)
			if err := os.WriteFile(filepath.Join(outDir, fname), []byte(sourceCode), 0644); err != nil {
				result.Files = append(result.Files, GeneratedFile{Name: fname, Status: "failed", Error: err.Error()})
				cmd.Println(" ✗ (write error)")
				continue
			}
			lines := analyzer.CountLines(sourceCode)
			result.Files = append(result.Files, GeneratedFile{Name: fname, Lines: lines, Status: "created"})

			// Write test file
			testCode = cleanCode(testCode)
			if testCode != "" {
				if err := os.WriteFile(filepath.Join(outDir, testFname), []byte(testCode), 0644); err != nil {
					result.Files = append(result.Files, GeneratedFile{Name: testFname, Status: "failed", Error: err.Error()})
				} else {
					testLines := analyzer.CountLines(testCode)
					testCount := countTestsInCode(testCode)
					result.Files = append(result.Files, GeneratedFile{Name: testFname, Lines: testLines, Status: "created", TestCount: testCount})
				}
			}
			cmd.Printf(" ✓ (%d lines + %d test lines)\n", lines, analyzer.CountLines(testCode))

		} else {
			// Source only (no existing tests or --skip-tests)
			ui.Step(i+1, len(filenames), fmt.Sprintf("Generating %s", fname))

			genPrompt := fmt.Sprintf(`You are splitting a Go file. Generate %s.

Source:
%s

Output ONLY valid Go code. Include package and imports. No markdown.`, fname, string(content))

			code, err := client.Call(genPrompt, 3000)
			if err != nil {
				result.Files = append(result.Files, GeneratedFile{Name: fname, Status: "failed", Error: err.Error()})
				cmd.Printf(" ✗ (%v)\n", err)
				continue
			}

			code = cleanCode(code)
			if err := os.WriteFile(filepath.Join(outDir, fname), []byte(code), 0644); err != nil {
				result.Files = append(result.Files, GeneratedFile{Name: fname, Status: "failed", Error: err.Error()})
				cmd.Println(" ✗ (write error)")
				continue
			}

			lines := analyzer.CountLines(code)
			result.Files = append(result.Files, GeneratedFile{Name: fname, Lines: lines, Status: "created"})
			cmd.Printf(" ✓ (%d lines)\n", lines)

			// Generate test stubs if no tests exist and not skipping
			if !hasTests && !genCfg.SkipTests {
				ui.Step(i+1, len(filenames), fmt.Sprintf("Generating %s (stubs)", testFname))

				stubPrompt := fmt.Sprintf(`Generate test stubs for this Go source file.
Each exported function should have a corresponding test stub with t.Skip("TODO: implement").

Source file %s:
%s

Output ONLY valid Go test code. Include package and imports. No markdown.`, fname, code)

				stubCode, err := client.Call(stubPrompt, 2000)
				if err != nil {
					result.Files = append(result.Files, GeneratedFile{Name: testFname, Status: "failed", Error: err.Error()})
					cmd.Printf(" ✗ (%v)\n", err)
					continue
				}

				stubCode = cleanCode(stubCode)
				if err := os.WriteFile(filepath.Join(outDir, testFname), []byte(stubCode), 0644); err != nil {
					result.Files = append(result.Files, GeneratedFile{Name: testFname, Status: "failed", Error: err.Error()})
					cmd.Println(" ✗ (write error)")
					continue
				}

				testLines := analyzer.CountLines(stubCode)
				result.Files = append(result.Files, GeneratedFile{Name: testFname, Lines: testLines, Status: "created"})
				cmd.Printf(" ✓ (%d lines, stubs)\n", testLines)
			}
		}
	}

	// Run validation unless skipped or dry-run
	if !cfg.DryRun && !genCfg.SkipValidation {
		cmd.Println()
		ui.StartSpinner("Validating split (go test)...")

		if err := runValidation(outDir); err != nil {
			ui.StopSpinnerMsg(false, "Validation failed")
			result.ValidationPassed = false
			result.ValidationError = err.Error()
			ui.Error(fmt.Sprintf("go test failed: %v", err))
		} else {
			ui.StopSpinnerMsg(true, "All tests pass")
			result.ValidationPassed = true
		}
	}

	if cfg.JSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	cmd.Println()
	if result.ValidationPassed || genCfg.SkipValidation || cfg.DryRun {
		ui.Success("Generation complete")
	} else {
		ui.Warning("Generation complete but validation failed")
	}
	return nil
}

func parseFilenames(response string) []string {
	var filenames []string

	start := strings.Index(response, "[")
	end := strings.LastIndex(response, "]")
	if start >= 0 && end > start {
		arr := response[start+1 : end]
		parts := strings.Split(arr, ",")
		for _, p := range parts {
			p = strings.Trim(p, " \"'\t\n")
			if strings.HasSuffix(p, ".go") {
				filenames = append(filenames, p)
			}
		}
	}

	if len(filenames) == 0 {
		words := strings.Fields(response)
		for _, w := range words {
			w = strings.Trim(w, `"',[]`)
			if strings.HasSuffix(w, ".go") {
				filenames = append(filenames, w)
			}
		}
	}

	return filenames
}

func cleanCode(code string) string {
	code = strings.TrimSpace(code)

	if strings.HasPrefix(code, "```") {
		lines := strings.Split(code, "\n")
		if len(lines) > 2 {
			endIdx := len(lines) - 1
			if strings.TrimSpace(lines[endIdx]) == "```" {
				lines = lines[1:endIdx]
			} else {
				lines = lines[1:]
			}
			code = strings.Join(lines, "\n")
		}
	}

	return code
}

// parseSourceAndTest extracts source and test code from a JSON response.
func parseSourceAndTest(response string) (source, test string) {
	// Try to parse as JSON first
	var result struct {
		Source string `json:"source"`
		Test   string `json:"test"`
	}
	if err := json.Unmarshal([]byte(response), &result); err == nil && result.Source != "" {
		return result.Source, result.Test
	}

	// Fallback: try to find JSON object with "source" key in the response
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")
	if start >= 0 && end > start {
		jsonStr := response[start : end+1]
		// Only try if it looks like our expected JSON structure
		if strings.Contains(jsonStr, `"source"`) {
			if err := json.Unmarshal([]byte(jsonStr), &result); err == nil && result.Source != "" {
				return result.Source, result.Test
			}
		}
	}

	// Last resort: return the whole response as source
	return response, ""
}

// countTestsInCode counts Test* functions in Go test code.
func countTestsInCode(code string) int {
	count := 0
	lines := strings.Split(code, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "func Test") {
			count++
		}
	}
	return count
}

// runValidation runs go test on the output directory.
func runValidation(dir string) error {
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		if len(output) > 0 {
			return fmt.Errorf("%s", strings.TrimSpace(string(output)))
		}
		return err
	}
	return nil
}
