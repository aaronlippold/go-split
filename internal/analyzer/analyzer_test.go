package analyzer_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aaronlippold/go-split/internal/analyzer"
)

func TestCountLines(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected int
	}{
		{"empty file", "", 1},
		{"single line", "package main", 1},
		{"two lines", "package main\n", 2},
		{"multiple lines", "package main\n\nfunc main() {\n}\n", 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := analyzer.CountLines(tt.content)
			if got != tt.expected {
				t.Errorf("CountLines() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestParseGoFile(t *testing.T) {
	// Create a temp file with valid Go code
	content := `package main

import "fmt"

// HelloFunc says hello
func HelloFunc() {
	fmt.Println("hello")
}

// GoodbyeFunc says goodbye
func GoodbyeFunc() {
	fmt.Println("goodbye")
}

var globalVar = "test"

type MyStruct struct {
	Name string
}
`
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	info, err := analyzer.ParseGoFile(tmpFile)
	if err != nil {
		t.Fatalf("ParseGoFile() error = %v", err)
	}

	if info.Package != "main" {
		t.Errorf("Package = %q, want %q", info.Package, "main")
	}

	if len(info.Functions) != 2 {
		t.Errorf("Functions count = %d, want 2", len(info.Functions))
	}

	if len(info.Types) != 1 {
		t.Errorf("Types count = %d, want 1", len(info.Types))
	}

	if len(info.Vars) != 1 {
		t.Errorf("Vars count = %d, want 1", len(info.Vars))
	}
}

func TestParseGoFile_Invalid(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "invalid.go")
	if err := os.WriteFile(tmpFile, []byte("not valid go code {{{"), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	_, err := analyzer.ParseGoFile(tmpFile)
	if err == nil {
		t.Error("ParseGoFile() expected error for invalid Go code")
	}
}

func TestParseGoFile_NotFound(t *testing.T) {
	_, err := analyzer.ParseGoFile("/nonexistent/file.go")
	if err == nil {
		t.Error("ParseGoFile() expected error for nonexistent file")
	}
}
