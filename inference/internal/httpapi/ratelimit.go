package httpapi

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type rateWindow struct {
	start time.Time
	count int
}

type RateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	clients map[string]rateWindow
	now     func() time.Time
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	if limit < 1 {
		limit = 1
	}
	if window <= 0 {
		window = time.Minute
	}
	return &RateLimiter{limit: limit, window: window, clients: make(map[string]rateWindow), now: time.Now}
}

func (l *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		now := l.now()
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		l.mu.Lock()
		for client, state := range l.clients {
			if now.Sub(state.start) >= l.window {
				delete(l.clients, client)
			}
		}
		state := l.clients[host]
		if state.start.IsZero() || now.Sub(state.start) >= l.window {
			state = rateWindow{start: now}
		}
		state.count++
		l.clients[host] = state
		allowed := state.count <= l.limit
		retryAfter := int(l.window.Seconds())
		l.mu.Unlock()

		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		next.ServeHTTP(w, r)
	})
}
