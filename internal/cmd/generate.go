package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aaronlippold/go-split/internal/analyzer"
)

// GenerateResult holds generation results for JSON output.
type GenerateResult struct {
	SourceFile string          `json:"source_file"`
	OutputDir  string          `json:"output_dir"`
	DryRun     bool            `json:"dry_run"`
	TestFile   string          `json:"test_file,omitempty"`
	Files      []GeneratedFile `json:"files"`
}

// GeneratedFile describes a generated file.
type GeneratedFile struct {
	Name   string `json:"name"`
	Lines  int    `json:"lines"`
	Status string `json:"status"` // "created", "failed", "skipped"
	Error  string `json:"error,omitempty"`
}

// newGenerateCmd creates the generate command.
func newGenerateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "generate <file>",
		Short: "Generate split files from a Go file",
		Long: `Generate split files based on AI analysis. The AI will determine
how to best split the file and generate the new files.`,
		Args: cobra.ExactArgs(1),
		RunE: runGenerate,
	}
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
	if testFilePath != "" {
		result.TestFile = filepath.Base(testFilePath)
		var err error
		testContent, err = os.ReadFile(testFilePath)
		if err != nil {
			testContent = nil // Ignore read errors
		}
	}

	ui.Header(fmt.Sprintf("📄 Splitting %s (%d lines)", filepath.Base(filename), info.Lines))

	if testFilePath != "" {
		testInfo, _ := analyzer.ParseGoFile(testFilePath)
		if testInfo != nil {
			testCount := countTestFunctions(testInfo)
			ui.Info(fmt.Sprintf("Found test file: %s (%d lines, %d tests) - will split alongside source", result.TestFile, testInfo.Lines, testCount))
		}
	}

	ui.StartSpinner("Planning split...")

	client := newAPIClient()
	planPrompt := fmt.Sprintf(`Analyze this Go file and return ONLY a JSON array of filenames to create.
Example: ["helpers.go", "handlers.go", "types.go"]

Rules:
- Use descriptive names based on content
- Keep related code together
- Separate helpers from main logic

File content:
%s`, string(content))

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

	// Generate each file (and its test file if applicable)
	for i, fname := range filenames {
		ui.Step(i+1, len(filenames), fmt.Sprintf("Generating %s", fname))

		genPrompt := fmt.Sprintf(`You are splitting a Go file. Generate %s.

Source:
%s

Output ONLY valid Go code. Include package and imports. No markdown.`, fname, string(content))

		code, err := client.Call(genPrompt, 3000)
		if err != nil {
			result.Files = append(result.Files, GeneratedFile{
				Name:   fname,
				Status: "failed",
				Error:  err.Error(),
			})
			cmd.Printf(" ✗ (%v)\n", err)
			continue
		}

		code = cleanCode(code)
		outPath := filepath.Join(outDir, fname)

		if err := os.WriteFile(outPath, []byte(code), 0644); err != nil {
			result.Files = append(result.Files, GeneratedFile{
				Name:   fname,
				Status: "failed",
				Error:  err.Error(),
			})
			cmd.Println(" ✗ (write error)")
			continue
		}

		lines := analyzer.CountLines(code)
		result.Files = append(result.Files, GeneratedFile{
			Name:   fname,
			Lines:  lines,
			Status: "created",
		})
		cmd.Printf(" ✓ (%d lines)\n", lines)

		// Generate corresponding test file if source has tests
		if len(testContent) > 0 && !strings.HasSuffix(fname, "_test.go") {
			testFname := strings.TrimSuffix(fname, ".go") + "_test.go"
			ui.Step(i+1, len(filenames), fmt.Sprintf("Generating %s", testFname))

			testPrompt := fmt.Sprintf(`You are splitting a Go test file to match a source file split.

The source file %s was created with this content:
%s

The original test file contained:
%s

Generate the test file %s that contains ONLY the tests relevant to %s.
Move tests that test functions/types now in %s to this new test file.
If no tests are relevant, output an empty test file with just the package declaration.

Output ONLY valid Go test code. Include package and imports. No markdown.`, fname, code, string(testContent), testFname, fname, fname)

			testCode, err := client.Call(testPrompt, 3000)
			if err != nil {
				result.Files = append(result.Files, GeneratedFile{
					Name:   testFname,
					Status: "failed",
					Error:  err.Error(),
				})
				cmd.Printf(" ✗ (%v)\n", err)
				continue
			}

			testCode = cleanCode(testCode)
			testOutPath := filepath.Join(outDir, testFname)

			if err := os.WriteFile(testOutPath, []byte(testCode), 0644); err != nil {
				result.Files = append(result.Files, GeneratedFile{
					Name:   testFname,
					Status: "failed",
					Error:  err.Error(),
				})
				cmd.Println(" ✗ (write error)")
				continue
			}

			testLines := analyzer.CountLines(testCode)
			result.Files = append(result.Files, GeneratedFile{
				Name:   testFname,
				Lines:  testLines,
				Status: "created",
			})
			cmd.Printf(" ✓ (%d lines)\n", testLines)
		}
	}

	if cfg.JSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	cmd.Println()
	ui.Success("Generation complete")
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
