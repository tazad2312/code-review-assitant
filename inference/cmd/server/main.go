package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/tazad0786/mr-review-assistant/inference/internal/httpapi"
	"github.com/tazad0786/mr-review-assistant/inference/internal/provider"
	"github.com/tazad0786/mr-review-assistant/inference/internal/review"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cacheSize := envInt("CACHE_SIZE", 256)
	tokenBudget := envInt("TOKEN_BUDGET", 6000)
	requestTimeout := envDuration("REQUEST_TIMEOUT", 45*time.Second)
	var llm review.Provider
	switch envString("REVIEW_PROVIDER", "mock") {
	case "mock":
		llm = &review.MockProvider{}
	case "anthropic":
		var err error
		llm, err = provider.NewAnthropicProvider(provider.AnthropicConfig{
			APIKey:      os.Getenv("ANTHROPIC_API_KEY"),
			Model:       envString("ANTHROPIC_MODEL", "claude-3-5-haiku-latest"),
			MaxTokens:   envInt("ANTHROPIC_MAX_TOKENS", 2048),
			MaxAttempts: envInt("ANTHROPIC_MAX_ATTEMPTS", 3),
			Backoff:     envDuration("ANTHROPIC_RETRY_BACKOFF", 200*time.Millisecond),
			HTTPClient:  &http.Client{Timeout: requestTimeout},
		})
		if err != nil {
			logger.Error("configure Anthropic provider", "error", err)
			os.Exit(1)
		}
	default:
		logger.Error("unsupported review provider", "provider", os.Getenv("REVIEW_PROVIDER"))
		os.Exit(1)
	}

	reviewer, err := review.NewService(llm, review.NewLRUCache(cacheSize), tokenBudget)
	if err != nil {
		logger.Error("configure review service", "error", err)
		os.Exit(1)
	}
	handler := httpapi.NewHandler(reviewer, requestTimeout, logger)
	routes := httpapi.NewRateLimiter(envInt("RATE_LIMIT_PER_MINUTE", 30), time.Minute).Middleware(handler.Routes())
	server := &http.Server{
		Addr:              envString("HTTP_ADDR", ":8080"),
		Handler:           routes,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      requestTimeout + 5*time.Second,
		IdleTimeout:       60 * time.Second,
	}
	stopped := make(chan os.Signal, 1)
	signal.Notify(stopped, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stopped
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
		}
	}()

	logger.Info("inference service listening", "addr", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("HTTP server failed", "error", err)
		os.Exit(1)
	}
}

func envString(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil || value < 1 {
		return fallback
	}
	return value
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value, err := time.ParseDuration(os.Getenv(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
