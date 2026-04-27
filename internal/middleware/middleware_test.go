package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	apierrors "github.com/maxlesscode/watchdog/internal/errors"
	"github.com/maxlesscode/watchdog/internal/middleware"
)

func okHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func TestLoggingMiddleware_SetsRequestID(t *testing.T) {
	t.Parallel()

	handler := middleware.LoggingMiddleware(http.HandlerFunc(okHandler))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	id := w.Header().Get("X-Request-ID")
	if id == "" {
		t.Error("X-Request-ID header not set on response")
	}
	if len(id) != 16 {
		t.Errorf("X-Request-ID length = %d, want 16", len(id))
	}
}

func TestLoggingMiddleware_EchoesExistingRequestID(t *testing.T) {
	t.Parallel()

	const clientID = "abc123def456abcd"
	handler := middleware.LoggingMiddleware(http.HandlerFunc(okHandler))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", clientID)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if got := w.Header().Get("X-Request-ID"); got != clientID {
		t.Errorf("X-Request-ID = %q, want %q", got, clientID)
	}
}

func TestLoggingMiddleware_StoresRequestIDInContext(t *testing.T) {
	t.Parallel()

	var ctxID string
	handler := middleware.LoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxID = apierrors.RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if ctxID == "" {
		t.Error("request ID not stored in context")
	}
	if ctxID != w.Header().Get("X-Request-ID") {
		t.Errorf("ctx ID %q != response header %q", ctxID, w.Header().Get("X-Request-ID"))
	}
}

func TestRateLimitMiddleware_AllowsUnderBurst(t *testing.T) {
	t.Parallel()

	// burst=5, rps tiny — 5 requests should all pass
	handler := middleware.RateLimitMiddleware(context.Background(), 0.001, 5)(http.HandlerFunc(okHandler))

	for i := range 5 {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		// RemoteAddr is loopback (trusted proxy) so XFF is honoured.
		req.RemoteAddr = "127.0.0.1:12345"
		req.Header.Set("X-Forwarded-For", "10.0.0.1")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("request %d: status = %d, want 200", i+1, w.Code)
		}
	}
}

func TestRateLimitMiddleware_Blocks429WhenExceeded(t *testing.T) {
	t.Parallel()

	// burst=1, rps tiny — second request from same IP must be 429
	handler := middleware.RateLimitMiddleware(context.Background(), 0.001, 1)(http.HandlerFunc(okHandler))

	for i, want := range []int{http.StatusOK, http.StatusTooManyRequests} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		req.Header.Set("X-Forwarded-For", "10.0.0.2")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != want {
			t.Errorf("request %d: status = %d, want %d", i+1, w.Code, want)
		}
	}
}

func TestRateLimitMiddleware_SeparateBucketsPerIP(t *testing.T) {
	t.Parallel()

	// burst=1 — each IP gets its own bucket, so both first requests should succeed
	handler := middleware.RateLimitMiddleware(context.Background(), 0.001, 1)(http.HandlerFunc(okHandler))

	for _, ip := range []string{"10.0.1.1", "10.0.1.2"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		req.Header.Set("X-Forwarded-For", ip)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("IP %s: status = %d, want 200", ip, w.Code)
		}
	}
}
