package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/tazad0786/mr-review-assistant/inference/internal/review"
)

const (
	defaultAnthropicURL     = "https://api.anthropic.com/v1/messages"
	defaultAnthropicVersion = "2023-06-01"
)

type AnthropicConfig struct {
	APIKey      string
	Model       string
	BaseURL     string
	MaxTokens   int
	MaxAttempts int
	Backoff     time.Duration
	HTTPClient  *http.Client
}

type AnthropicProvider struct {
	apiKey      string
	model       string
	baseURL     string
	maxTokens   int
	maxAttempts int
	backoff     time.Duration
	client      *http.Client
}

func NewAnthropicProvider(config AnthropicConfig) (*AnthropicProvider, error) {
	if strings.TrimSpace(config.APIKey) == "" {
		return nil, fmt.Errorf("Anthropic API key is required")
	}
	if strings.TrimSpace(config.Model) == "" {
		return nil, fmt.Errorf("Anthropic model is required")
	}
	if config.BaseURL == "" {
		config.BaseURL = defaultAnthropicURL
	}
	if config.MaxTokens < 1 {
		config.MaxTokens = 2048
	}
	if config.MaxAttempts < 1 {
		config.MaxAttempts = 3
	}
	if config.Backoff <= 0 {
		config.Backoff = 200 * time.Millisecond
	}
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{Timeout: 45 * time.Second}
	}
	return &AnthropicProvider{
		apiKey:      config.APIKey,
		model:       config.Model,
		baseURL:     config.BaseURL,
		maxTokens:   config.MaxTokens,
		maxAttempts: config.MaxAttempts,
		backoff:     config.Backoff,
		client:      config.HTTPClient,
	}, nil
}

func (p *AnthropicProvider) Review(ctx context.Context, diff string) ([]review.Comment, error) {
	body, err := json.Marshal(map[string]any{
		"model":      p.model,
		"max_tokens": p.maxTokens,
		"system":     "Review the supplied git diff for actionable defects. Return only a JSON object with a comments array. Each comment must contain file, positive line, severity (critical, high, medium, or low), message, and category. Do not report issues outside the diff.",
		"messages": []map[string]string{
			{"role": "user", "content": "Review this diff:\n\n" + diff},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("encode Anthropic request: %w", err)
	}

	var response anthropicResponse
	var lastErr error
	for attempt := 0; attempt < p.maxAttempts; attempt++ {
		response, lastErr = p.send(ctx, body)
		if lastErr == nil {
			break
		}
		if attempt+1 == p.maxAttempts || !isRetryable(lastErr) {
			return nil, lastErr
		}
		delay := time.Duration(math.Min(float64(p.backoff)*math.Pow(2, float64(attempt)), float64(5*time.Second)))
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}

	var text strings.Builder
	for _, block := range response.Content {
		if block.Type == "text" {
			text.WriteString(block.Text)
		}
	}
	comments, err := parseComments(text.String())
	if err != nil {
		return nil, fmt.Errorf("parse Anthropic review response: %w", err)
	}
	return comments, nil
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

type anthropicAPIError struct {
	statusCode int
	message    string
}

func (e anthropicAPIError) Error() string {
	return fmt.Sprintf("Anthropic API returned status %d: %s", e.statusCode, e.message)
}

func isRetryable(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var apiError anthropicAPIError
	if errors.As(err, &apiError) {
		return apiError.statusCode == http.StatusRequestTimeout ||
			apiError.statusCode == http.StatusTooManyRequests ||
			apiError.statusCode >= http.StatusInternalServerError
	}
	return true
}

func (p *AnthropicProvider) send(ctx context.Context, body []byte) (anthropicResponse, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL, bytes.NewReader(body))
	if err != nil {
		return anthropicResponse{}, fmt.Errorf("create Anthropic request: %w", err)
	}
	request.Header.Set("content-type", "application/json")
	request.Header.Set("x-api-key", p.apiKey)
	request.Header.Set("anthropic-version", defaultAnthropicVersion)

	response, err := p.client.Do(request)
	if err != nil {
		return anthropicResponse{}, fmt.Errorf("send Anthropic request: %w", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return anthropicResponse{}, fmt.Errorf("read Anthropic response: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		message := strings.TrimSpace(string(responseBody))
		if len(message) > 1024 {
			message = message[:1024]
		}
		return anthropicResponse{}, anthropicAPIError{statusCode: response.StatusCode, message: message}
	}
	var result anthropicResponse
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return anthropicResponse{}, fmt.Errorf("decode Anthropic response: %w", err)
	}
	return result, nil
}

func parseComments(text string) ([]review.Comment, error) {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```json")
		text = strings.TrimPrefix(text, "```")
		text = strings.TrimSuffix(strings.TrimSpace(text), "```")
	}
	start, end := strings.Index(text, "{"), strings.LastIndex(text, "}")
	if start < 0 || end < start {
		return nil, fmt.Errorf("response contains no JSON object")
	}
	decoder := json.NewDecoder(strings.NewReader(text[start : end+1]))
	decoder.DisallowUnknownFields()
	var result struct {
		Comments []review.Comment `json:"comments"`
	}
	if err := decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf("decode comments JSON: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("comments JSON contains trailing data")
	}
	if err := review.ValidateComments(result.Comments); err != nil {
		return nil, err
	}
	return result.Comments, nil
}
