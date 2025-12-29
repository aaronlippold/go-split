package cmd_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aaronlippold/go-split/internal/cmd"
)

func TestHelp(t *testing.T) {
	tests := []struct {
		args []string
	}{
		{[]string{"--help"}},
		{[]string{"-h"}},
		{[]string{"help"}},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := cmd.ExecuteWithArgs(tt.args, &stdout, &stderr)
			if err != nil {
				t.Errorf("ExecuteWithArgs(%v) error = %v", tt.args, err)
			}

			output := stdout.String()
			if !strings.Contains(output, "go-split") {
				t.Errorf("Expected 'go-split' in output")
			}
			if !strings.Contains(output, "analyze") {
				t.Errorf("Expected 'analyze' command in output")
			}
		})
	}
}

func TestVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := cmd.ExecuteWithArgs([]string{"--version"}, &stdout, &stderr)
	if err != nil {
		t.Errorf("ExecuteWithArgs() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "go-split") {
		t.Errorf("Expected version info, got: %s", output)
	}
}

func TestUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := cmd.ExecuteWithArgs([]string{"unknown-command"}, &stdout, &stderr)
	if err == nil {
		t.Error("Expected error for unknown command")
	}
}

func TestAnalyzeMissingFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := cmd.ExecuteWithArgs([]string{"analyze"}, &stdout, &stderr)
	if err == nil {
		t.Error("Expected error when file not specified")
	}
}

func TestAnalyzeNonexistentFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := cmd.ExecuteWithArgs([]string{"analyze", "/nonexistent/file.go"}, &stdout, &stderr)
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestValidateJSON(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.go")
	if err := os.WriteFile(testFile, []byte("package test\n\nfunc Hello() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	err := cmd.ExecuteWithArgs([]string{"--format=json", "validate", dir}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("ExecuteWithArgs() error = %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v\nOutput: %s", err, stdout.String())
	}

	if result["valid"] != true {
		t.Errorf("Expected valid=true, got %v", result["valid"])
	}
	if result["file_count"].(float64) != 1 {
		t.Errorf("Expected file_count=1, got %v", result["file_count"])
	}
}

func TestValidateJSON_Invalid(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "bad.go")
	if err := os.WriteFile(testFile, []byte("not valid go code {{{"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	// JSON mode returns structured output, no Go error
	err := cmd.ExecuteWithArgs([]string{"--format=json", "validate", dir}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Unexpected error in JSON mode: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v\nOutput: %s", err, stdout.String())
	}

	if result["valid"] != false {
		t.Errorf("Expected valid=false, got %v", result["valid"])
	}

	files := result["files"].([]interface{})
	if len(files) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(files))
	}
	file := files[0].(map[string]interface{})
	if file["valid"] != false {
		t.Errorf("Expected file valid=false")
	}
	if file["error"] == nil || file["error"] == "" {
		t.Error("Expected error message")
	}
}

func TestCheckJSON_SkipAll(t *testing.T) {
	dir := t.TempDir()

	var stdout, stderr bytes.Buffer
	err := cmd.ExecuteWithArgs([]string{"--format=json", "check", "--skip-checks", dir}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("ExecuteWithArgs() error = %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v\nOutput: %s", err, stdout.String())
	}

	if result["passed"] != true {
		t.Errorf("Expected passed=true with skip-checks")
	}
}

func TestSubcommandHelp(t *testing.T) {
	subcommands := []string{"analyze", "generate", "check", "validate"}

	for _, subcmd := range subcommands {
		t.Run(subcmd, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := cmd.ExecuteWithArgs([]string{subcmd, "--help"}, &stdout, &stderr)
			if err != nil {
				t.Errorf("ExecuteWithArgs() error = %v", err)
			}

			output := stdout.String()
			if !strings.Contains(output, "Usage:") {
				t.Errorf("Expected usage info for %s", subcmd)
			}
		})
	}
}

func TestValidateJSONL(t *testing.T) {
	dir := t.TempDir()
	testFile1 := filepath.Join(dir, "test1.go")
	testFile2 := filepath.Join(dir, "test2.go")
	if err := os.WriteFile(testFile1, []byte("package test\n\nfunc Hello() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(testFile2, []byte("package test\n\nfunc World() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	err := cmd.ExecuteWithArgs([]string{"--format=jsonl", "validate", dir}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("ExecuteWithArgs() error = %v", err)
	}

	// JSONL should output one JSON object per line
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines of JSONL output, got %d: %s", len(lines), stdout.String())
	}

	// Each line should be valid JSON
	for i, line := range lines {
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			t.Errorf("Line %d is not valid JSON: %v", i+1, err)
		}
		if result["valid"] != true {
			t.Errorf("Line %d: expected valid=true", i+1)
		}
	}
}

func TestCheckJSONL(t *testing.T) {
	dir := t.TempDir()

	var stdout, stderr bytes.Buffer
	// Skip most checks, just run gofmt which is fast
	err := cmd.ExecuteWithArgs([]string{"--format=jsonl", "check", "--skip-vet", "--skip-lint", "--skip-sec", "--skip-build", "--skip-tests", dir}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("ExecuteWithArgs() error = %v", err)
	}

	// JSONL should output one JSON object per check
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) < 1 {
		t.Errorf("Expected at least 1 line of JSONL output, got: %s", stdout.String())
	}

	// Each line should be valid JSON with name and passed fields
	for i, line := range lines {
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			t.Errorf("Line %d is not valid JSON: %v", i+1, err)
		}
		if result["name"] == nil {
			t.Errorf("Line %d: expected name field", i+1)
		}
	}
}

func TestValidateYAML(t *testing.T) {
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.go")
	if err := os.WriteFile(testFile, []byte("package test\n\nfunc Hello() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	err := cmd.ExecuteWithArgs([]string{"--format=yaml", "validate", dir}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("ExecuteWithArgs() error = %v", err)
	}

	output := stdout.String()
	// YAML output should contain key: value pairs
	if !strings.Contains(output, "target:") {
		t.Errorf("Expected 'target:' in YAML output, got: %s", output)
	}
	if !strings.Contains(output, "valid: true") {
		t.Errorf("Expected 'valid: true' in YAML output, got: %s", output)
	}
}
