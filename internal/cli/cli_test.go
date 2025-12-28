package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/aaronlippold/go-split/internal/cli"
)

func TestRun_NoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := cli.Run([]string{}, &stdout, &stderr)

	if err == nil {
		t.Error("Run() expected error with no args")
	}

	if !strings.Contains(stderr.String(), "Usage:") {
		t.Error("Expected usage message in stderr")
	}
}

func TestRun_Help(t *testing.T) {
	tests := []struct {
		args []string
	}{
		{[]string{"-h"}},
		{[]string{"--help"}},
		{[]string{"help"}},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := cli.Run(tt.args, &stdout, &stderr)

			// Help should not return error
			if err != nil {
				t.Errorf("Run(%v) error = %v", tt.args, err)
			}

			output := stdout.String()
			if !strings.Contains(output, "go-split") {
				t.Errorf("Expected 'go-split' in output, got: %s", output)
			}
			if !strings.Contains(output, "analyze") {
				t.Errorf("Expected 'analyze' command in output, got: %s", output)
			}
		})
	}
}

func TestRun_Version(t *testing.T) {
	tests := []struct {
		args []string
	}{
		{[]string{"-v"}},
		{[]string{"--version"}},
		{[]string{"version"}},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := cli.Run(tt.args, &stdout, &stderr)

			if err != nil {
				t.Errorf("Run(%v) error = %v", tt.args, err)
			}

			output := stdout.String()
			if !strings.Contains(output, "go-split") {
				t.Errorf("Expected version info, got: %s", output)
			}
		})
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := cli.Run([]string{"unknown-command"}, &stdout, &stderr)

	if err == nil {
		t.Error("Run() expected error for unknown command")
	}

	if !strings.Contains(err.Error(), "unknown") {
		t.Errorf("Expected 'unknown' in error, got: %v", err)
	}
}

func TestRun_AnalyzeMissingFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := cli.Run([]string{"analyze"}, &stdout, &stderr)

	if err == nil {
		t.Error("Run() expected error when file not specified")
	}
}

func TestRun_AnalyzeNonexistentFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := cli.Run([]string{"analyze", "/nonexistent/file.go"}, &stdout, &stderr)

	if err == nil {
		t.Error("Run() expected error for nonexistent file")
	}
}
