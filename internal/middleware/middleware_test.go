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

func TestAPIKeyMiddleware(t *testing.T) {
	// Not parallel: t.Setenv modifies global OS state; cannot mix with t.Parallel().
	// Each subtest sets its own API_KEY value before constructing the middleware.
	tests := []struct {
		name       string
		envKey     string
		headerKey  string
		path       string
		wantStatus int
	}{
		{
			name:       "correct key is accepted",
			envKey:     "secret",
			headerKey:  "secret",
			path:       "/products",
			wantStatus: http.StatusOK,
		},
		{
			name:       "wrong key returns 401",
			envKey:     "secret",
			headerKey:  "wrong",
			path:       "/products",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "missing key header returns 401",
			envKey:     "secret",
			headerKey:  "",
			path:       "/products",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "empty API_KEY env always returns 401",
			envKey:     "",
			headerKey:  "",
			path:       "/products",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "health path bypasses auth",
			envKey:     "secret",
			headerKey:  "",
			path:       "/health",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("API_KEY", tt.envKey)
			handler := middleware.APIKeyMiddleware(http.HandlerFunc(okHandler))

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if tt.headerKey != "" {
				req.Header.Set("X-API-key", tt.headerKey)
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestCORSMiddleware_Wildcard(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		method     string
		wantStatus int
	}{
		{
			name:       "OPTIONS preflight returns 204 with CORS headers",
			method:     http.MethodOptions,
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "GET passes through and receives CORS headers",
			method:     http.MethodGet,
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := middleware.CORSMiddleware([]string{"*"})(http.HandlerFunc(okHandler))
			req := httptest.NewRequest(tt.method, "/", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if got := w.Header().Get("Access-Control-Allow-Origin"); got != "*" {
				t.Errorf("Access-Control-Allow-Origin = %q, want *", got)
			}
			if got := w.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, PUT, DELETE, OPTIONS" {
				t.Errorf("Access-Control-Allow-Methods = %q, want %q", got, "GET, POST, PUT, DELETE, OPTIONS")
			}
			if got := w.Header().Get("Access-Control-Allow-Headers"); got != "X-API-Key, Content-Type" {
				t.Errorf("Access-Control-Allow-Headers = %q, want %q", got, "X-API-Key, Content-Type")
			}
		})
	}
}

func TestCORSMiddleware_SpecificOrigin(t *testing.T) {
	t.Parallel()

	const allowed = "https://app.example.com"
	handler := middleware.CORSMiddleware([]string{allowed})(http.HandlerFunc(okHandler))

	t.Run("matching origin echoed back", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Origin", allowed)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != allowed {
			t.Errorf("Allow-Origin = %q, want %q", got, allowed)
		}
		if got := w.Header().Get("Vary"); got != "Origin" {
			t.Errorf("Vary = %q, want %q", got, "Origin")
		}
	})

	t.Run("non-matching origin gets no CORS headers", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Origin", "https://evil.example.com")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("Allow-Origin = %q, want empty", got)
		}
	})
}

func TestCORSMiddleware_NoOrigins(t *testing.T) {
	t.Parallel()

	handler := middleware.CORSMiddleware(nil)(http.HandlerFunc(okHandler))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://anything.example.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin = %q, want empty when no origins configured", got)
	}
}
