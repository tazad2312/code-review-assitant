package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tazad0786/mr-review-assistant/inference/internal/review"
)

func TestParseComments(t *testing.T) {
	valid := `{"comments":[{"file":"src/main.go","line":7,"severity":"high","message":"Handle this error.","category":"bug"}]}`
	tests := []struct {
		name    string
		text    string
		wantLen int
		wantErr bool
	}{
		{name: "valid JSON", text: valid, wantLen: 1},
		{name: "JSON in a fenced code block", text: "```json\n" + valid + "\n```", wantLen: 1},
		{name: "no JSON", text: "No findings.", wantErr: true},
		{name: "invalid JSON", text: `{"comments":[`, wantErr: true},
		{name: "unknown field", text: `{"comments":[{"file":"x","line":1,"severity":"low","message":"m","category":"style","extra":true}]}`, wantErr: true},
		{name: "invalid comment", text: `{"comments":[{"file":"x","line":0,"severity":"low","message":"m","category":"style"}]}`, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			comments, err := parseComments(test.text)
			if (err != nil) != test.wantErr {
				t.Fatalf("parseComments() error = %v, wantErr %v", err, test.wantErr)
			}
			if len(comments) != test.wantLen {
				t.Fatalf("parseComments() returned %d comments, want %d", len(comments), test.wantLen)
			}
		})
	}
}

func TestAnthropicProviderRetriesServerError(t *testing.T) {
	var requests atomic.Int32
	commentJSON, err := json.Marshal(map[string][]review.Comment{
		"comments": {{File: "a.go", Line: 3, Severity: review.SeverityMedium, Message: "Check this branch.", Category: "bug"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("x-api-key = %q", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") != defaultAnthropicVersion {
			t.Errorf("anthropic-version = %q", r.Header.Get("anthropic-version"))
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if requests.Add(1) == 1 {
			http.Error(w, "temporarily unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"content":[{"type":"text","text":%q}]}`, string(commentJSON))
	}))
	defer server.Close()

	provider, err := NewAnthropicProvider(AnthropicConfig{
		APIKey: "test-key", Model: "test-model", BaseURL: server.URL,
		MaxAttempts: 2, Backoff: time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	comments, err := provider.Review(context.Background(), "diff")
	if err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 2 {
		t.Fatalf("requests = %d, want 2", requests.Load())
	}
	if len(comments) != 1 || comments[0].Line != 3 {
		t.Fatalf("unexpected comments: %#v", comments)
	}
}

func TestAnthropicProviderDoesNotRetryClientError(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		http.Error(w, "invalid API key", http.StatusUnauthorized)
	}))
	defer server.Close()
	provider, err := NewAnthropicProvider(AnthropicConfig{
		APIKey: "test-key", Model: "test-model", BaseURL: server.URL,
		MaxAttempts: 4, Backoff: time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = provider.Review(context.Background(), "diff")
	if err == nil || !strings.Contains(err.Error(), "status 401") {
		t.Fatalf("Review() error = %v, want status 401", err)
	}
	if requests.Load() != 1 {
		t.Fatalf("requests = %d, want 1", requests.Load())
	}
}

func TestNewAnthropicProviderRequiresCredentials(t *testing.T) {
	if _, err := NewAnthropicProvider(AnthropicConfig{Model: "test-model"}); err == nil {
		t.Fatal("expected missing API key to fail")
	}
	if _, err := NewAnthropicProvider(AnthropicConfig{APIKey: "key"}); err == nil {
		t.Fatal("expected missing model to fail")
	}
}
