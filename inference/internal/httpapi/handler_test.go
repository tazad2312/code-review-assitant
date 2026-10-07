package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tazad0786/mr-review-assistant/inference/internal/review"
)

type testReviewer struct {
	comments []review.Comment
	err      error
}

func (r testReviewer) Review(context.Context, string) ([]review.Comment, error) {
	return r.comments, r.err
}

func TestReviewEndpoint(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		reviewer   Reviewer
		wantStatus int
	}{
		{name: "returns structured comments", method: http.MethodPost, path: "/v1/review", body: `{"diff":"patch"}`, reviewer: testReviewer{comments: []review.Comment{{File: "a.go", Line: 8, Severity: review.SeverityLow, Message: "Use a helper", Category: "style"}}}, wantStatus: http.StatusOK},
		{name: "requires diff", method: http.MethodPost, path: "/v1/review", body: `{"diff":" "}`, wantStatus: http.StatusBadRequest},
		{name: "rejects unknown fields", method: http.MethodPost, path: "/v1/review", body: `{"diff":"patch","other":true}`, wantStatus: http.StatusBadRequest},
		{name: "rejects malformed JSON", method: http.MethodPost, path: "/v1/review", body: `{"diff":`, wantStatus: http.StatusBadRequest},
		{name: "maps provider failure", method: http.MethodPost, path: "/v1/review", body: `{"diff":"patch"}`, reviewer: testReviewer{err: errors.New("unavailable")}, wantStatus: http.StatusBadGateway},
		{name: "health check", method: http.MethodGet, path: "/healthz", wantStatus: http.StatusOK},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := NewHandler(test.reviewer, time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			response := httptest.NewRecorder()
			handler.Routes().ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s", response.Code, test.wantStatus, response.Body)
			}
			if test.wantStatus == http.StatusOK && test.path == "/v1/review" {
				var result struct {
					Comments []review.Comment `json:"comments"`
				}
				if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				if len(result.Comments) != 1 || result.Comments[0].Line != 8 {
					t.Fatalf("unexpected response: %#v", result)
				}
			}
		})
	}
}

func TestReviewEndpointRejectsTrailingJSON(t *testing.T) {
	handler := NewHandler(testReviewer{}, time.Second, nil)
	request := httptest.NewRequest(http.MethodPost, "/v1/review", strings.NewReader(`{"diff":"patch"} {}`))
	response := httptest.NewRecorder()
	handler.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestRateLimiter(t *testing.T) {
	limiter := NewRateLimiter(1, time.Minute)
	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	for i, want := range []int{http.StatusNoContent, http.StatusTooManyRequests} {
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.RemoteAddr = "192.0.2.1:1234"
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != want {
			t.Fatalf("request %d status = %d, want %d", i+1, response.Code, want)
		}
	}
}
