// Package cli implements the command-line interface for go-split.
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/aaronlippold/go-split/internal/analyzer"
	"github.com/aaronlippold/go-split/internal/api"
)

// Version is set at build time.
var Version = "dev"

const (
	defaultEndpoint = "http://localhost:8000/v1/messages"
	defaultModel    = "claude-sonnet-4-5-20250929"
	defaultTimeout  = 120 * time.Second
)

// Config holds CLI configuration.
type Config struct {
	Endpoint   string
	Model      string
	Timeout    time.Duration
	Verbose    bool
	DryRun     bool
	OutputDir  string
	CaptureDir string // Directory to capture API requests/responses
	APIKey     string // Direct Anthropic API key (bypasses wrapper)
	// Check flags
	SkipFmt    bool
	SkipVet    bool
	SkipLint   bool
	SkipSec    bool
	SkipBuild  bool
	SkipTests  bool
	SkipChecks bool // Skip all checks
	// Output flags
	JSON   bool   // Output in JSON format for scripting
	Format string // Output format: json, jsonl, toon (default: human)
}

// Run executes the CLI with the given arguments.
func Run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printUsage(stderr)
		return fmt.Errorf("no command specified")
	}

	// Check for help/version flags or commands first
	first := args[0]
	if first == "-h" || first == "--help" || first == "help" {
		printUsage(stdout)
		return nil
	}
	if first == "-v" || first == "--version" || first == "version" {
		fmt.Fprintf(stdout, "go-split %s\n", Version)
		return nil
	}

	// Parse global flags
	cfg := &Config{
		Endpoint: getEnvOrDefault("GO_SPLIT_ENDPOINT", defaultEndpoint),
		Model:    getEnvOrDefault("GO_SPLIT_MODEL", defaultModel),
		Timeout:  defaultTimeout,
	}

	fs := flag.NewFlagSet("go-split", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&cfg.Endpoint, "endpoint", cfg.Endpoint, "API endpoint URL")
	fs.StringVar(&cfg.Model, "model", cfg.Model, "Model to use")
	fs.BoolVar(&cfg.Verbose, "verbose", false, "Verbose output")
	fs.BoolVar(&cfg.DryRun, "dry-run", false, "Preview changes without writing files")
	fs.StringVar(&cfg.OutputDir, "output", "", "Output directory (default: same as input)")
	fs.StringVar(&cfg.CaptureDir, "capture", getEnvOrDefault("GO_SPLIT_CAPTURE", ""), "Capture API requests/responses to directory")
	fs.StringVar(&cfg.APIKey, "api-key", "", "Anthropic API key (uses ANTHROPIC_API_KEY if not set)")
	// Check control flags
	fs.BoolVar(&cfg.SkipChecks, "skip-checks", false, "Skip all quality checks")
	fs.BoolVar(&cfg.SkipFmt, "skip-fmt", false, "Skip gofmt check")
	fs.BoolVar(&cfg.SkipVet, "skip-vet", false, "Skip go vet check")
	fs.BoolVar(&cfg.SkipLint, "skip-lint", false, "Skip golangci-lint check")
	fs.BoolVar(&cfg.SkipSec, "skip-sec", false, "Skip gosec security check")
	fs.BoolVar(&cfg.SkipBuild, "skip-build", false, "Skip go build check")
	fs.BoolVar(&cfg.SkipTests, "skip-tests", false, "Skip go test check")
	// Output flags
	fs.BoolVar(&cfg.JSON, "json", false, "Output in JSON format for scripting")

	// Find command position (first non-flag arg)
	cmdIdx := 0
	for i, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			cmdIdx = i
			break
		}
	}

	// Parse flags before command
	if cmdIdx > 0 {
		if err := fs.Parse(args[:cmdIdx]); err != nil {
			return err
		}
	}

	// Get command and remaining args
	remaining := args[cmdIdx:]
	if len(remaining) == 0 {
		printUsage(stderr)
		return fmt.Errorf("no command specified")
	}

	command := remaining[0]
	cmdArgs := remaining[1:]

	// Dispatch command
	switch command {
	case "analyze":
		return runAnalyze(cfg, cmdArgs, stdout, stderr)
	case "generate":
		return runGenerate(cfg, cmdArgs, stdout, stderr)
	case "validate":
		return runValidate(cfg, cmdArgs, stdout, stderr)
	case "check":
		return runCheck(cfg, cmdArgs, stdout, stderr)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

// AnalyzeResult holds analysis results for JSON output.
type AnalyzeResult struct {
	File           string   `json:"file"`
	Package        string   `json:"package"`
	Lines          int      `json:"lines"`
	Functions      int      `json:"functions"`
	Types          int      `json:"types"`
	Variables      int      `json:"variables"`
	TestFile       string   `json:"test_file,omitempty"`
	TestLines      int      `json:"test_lines,omitempty"`
	TestFunctions  int      `json:"test_functions,omitempty"`
	Recommendations string  `json:"recommendations,omitempty"`
}

func runAnalyze(cfg *Config, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("analyze requires a file argument")
	}

	filename := args[0]
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", filename)
	}

	// Parse the file to get info
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

	if !cfg.JSON {
		fmt.Fprintf(stdout, "📄 Analyzing %s (%d lines)...\n", result.File, result.Lines)
		fmt.Fprintf(stdout, "   Package: %s\n", result.Package)
		fmt.Fprintf(stdout, "   Functions: %d\n", result.Functions)
		fmt.Fprintf(stdout, "   Types: %d\n", result.Types)
		fmt.Fprintf(stdout, "   Variables: %d\n", result.Variables)

		if result.TestFile != "" {
			fmt.Fprintf(stdout, "\n🧪 Associated test file: %s (%d lines)\n", result.TestFile, result.TestLines)
			fmt.Fprintf(stdout, "   Test functions: %d\n", result.TestFunctions)
		} else {
			fmt.Fprintf(stdout, "\n⚠️  No associated test file found\n")
		}
	}

	if cfg.Verbose {
		fmt.Fprintf(stdout, "\n   Functions:\n")
		for _, fn := range info.Functions {
			if fn.Receiver != "" {
				fmt.Fprintf(stdout, "     - (%s) %s (lines %d-%d)\n", fn.Receiver, fn.Name, fn.Line, fn.EndLine)
			} else {
				fmt.Fprintf(stdout, "     - %s (lines %d-%d)\n", fn.Name, fn.Line, fn.EndLine)
			}
		}
	}

	// Call API for split recommendations
	content, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	fmt.Fprintf(stdout, "\n🤖 Getting AI recommendations")

	client := newAPIClient(cfg)
	prompt := fmt.Sprintf(`Analyze this Go file and propose how to split it into smaller, focused files.

Return a brief summary with:
1. Recommended file names
2. What each file should contain
3. Why this split makes sense

Be concise. File content:
%s`, string(content))

	response, err := client.Call(prompt, 1500)
	if err != nil {
		fmt.Fprintf(stdout, " ✗\n")
		return fmt.Errorf("API call failed: %w", err)
	}
	fmt.Fprintf(stdout, " ✓\n\n")

	fmt.Fprintf(stdout, "📋 Recommendations:\n%s\n", response)

	return nil
}

func runGenerate(cfg *Config, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("generate requires a file argument")
	}

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

	fmt.Fprintf(stdout, "📄 Splitting %s (%d lines)...\n", filepath.Base(filename), info.Lines)

	// First get the plan
	fmt.Fprintf(stdout, "🤖 Planning split")

	client := newAPIClient(cfg)
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
		fmt.Fprintf(stdout, " ✗\n")
		return fmt.Errorf("planning failed: %w", err)
	}
	fmt.Fprintf(stdout, " ✓\n")

	filenames := parseFilenames(planResult)
	if len(filenames) == 0 {
		return fmt.Errorf("could not determine files to create")
	}

	fmt.Fprintf(stdout, "📋 Will create: %s\n\n", strings.Join(filenames, ", "))

	if cfg.DryRun {
		fmt.Fprintf(stdout, "🔍 Dry run - no files will be created\n")
		return nil
	}

	outDir := cfg.OutputDir
	if outDir == "" {
		outDir = filepath.Dir(filename)
	}

	// Generate each file
	for i, fname := range filenames {
		fmt.Fprintf(stdout, "[%d/%d] Generating %s", i+1, len(filenames), fname)

		genPrompt := fmt.Sprintf(`You are splitting a Go file. Generate %s.

Source:
%s

Output ONLY valid Go code. Include package and imports. No markdown.`, fname, string(content))

		code, err := client.Call(genPrompt, 3000)
		if err != nil {
			fmt.Fprintf(stdout, " ✗ (%v)\n", err)
			continue
		}

		code = cleanCode(code)
		outPath := filepath.Join(outDir, fname)

		if err := os.WriteFile(outPath, []byte(code), 0644); err != nil {
			fmt.Fprintf(stdout, " ✗ (write error)\n")
			continue
		}

		lines := analyzer.CountLines(code)
		fmt.Fprintf(stdout, " ✓ (%d lines)\n", lines)
	}

	fmt.Fprintf(stdout, "\n✅ Generation complete\n")
	return nil
}

func runCheck(cfg *Config, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("check requires a file or directory argument")
	}

	target := args[0]
	info, err := os.Stat(target)
	if os.IsNotExist(err) {
		return fmt.Errorf("not found: %s", target)
	}

	dir := target
	if !info.IsDir() {
		dir = filepath.Dir(target)
	}

	fmt.Fprintf(stdout, "🔍 Running quality checks on %s...\n\n", dir)

	if cfg.SkipChecks {
		fmt.Fprintf(stdout, "⏭️  All checks skipped (--skip-checks)\n")
		return nil
	}

	allPassed := true

	// 1. gofmt check
	if cfg.SkipFmt {
		fmt.Fprintf(stdout, "⏭️  Formatting (gofmt) - skipped\n")
	} else {
		fmt.Fprintf(stdout, "📝 Formatting (gofmt)...")
		if err := runTool(dir, "gofmt", "-l", "-d", "."); err != nil {
			fmt.Fprintf(stdout, " ✗\n   %v\n", err)
			allPassed = false
		} else {
			fmt.Fprintf(stdout, " ✓\n")
		}
	}

	// 2. go vet
	if cfg.SkipVet {
		fmt.Fprintf(stdout, "⏭️  Static analysis (go vet) - skipped\n")
	} else {
		fmt.Fprintf(stdout, "🔬 Static analysis (go vet)...")
		if err := runTool(dir, "go", "vet", "./..."); err != nil {
			fmt.Fprintf(stdout, " ✗\n   %v\n", err)
			allPassed = false
		} else {
			fmt.Fprintf(stdout, " ✓\n")
		}
	}

	// 3. golangci-lint (if available)
	if cfg.SkipLint {
		fmt.Fprintf(stdout, "⏭️  Linting (golangci-lint) - skipped\n")
	} else if _, err := exec.LookPath("golangci-lint"); err == nil {
		fmt.Fprintf(stdout, "🧹 Linting (golangci-lint)...")
		if err := runTool(dir, "golangci-lint", "run", "--timeout", "2m"); err != nil {
			fmt.Fprintf(stdout, " ✗\n   %v\n", err)
			allPassed = false
		} else {
			fmt.Fprintf(stdout, " ✓\n")
		}
	} else if cfg.Verbose {
		fmt.Fprintf(stdout, "⏭️  Skipping golangci-lint (not installed)\n")
	}

	// 4. gosec (if available)
	if cfg.SkipSec {
		fmt.Fprintf(stdout, "⏭️  Security (gosec) - skipped\n")
	} else if _, err := exec.LookPath("gosec"); err == nil {
		fmt.Fprintf(stdout, "🔒 Security (gosec)...")
		if err := runTool(dir, "gosec", "-quiet", "./..."); err != nil {
			fmt.Fprintf(stdout, " ✗\n   %v\n", err)
			allPassed = false
		} else {
			fmt.Fprintf(stdout, " ✓\n")
		}
	} else if cfg.Verbose {
		fmt.Fprintf(stdout, "⏭️  Skipping gosec (not installed)\n")
	}

	// 5. go build
	if cfg.SkipBuild {
		fmt.Fprintf(stdout, "⏭️  Build check (go build) - skipped\n")
	} else {
		fmt.Fprintf(stdout, "🔨 Build check (go build)...")
		if err := runTool(dir, "go", "build", "./..."); err != nil {
			fmt.Fprintf(stdout, " ✗\n   %v\n", err)
			allPassed = false
		} else {
			fmt.Fprintf(stdout, " ✓\n")
		}
	}

	// 6. go test
	if cfg.SkipTests {
		fmt.Fprintf(stdout, "⏭️  Tests (go test) - skipped\n")
	} else {
		fmt.Fprintf(stdout, "🧪 Tests (go test)...")
		if err := runTool(dir, "go", "test", "-short", "./..."); err != nil {
			fmt.Fprintf(stdout, " ✗\n   %v\n", err)
			allPassed = false
		} else {
			fmt.Fprintf(stdout, " ✓\n")
		}
	}

	fmt.Fprintln(stdout)
	if allPassed {
		fmt.Fprintf(stdout, "✅ All quality checks passed\n")
		return nil
	}
	return fmt.Errorf("some checks failed")
}

func runTool(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
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

func runValidate(cfg *Config, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("validate requires a file or directory argument")
	}

	target := args[0]
	info, err := os.Stat(target)
	if os.IsNotExist(err) {
		return fmt.Errorf("not found: %s", target)
	}

	dir := target
	if !info.IsDir() {
		dir = filepath.Dir(target)
	}

	fmt.Fprintf(stdout, "🔍 Validating Go files in %s...\n", dir)

	// Find Go files
	matches, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return fmt.Errorf("finding files: %w", err)
	}

	if len(matches) == 0 {
		fmt.Fprintf(stdout, "   No Go files found\n")
		return nil
	}

	// Check each file parses
	allValid := true
	for _, f := range matches {
		_, err := analyzer.ParseGoFile(f)
		if err != nil {
			fmt.Fprintf(stdout, "   ✗ %s: %v\n", filepath.Base(f), err)
			allValid = false
		} else if cfg.Verbose {
			fmt.Fprintf(stdout, "   ✓ %s\n", filepath.Base(f))
		}
	}

	if !allValid {
		return fmt.Errorf("validation failed")
	}

	fmt.Fprintf(stdout, "✅ All %d files are valid Go syntax\n", len(matches))
	return nil
}

func printUsage(w io.Writer) {
	fmt.Fprintf(w, `go-split - Intelligently split large Go files into smaller modules

Usage:
  go-split [flags] <command> <file>

Commands:
  analyze   Analyze a file and show recommended split
  generate  Generate split files
  check     Run quality checks (fmt, vet, lint, security, test)
  validate  Validate Go syntax of files

Flags:
  -h, --help       Show this help
  -v, --version    Show version
  --json           Output in JSON format (for scripting)
  --endpoint URL   API endpoint (default: %s)
  --model NAME     Model to use (default: %s)
  --verbose        Verbose output
  --dry-run        Preview without writing files
  --output DIR     Output directory
  --capture DIR    Capture API requests/responses to directory
  --api-key KEY    Direct Anthropic API key (bypasses wrapper)

Check Flags (for 'check' command):
  --skip-checks    Skip all quality checks
  --skip-fmt       Skip gofmt
  --skip-vet       Skip go vet
  --skip-lint      Skip golangci-lint
  --skip-sec       Skip gosec
  --skip-build     Skip go build
  --skip-tests     Skip go test

Environment:
  GO_SPLIT_ENDPOINT   API endpoint override
  GO_SPLIT_MODEL      Model override
  GO_SPLIT_CAPTURE    Capture directory override
  ANTHROPIC_API_KEY   Direct Anthropic API key (bypasses wrapper)

Examples:
  go-split analyze server.go
  go-split --dry-run generate server.go
  go-split validate ./cmd/

`, defaultEndpoint, defaultModel)
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// newAPIClient creates an API client with all configured options.
func newAPIClient(cfg *Config) *api.Client {
	client := api.NewClient(cfg.Endpoint, cfg.Model, cfg.Timeout)

	// Enable direct Anthropic API if key provided or in env
	if cfg.APIKey != "" || os.Getenv("ANTHROPIC_API_KEY") != "" {
		client = client.WithAPIKey(cfg.APIKey)
	}

	// Enable capture mode if directory specified
	if cfg.CaptureDir != "" {
		client = client.WithCapture(cfg.CaptureDir)
	}

	return client
}

func parseFilenames(response string) []string {
	var filenames []string

	// Find JSON array
	start := strings.Index(response, "[")
	end := strings.LastIndex(response, "]")
	if start >= 0 && end > start {
		// Simple JSON array parsing
		arr := response[start+1 : end]
		parts := strings.Split(arr, ",")
		for _, p := range parts {
			p = strings.Trim(p, ` "'\t\n`)
			if strings.HasSuffix(p, ".go") {
				filenames = append(filenames, p)
			}
		}
	}

	// Fallback: look for .go files
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

func cleanCode(code string) string {
	code = strings.TrimSpace(code)

	// Remove markdown code blocks
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
