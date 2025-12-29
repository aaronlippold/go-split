package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/aaronlippold/go-split/internal/analyzer"
)

// ValidateResult holds validation results for JSON output.
type ValidateResult struct {
	Target    string          `json:"target"`
	FileCount int             `json:"file_count"`
	Valid     bool            `json:"valid"`
	Files     []ValidatedFile `json:"files,omitempty"`
}

// ValidatedFile describes a validated file.
type ValidatedFile struct {
	Name  string `json:"name"`
	Valid bool   `json:"valid"`
	Error string `json:"error,omitempty"`
}

var validateCmd = &cobra.Command{
	Use:   "validate <path>",
	Short: "Validate Go syntax of files",
	Long:  `Validate that all Go files in the specified path have valid syntax.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runValidate,
}

func runValidate(cmd *cobra.Command, args []string) error {
	ui := NewUI(cmd.OutOrStdout(), cfg.JSON)

	target := args[0]
	info, err := os.Stat(target)
	if os.IsNotExist(err) {
		return fmt.Errorf("not found: %s", target)
	}

	dir := target
	if !info.IsDir() {
		dir = filepath.Dir(target)
	}

	result := ValidateResult{
		Target: dir,
		Valid:  true,
		Files:  []ValidatedFile{},
	}

	ui.Header(fmt.Sprintf("🔍 Validating Go files in %s", dir))

	matches, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return fmt.Errorf("finding files: %w", err)
	}

	result.FileCount = len(matches)

	if len(matches) == 0 {
		if cfg.JSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		}
		ui.Info("No Go files found")
		return nil
	}

	cmd.Println()

	for i, f := range matches {
		vf := ValidatedFile{Name: filepath.Base(f), Valid: true}
		_, err := analyzer.ParseGoFile(f)
		if err != nil {
			vf.Valid = false
			vf.Error = err.Error()
			result.Valid = false
			if !cfg.JSON {
				cmd.Printf("   [%d/%d] %s ✗\n        %v\n", i+1, len(matches), filepath.Base(f), err)
			}
		} else if !cfg.JSON {
			if cfg.Verbose {
				cmd.Printf("   [%d/%d] %s ✓\n", i+1, len(matches), filepath.Base(f))
			}
		}
		result.Files = append(result.Files, vf)
	}

	if cfg.JSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	if !result.Valid {
		cmd.Println()
		ui.Error("Validation failed")
		return fmt.Errorf("validation failed")
	}

	cmd.Println()
	ui.Success(fmt.Sprintf("All %d files are valid Go syntax", len(matches)))
	return nil
}
