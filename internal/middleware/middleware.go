package middleware

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	apierrors "github.com/maxlesscode/watchdog/internal/errors"
	"github.com/maxlesscode/watchdog/internal/netutil"
	"golang.org/x/time/rate"
)

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// APIKeyMiddleware rejects requests missing a valid X-API-Key header. /health is exempted.
func APIKeyMiddleware(next http.Handler) http.Handler {
	apiKey := os.Getenv("API_KEY")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		if apiKey == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-API-key")), []byte(apiKey)) != 1 {
			slog.Warn("wrong api key")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// CORSMiddleware returns a middleware that sets CORS headers based on the allowed origins list.
// Pass "*" as a single entry for wildcard (dev only). An empty list disables CORS headers entirely.
// For specific origins, the request's Origin header is echoed back only when it matches.
func CORSMiddleware(origins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(origins))
	wildcard := false
	for _, o := range origins {
		if o == "*" {
			wildcard = true
		}
		allowed[o] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if wildcard {
				w.Header().Set("Access-Control-Allow-Origin", "*")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "X-API-Key, Content-Type")
			} else if origin := r.Header.Get("Origin"); origin != "" {
				if _, ok := allowed[origin]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "X-API-Key, Content-Type")
					w.Header().Add("Vary", "Origin")
				}
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// LoggingMiddleware logs each request's method, path, status, and duration. It also generates and propagates a request ID.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = newRequestID()
		}

		ctx := apierrors.WithRequestID(r.Context(), id)
		r = r.WithContext(ctx)
		w.Header().Set("X-Request-ID", id)

		rw := &responseWriter{w, http.StatusOK}
		next.ServeHTTP(rw, r)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.status,
			"duration", time.Since(start),
			"request_id", id,
		)
	})
}

type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type rateLimiterStore struct {
	mu       sync.Mutex
	limiters map[string]*limiterEntry
	rps      rate.Limit
	burst    int
}

func (s *rateLimiterStore) get(ip string) *rate.Limiter {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.limiters[ip]
	if !ok {
		e = &limiterEntry{limiter: rate.NewLimiter(s.rps, s.burst)}
		s.limiters[ip] = e
	}
	e.lastSeen = time.Now()
	return e.limiter
}

func (s *rateLimiterStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := time.Now().Add(-1 * time.Hour)
	for ip, e := range s.limiters {
		if e.lastSeen.Before(cutoff) {
			delete(s.limiters, ip)
		}
	}
}

// RateLimitMiddleware enforces per-IP request rate limiting.
// ctx controls the lifetime of the background cleanup goroutine.
func RateLimitMiddleware(ctx context.Context, rps float64, burst int) func(http.Handler) http.Handler {
	store := &rateLimiterStore{
		limiters: make(map[string]*limiterEntry),
		rps:      rate.Limit(rps),
		burst:    burst,
	}

	go func() {
		t := time.NewTicker(10 * time.Minute)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				store.cleanup()
			}
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !store.get(realIP(r)).Allow() {
				apierrors.SendError(r.Context(), w, apierrors.ErrorInput{
					Code:    http.StatusTooManyRequests,
					Tech:    apierrors.CodeRateLimitExceeded,
					Message: "too many requests",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func newRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func realIP(r *http.Request) string {
	remoteHost, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		remoteHost = r.RemoteAddr
	}

	// Only trust X-Forwarded-For when the direct caller is a trusted proxy.
	if remoteIP := net.ParseIP(remoteHost); remoteIP != nil && netutil.IsPrivateIP(remoteIP) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if first, _, ok := strings.Cut(xff, ","); ok {
				return strings.TrimSpace(first)
			}
			return strings.TrimSpace(xff)
		}
	}

	return remoteHost
}
