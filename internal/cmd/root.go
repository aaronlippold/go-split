// Package cmd implements the CLI commands using cobra.
package cmd

import (
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/aaronlippold/go-split/internal/api"
)

// Version is set at build time.
var Version = "dev"

const (
	defaultEndpoint = "http://localhost:8000/v1/messages"
	defaultModel    = "claude-sonnet-4-5-20250929"
	defaultTimeout  = 120 * time.Second
)

// Config holds CLI configuration shared across commands.
type Config struct {
	Endpoint   string
	Model      string
	Timeout    time.Duration
	Verbose    bool
	DryRun     bool
	OutputDir  string
	CaptureDir string
	APIKey     string
	JSON       bool
	JSONL      bool
	NoColor    bool
	// Check flags
	SkipFmt    bool
	SkipVet    bool
	SkipLint   bool
	SkipSec    bool
	SkipBuild  bool
	SkipTests  bool
	SkipChecks bool
}

// Global config instance used by commands
var cfg = &Config{}

// NewRootCmd creates a new root command with all subcommands.
// This factory function allows creating fresh command trees for testing.
func NewRootCmd() *cobra.Command {
	// Reset config to defaults
	*cfg = Config{
		Endpoint: defaultEndpoint,
		Model:    defaultModel,
		Timeout:  defaultTimeout,
	}

	rootCmd := &cobra.Command{
		Use:   "go-split",
		Short: "Intelligently split large Go files into smaller modules",
		Long: `go-split uses AI to analyze Go files and recommend how to split them
into smaller, more focused modules. It can also generate the split files
and run quality checks.`,
		Version: Version,
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfg.Endpoint, "endpoint", getEnvOrDefault("GO_SPLIT_ENDPOINT", defaultEndpoint), "API endpoint URL")
	rootCmd.PersistentFlags().StringVar(&cfg.Model, "model", getEnvOrDefault("GO_SPLIT_MODEL", defaultModel), "Model to use")
	rootCmd.PersistentFlags().BoolVarP(&cfg.Verbose, "verbose", "V", false, "Verbose output")
	rootCmd.PersistentFlags().BoolVar(&cfg.DryRun, "dry-run", false, "Preview changes without writing files")
	rootCmd.PersistentFlags().StringVarP(&cfg.OutputDir, "output", "o", "", "Output directory (default: same as input)")
	rootCmd.PersistentFlags().StringVar(&cfg.CaptureDir, "capture", getEnvOrDefault("GO_SPLIT_CAPTURE", ""), "Capture API requests/responses to directory")
	rootCmd.PersistentFlags().StringVar(&cfg.APIKey, "api-key", "", "Anthropic API key (uses ANTHROPIC_API_KEY env if not set)")
	rootCmd.PersistentFlags().BoolVar(&cfg.JSON, "json", false, "Output in JSON format for scripting")
	rootCmd.PersistentFlags().BoolVar(&cfg.JSONL, "jsonl", false, "Output in JSONL format (newline-delimited JSON)")
	rootCmd.PersistentFlags().BoolVar(&cfg.NoColor, "no-color", false, "Disable colored output")

	// Create subcommands
	checkCmd := newCheckCmd()

	// Add subcommands
	rootCmd.AddCommand(newAnalyzeCmd())
	rootCmd.AddCommand(newGenerateCmd())
	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(newValidateCmd())

	return rootCmd
}

// Execute runs the CLI.
func Execute() error {
	return NewRootCmd().Execute()
}

// ExecuteWithArgs runs the CLI with custom args and writers (for testing).
func ExecuteWithArgs(args []string, stdout, stderr io.Writer) error {
	cmd := NewRootCmd()
	cmd.SetArgs(args)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	return cmd.Execute()
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// newAPIClient creates an API client with configured options.
func newAPIClient() *api.Client {
	client := api.NewClient(cfg.Endpoint, cfg.Model, cfg.Timeout)

	if cfg.APIKey != "" || os.Getenv("ANTHROPIC_API_KEY") != "" {
		client = client.WithAPIKey(cfg.APIKey)
	}

	if cfg.CaptureDir != "" {
		client = client.WithCapture(cfg.CaptureDir)
	}

	return client
}
