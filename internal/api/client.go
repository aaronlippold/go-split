// Package api provides a client for the Anthropic Messages API.
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Request is the Anthropic Messages API request format.
type Request struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	Messages  []Message `json:"messages"`
}

// Message represents a single message in the conversation.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Response is the Anthropic Messages API response format.
type Response struct {
	Content []ContentBlock `json:"content"`
	Error   *APIError      `json:"error,omitempty"`
}

// ContentBlock is a content block in the response.
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// APIError represents an API error.
type APIError struct {
	Message string `json:"message"`
}

// Client is an API client for the Anthropic Messages API.
type Client struct {
	endpoint   string
	model      string
	timeout    time.Duration
	http       *http.Client
	captureDir string // If set, captures request/response to files
}

// NewClient creates a new API client.
func NewClient(endpoint, model string, timeout time.Duration) *Client {
	return &Client{
		endpoint: endpoint,
		model:    model,
		timeout:  timeout,
		http:     &http.Client{Timeout: timeout},
	}
}

// WithCapture enables capture mode, saving requests/responses to the given directory.
func (c *Client) WithCapture(dir string) *Client {
	c.captureDir = dir
	return c
}

// Call sends a prompt to the API and returns the response text.
func (c *Client) Call(prompt string, maxTokens int) (string, error) {
	req := Request{
		Model:     c.model,
		MaxTokens: maxTokens,
		Messages: []Message{
			{Role: "user", Content: prompt},
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var apiResp Response
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if apiResp.Error != nil {
		return "", fmt.Errorf("API error: %s", apiResp.Error.Message)
	}

	if len(apiResp.Content) == 0 {
		return "", fmt.Errorf("empty response from API")
	}

	responseText := apiResp.Content[0].Text

	// Capture request/response if capture mode is enabled
	if c.captureDir != "" {
		if err := c.captureExchange(prompt, responseText); err != nil {
			// Log but don't fail on capture errors
			fmt.Fprintf(os.Stderr, "Warning: capture failed: %v\n", err)
		}
	}

	return responseText, nil
}

// captureExchange saves the prompt and response to files in the capture directory.
func (c *Client) captureExchange(prompt, response string) error {
	if err := os.MkdirAll(c.captureDir, 0755); err != nil {
		return fmt.Errorf("create capture dir: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")

	// Write request (prompt)
	reqPath := filepath.Join(c.captureDir, fmt.Sprintf("%s_request.txt", timestamp))
	if err := os.WriteFile(reqPath, []byte(prompt), 0644); err != nil {
		return fmt.Errorf("write request: %w", err)
	}

	// Write response
	respPath := filepath.Join(c.captureDir, fmt.Sprintf("%s_response.txt", timestamp))
	if err := os.WriteFile(respPath, []byte(response), 0644); err != nil {
		return fmt.Errorf("write response: %w", err)
	}

	fmt.Fprintf(os.Stderr, "📝 Captured: %s\n", timestamp)
	return nil
}
