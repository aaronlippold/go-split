package cmd

import (
	"testing"
)

func TestParseSourceAndTest(t *testing.T) {
	tests := []struct {
		name           string
		response       string
		expectedSource string
		expectedTest   string
	}{
		{
			name: "valid JSON",
			response: `{
  "source": "package main\n\nfunc Hello() {}",
  "test": "package main\n\nimport \"testing\"\n\nfunc TestHello(t *testing.T) {}"
}`,
			expectedSource: "package main\n\nfunc Hello() {}",
			expectedTest:   "package main\n\nimport \"testing\"\n\nfunc TestHello(t *testing.T) {}",
		},
		{
			name:           "JSON with surrounding text",
			response:       "Here is the code:\n{\"source\": \"package foo\", \"test\": \"package foo_test\"}\nDone.",
			expectedSource: "package foo",
			expectedTest:   "package foo_test",
		},
		{
			name: "JSON wrapped in markdown code fence",
			response: "```json\n{\n  \"source\": \"package main\\n\\nfunc Hello() {}\",\n  \"test\": \"package main\\n\\nfunc TestHello(t *testing.T) {}\"\n}\n```",
			expectedSource: "package main\n\nfunc Hello() {}",
			expectedTest:   "package main\n\nfunc TestHello(t *testing.T) {}",
		},
		{
			name:           "fallback to raw response",
			response:       "package main\n\nfunc Hello() {}",
			expectedSource: "package main\n\nfunc Hello() {}",
			expectedTest:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, test := parseSourceAndTest(tt.response)
			if source != tt.expectedSource {
				t.Errorf("source mismatch:\ngot:  %q\nwant: %q", source, tt.expectedSource)
			}
			if test != tt.expectedTest {
				t.Errorf("test mismatch:\ngot:  %q\nwant: %q", test, tt.expectedTest)
			}
		})
	}
}

func TestCountTestsInCode(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected int
	}{
		{
			name:     "no tests",
			code:     "package foo\n\nfunc Hello() {}",
			expected: 0,
		},
		{
			name:     "one test",
			code:     "package foo\n\nfunc TestHello(t *testing.T) {}",
			expected: 1,
		},
		{
			name: "multiple tests",
			code: `package foo

func TestHello(t *testing.T) {}

func TestWorld(t *testing.T) {}

func TestFoo(t *testing.T) {}`,
			expected: 3,
		},
		{
			name: "mixed functions",
			code: `package foo

func Hello() {}

func TestHello(t *testing.T) {}

func helper() {}

func TestWorld(t *testing.T) {}`,
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countTestsInCode(tt.code)
			if got != tt.expected {
				t.Errorf("countTestsInCode() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestCleanCode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "already clean",
			input:    "package main\n\nfunc Hello() {}",
			expected: "package main\n\nfunc Hello() {}",
		},
		{
			name:     "with markdown fence",
			input:    "```go\npackage main\n\nfunc Hello() {}\n```",
			expected: "package main\n\nfunc Hello() {}",
		},
		{
			name:     "with whitespace",
			input:    "\n  package main\n\nfunc Hello() {}  \n",
			expected: "package main\n\nfunc Hello() {}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanCode(tt.input)
			if got != tt.expected {
				t.Errorf("cleanCode() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestParseFilenames(t *testing.T) {
	tests := []struct {
		name     string
		response string
		expected []string
	}{
		{
			name:     "JSON array",
			response: `["types.go", "helpers.go", "handlers.go"]`,
			expected: []string{"types.go", "helpers.go", "handlers.go"},
		},
		{
			name:     "JSON array with surrounding text",
			response: "Here are the files:\n[\"types.go\", \"utils.go\"]\nDone.",
			expected: []string{"types.go", "utils.go"},
		},
		{
			name:     "fallback parsing",
			response: "Create types.go and helpers.go",
			expected: []string{"types.go", "helpers.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseFilenames(tt.response)
			if len(got) != len(tt.expected) {
				t.Errorf("parseFilenames() returned %d files, want %d", len(got), len(tt.expected))
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("parseFilenames()[%d] = %q, want %q", i, got[i], tt.expected[i])
				}
			}
		})
	}
}
