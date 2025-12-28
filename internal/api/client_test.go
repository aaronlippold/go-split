package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aaronlippold/go-split/internal/api"
)

func TestClient_Call_Success(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request format
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", ct)
		}

		// Parse request
		var req api.Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		if req.Model != "test-model" {
			t.Errorf("Expected model test-model, got %s", req.Model)
		}

		// Send response
		resp := api.Response{
			Content: []api.ContentBlock{
				{Type: "text", Text: "Hello from test"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "test-model", 10*time.Second)
	result, err := client.Call("Test prompt", 100)
	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}

	if result != "Hello from test" {
		t.Errorf("Call() = %q, want %q", result, "Hello from test")
	}
}

func TestClient_Call_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": {"message": "Internal error"}}`))
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "test-model", 10*time.Second)
	_, err := client.Call("Test prompt", 100)
	if err == nil {
		t.Error("Call() expected error for 500 response")
	}
}

func TestClient_Call_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := api.Response{Content: []api.ContentBlock{}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "test-model", 10*time.Second)
	_, err := client.Call("Test prompt", 100)
	if err == nil {
		t.Error("Call() expected error for empty content")
	}
}

func TestClient_Call_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		json.NewEncoder(w).Encode(api.Response{
			Content: []api.ContentBlock{{Type: "text", Text: "slow"}},
		})
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "test-model", 10*time.Millisecond)
	_, err := client.Call("Test prompt", 100)
	if err == nil {
		t.Error("Call() expected timeout error")
	}
}
