package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/tazad0786/mr-review-assistant/inference/internal/review"
)

const maxRequestBytes = 2 << 20

type Reviewer interface {
	Review(context.Context, string) ([]review.Comment, error)
}

type Handler struct {
	reviewer       Reviewer
	requestTimeout time.Duration
	logger         *slog.Logger
}

func NewHandler(reviewer Reviewer, requestTimeout time.Duration, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{reviewer: reviewer, requestTimeout: requestTimeout, logger: logger}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /v1/review", h.review)
	return mux
}

type reviewRequest struct {
	Diff string `json:"diff"`
}

func (h *Handler) review(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request reviewRequest
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request")
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "request must contain one JSON object")
		return
	}
	if strings.TrimSpace(request.Diff) == "" {
		writeError(w, http.StatusBadRequest, "diff is required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.requestTimeout)
	defer cancel()
	comments, err := h.reviewer.Review(ctx, request.Diff)
	if err != nil {
		h.logger.Error("review failed", "error", err)
		writeError(w, http.StatusBadGateway, "review provider failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string][]review.Comment{"comments": comments})
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("write JSON response", "error", err)
	}
}
