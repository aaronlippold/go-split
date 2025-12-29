package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// CheckResult holds quality check results for JSON output.
type CheckResult struct {
	Target string        `json:"target"`
	Passed bool          `json:"passed"`
	Checks []CheckStatus `json:"checks"`
}

// CheckStatus describes a single check result.
type CheckStatus struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Skipped bool   `json:"skipped"`
	Error   string `json:"error,omitempty"`
}

// newCheckCmd creates the check command with its flags.
func newCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check <path>",
		Short: "Run quality checks on Go files",
		Long: `Run quality checks including gofmt, go vet, golangci-lint,
gosec, go build, and go test on the specified file or directory.`,
		Args: cobra.ExactArgs(1),
		RunE: runCheck,
	}

	// Check command flags
	cmd.Flags().BoolVar(&cfg.SkipChecks, "skip-checks", false, "Skip all quality checks")
	cmd.Flags().BoolVar(&cfg.SkipFmt, "skip-fmt", false, "Skip gofmt check")
	cmd.Flags().BoolVar(&cfg.SkipVet, "skip-vet", false, "Skip go vet check")
	cmd.Flags().BoolVar(&cfg.SkipLint, "skip-lint", false, "Skip golangci-lint check")
	cmd.Flags().BoolVar(&cfg.SkipSec, "skip-sec", false, "Skip gosec security check")
	cmd.Flags().BoolVar(&cfg.SkipBuild, "skip-build", false, "Skip go build check")
	cmd.Flags().BoolVar(&cfg.SkipTests, "skip-tests", false, "Skip go test check")

	return cmd
}

func runCheck(cmd *cobra.Command, args []string) error {
	ui := NewUI(cmd.OutOrStdout(), cfg.JSON || cfg.JSONL)

	target := args[0]
	info, err := os.Stat(target)
	if os.IsNotExist(err) {
		return fmt.Errorf("not found: %s", target)
	}

	dir := target
	if !info.IsDir() {
		dir = filepath.Dir(target)
	}

	result := CheckResult{
		Target: dir,
		Passed: true,
		Checks: []CheckStatus{},
	}

	ui.Header(fmt.Sprintf("🔍 Running quality checks on %s", dir))

	if cfg.SkipChecks {
		if cfg.JSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		}
		if cfg.JSONL {
			return nil
		}
		ui.Info("All checks skipped (--skip-checks)")
		return nil
	}

	if !cfg.JSON && !cfg.JSONL {
		cmd.Println()
	}

	jsonlEnc := json.NewEncoder(cmd.OutOrStdout())

	checks := []struct {
		name string
		skip bool
		tool string
		args []string
	}{
		{"gofmt", cfg.SkipFmt, "gofmt", []string{"-l", "-d", "."}},
		{"go vet", cfg.SkipVet, "go", []string{"vet", "./..."}},
		{"golangci-lint", cfg.SkipLint, "golangci-lint", []string{"run", "--timeout", "2m"}},
		{"gosec", cfg.SkipSec, "gosec", []string{"-quiet", "./..."}},
		{"go build", cfg.SkipBuild, "go", []string{"build", "./..."}},
		{"go test", cfg.SkipTests, "go", []string{"test", "-short", "./..."}},
	}

	for i, check := range checks {
		status := CheckStatus{Name: check.name}

		if check.skip {
			status.Skipped = true
			status.Passed = true
			result.Checks = append(result.Checks, status)
			if cfg.JSONL {
				_ = jsonlEnc.Encode(status)
				continue
			}
			if !cfg.JSON {
				cmd.Printf("   [%d/%d] %s - skipped\n", i+1, len(checks), check.name)
			}
			continue
		}

		// Check if optional tool is available
		if check.tool == "golangci-lint" || check.tool == "gosec" {
			if _, err := exec.LookPath(check.tool); err != nil {
				status.Skipped = true
				status.Passed = true
				status.Error = "not installed"
				result.Checks = append(result.Checks, status)
				if cfg.JSONL {
					_ = jsonlEnc.Encode(status)
					continue
				}
				if !cfg.JSON && cfg.Verbose {
					cmd.Printf("   [%d/%d] %s - not installed\n", i+1, len(checks), check.name)
				}
				continue
			}
		}

		if !cfg.JSON && !cfg.JSONL {
			cmd.Printf("   [%d/%d] %s...", i+1, len(checks), check.name)
		}

		if err := runTool(dir, check.tool, check.args...); err != nil {
			status.Passed = false
			status.Error = err.Error()
			result.Passed = false
		} else {
			status.Passed = true
		}
		result.Checks = append(result.Checks, status)

		if cfg.JSONL {
			_ = jsonlEnc.Encode(status)
			continue
		}

		if cfg.JSON {
			continue
		}

		// Text mode output
		if !status.Passed {
			cmd.Printf(" ✗\n        %v\n", status.Error)
		} else {
			cmd.Println(" ✓")
		}
	}

	if cfg.JSONL {
		if !result.Passed {
			return fmt.Errorf("some checks failed")
		}
		return nil
	}

	if cfg.JSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	cmd.Println()
	if result.Passed {
		ui.Success("All quality checks passed")
		return nil
	}
	ui.Error("Some checks failed")
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
