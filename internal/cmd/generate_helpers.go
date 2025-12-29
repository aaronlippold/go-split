package cmd

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// parseFilenames extracts Go filenames from an AI response.
// It tries JSON array parsing first, then falls back to word extraction.
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

// cleanCode removes markdown fences and trims whitespace from code.
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

// parseSourceAndTest extracts source and test code from a JSON response.
func parseSourceAndTest(response string) (source, test string) {
	// Strip markdown code fences first (AI often wraps JSON in ```json ... ```)
	cleaned := stripMarkdownFence(response)

	// Try to parse as JSON first
	var result struct {
		Source string `json:"source"`
		Test   string `json:"test"`
	}
	if err := json.Unmarshal([]byte(cleaned), &result); err == nil && result.Source != "" {
		return result.Source, result.Test
	}

	// Fallback: try to find JSON object with "source" key in the response
	start := strings.Index(cleaned, "{")
	end := strings.LastIndex(cleaned, "}")
	if start >= 0 && end > start {
		jsonStr := cleaned[start : end+1]
		// Only try if it looks like our expected JSON structure
		if strings.Contains(jsonStr, `"source"`) {
			if err := json.Unmarshal([]byte(jsonStr), &result); err == nil && result.Source != "" {
				return result.Source, result.Test
			}
		}
	}

	// Last resort: return the whole response as source
	return response, ""
}

// stripMarkdownFence removes markdown code fences from a response.
func stripMarkdownFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		lines := strings.Split(s, "\n")
		if len(lines) > 2 {
			// Remove first line (```json or ```)
			lines = lines[1:]
			// Remove last line if it's just ```
			if strings.TrimSpace(lines[len(lines)-1]) == "```" {
				lines = lines[:len(lines)-1]
			}
			return strings.Join(lines, "\n")
		}
	}
	return s
}

// countTestsInCode counts Test* functions in Go test code.
func countTestsInCode(code string) int {
	count := 0
	lines := strings.Split(code, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "func Test") {
			count++
		}
	}
	return count
}

// runValidation runs go test on the output directory.
func runValidation(dir string) error {
	cmd := exec.Command("go", "test", "./...")
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
