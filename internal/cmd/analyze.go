package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aaronlippold/go-split/internal/analyzer"
)

// AnalyzeResult holds analysis results for JSON output.
type AnalyzeResult struct {
	File            string `json:"file"`
	Package         string `json:"package"`
	Lines           int    `json:"lines"`
	Functions       int    `json:"functions"`
	Types           int    `json:"types"`
	Variables       int    `json:"variables"`
	TestFile        string `json:"test_file,omitempty"`
	TestLines       int    `json:"test_lines,omitempty"`
	TestFunctions   int    `json:"test_functions,omitempty"`
	Recommendations string `json:"recommendations,omitempty"`
}

// newAnalyzeCmd creates the analyze command.
func newAnalyzeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "analyze <file>",
		Short: "Analyze a Go file and show recommended splits",
		Long: `Analyze a Go file to understand its structure and get AI-powered
recommendations for how to split it into smaller, focused modules.`,
		Args: cobra.ExactArgs(1),
		RunE: runAnalyze,
	}
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	ui := NewUI(cmd.OutOrStdout(), IsStructuredOutput())

	filename := args[0]
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", filename)
	}

	info, err := analyzer.ParseGoFile(filename)
	if err != nil {
		return fmt.Errorf("parsing file: %w", err)
	}

	result := AnalyzeResult{
		File:      filepath.Base(filename),
		Package:   info.Package,
		Lines:     info.Lines,
		Functions: len(info.Functions),
		Types:     len(info.Types),
		Variables: len(info.Vars),
	}

	// Check for associated test file
	testFile := findTestFile(filename)
	if testFile != "" {
		testInfo, err := analyzer.ParseGoFile(testFile)
		if err == nil {
			result.TestFile = filepath.Base(testFile)
			result.TestLines = testInfo.Lines
			result.TestFunctions = countTestFunctions(testInfo)
		}
	}

	if !IsStructuredOutput() {
		ui.Header(fmt.Sprintf("📄 Analyzing %s (%d lines)", result.File, result.Lines))
		cmd.Printf("   Package:   %s\n", result.Package)
		cmd.Printf("   Functions: %d\n", result.Functions)
		cmd.Printf("   Types:     %d\n", result.Types)
		cmd.Printf("   Variables: %d\n", result.Variables)

		if result.TestFile != "" {
			cmd.Printf("\n🧪 Test file: %s (%d lines, %d tests)\n", result.TestFile, result.TestLines, result.TestFunctions)
		} else {
			ui.Warning("No associated test file found")
		}

		if cfg.Verbose {
			cmd.Println("\n   Functions:")
			for _, fn := range info.Functions {
				if fn.Receiver != "" {
					cmd.Printf("     • (%s) %s (lines %d-%d)\n", fn.Receiver, fn.Name, fn.Line, fn.EndLine)
				} else {
					cmd.Printf("     • %s (lines %d-%d)\n", fn.Name, fn.Line, fn.EndLine)
				}
			}
		}
	}

	// Call API for recommendations
	content, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	ui.StartSpinner("Getting AI recommendations...")

	client := newAPIClient()
	prompt := fmt.Sprintf(`Analyze this Go file and propose how to split it into smaller, focused files.

Return a brief summary with:
1. Recommended file names
2. What each file should contain
3. Why this split makes sense

Be concise. File content:
%s`, string(content))

	response, err := client.Call(prompt, 1500)
	if err != nil {
		ui.StopSpinnerMsg(false, "API call failed")
		return fmt.Errorf("API call failed: %w", err)
	}

	ui.StopSpinnerMsg(true, "Got recommendations")
	result.Recommendations = response

	if IsStructuredOutput() {
		return PrintOutput(cmd.OutOrStdout(), result)
	}

	ui.Header("📋 Recommendations")
	cmd.Println(response)

	return nil
}

func findTestFile(filename string) string {
	base := strings.TrimSuffix(filename, ".go")
	testFile := base + "_test.go"
	if _, err := os.Stat(testFile); err == nil {
		return testFile
	}
	return ""
}

func countTestFunctions(info *analyzer.FileInfo) int {
	count := 0
	for _, fn := range info.Functions {
		if strings.HasPrefix(fn.Name, "Test") {
			count++
		}
	}
	return count
}
