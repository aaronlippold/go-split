// Package api provides a client for the Anthropic Messages API.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
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

const (
	maxRetries     = 3
	initialBackoff = 1 * time.Second
)

// Client is an API client for the Anthropic Messages API.
type Client struct {
	endpoint   string
	model      string
	timeout    time.Duration
	http       *http.Client
	captureDir string // If set, captures request/response to files
	// Direct API mode
	apiKey     string
	directMode bool
	anthropic  *anthropic.Client
}

// NewClient creates a new API client.
// Uses wrapper mode by default. Call WithAPIKey() to use direct Anthropic API.
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

// WithAPIKey enables direct Anthropic API mode.
// If key is empty, checks ANTHROPIC_API_KEY environment variable.
func (c *Client) WithAPIKey(key string) *Client {
	if key == "" {
		key = os.Getenv("ANTHROPIC_API_KEY")
	}
	if key != "" {
		c.apiKey = key
		c.directMode = true
		client := anthropic.NewClient(option.WithAPIKey(key))
		c.anthropic = &client
	}
	return c
}

// IsDirectMode returns true if using direct Anthropic API.
func (c *Client) IsDirectMode() bool {
	return c.directMode
}

// Call sends a prompt to the API and returns the response text.
func (c *Client) Call(prompt string, maxTokens int) (string, error) {
	var responseText string
	var err error

	if c.directMode {
		responseText, err = c.callDirect(prompt, maxTokens)
	} else {
		responseText, err = c.callWrapper(prompt, maxTokens)
	}

	if err != nil {
		return "", err
	}

	// Capture request/response if capture mode is enabled
	if c.captureDir != "" {
		if captureErr := c.captureExchange(prompt, responseText); captureErr != nil {
			// Log but don't fail on capture errors
			fmt.Fprintf(os.Stderr, "Warning: capture failed: %v\n", captureErr)
		}
	}

	return responseText, nil
}

// callWrapper calls the API via the claude-code-openai-wrapper.
func (c *Client) callWrapper(prompt string, maxTokens int) (string, error) {
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
	defer func() { _ = resp.Body.Close() }()

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

	return apiResp.Content[0].Text, nil
}

// callDirect calls the Anthropic API directly using the SDK.
func (c *Client) callDirect(prompt string, maxTokens int) (string, error) {
	if c.anthropic == nil {
		return "", fmt.Errorf("direct mode not initialized: call WithAPIKey first")
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	// Map model string to anthropic.Model
	model := c.mapModel()

	return c.callDirectWithRetry(ctx, prompt, maxTokens, model)
}

// mapModel converts the model string to anthropic.Model.
// The SDK accepts any string - Anthropic validates server-side.
func (c *Client) mapModel() anthropic.Model {
	return anthropic.Model(c.model)
}

// callDirectWithRetry implements retry with exponential backoff.
func (c *Client) callDirectWithRetry(ctx context.Context, prompt string, maxTokens int, model anthropic.Model) (string, error) {
	var lastErr error
	params := anthropic.MessageNewParams{
		Model:     model,
		MaxTokens: int64(maxTokens),
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := initialBackoff * time.Duration(math.Pow(2, float64(attempt-1)))
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}

		message, err := c.anthropic.Messages.New(ctx, params)

		if err == nil {
			if len(message.Content) > 0 {
				content := message.Content[0]
				if content.Type == "text" {
					return content.Text, nil
				}
				return "", fmt.Errorf("unexpected response format: not a text block (type=%s)", content.Type)
			}
			return "", fmt.Errorf("unexpected response format: no content blocks")
		}

		lastErr = err

		if ctx.Err() != nil {
			return "", ctx.Err()
		}

		if !isRetryable(err) {
			return "", fmt.Errorf("non-retryable error: %w", err)
		}
	}

	return "", fmt.Errorf("failed after %d retries: %w", maxRetries+1, lastErr)
}

// isRetryable determines if an error should trigger a retry.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	var apiErr *anthropic.Error
	if errors.As(err, &apiErr) {
		statusCode := apiErr.StatusCode
		// Retry on rate limit (429) or server errors (5xx)
		if statusCode == 429 || statusCode >= 500 {
			return true
		}
		return false
	}

	return false
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
