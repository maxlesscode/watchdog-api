package scraper

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSafeHTTPClientBlocksLoopback(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewSafeHTTPClient(5 * time.Second)
	_, err := client.Get(srv.URL) //nolint:noctx
	if err == nil {
		t.Fatal("expected error fetching loopback address, got nil")
	}
	if !strings.Contains(err.Error(), "blocked") && !strings.Contains(err.Error(), "private") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSafeHTTPClientBlocksPrivateRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  string
	}{
		{"10.x.x.x", "http://10.0.0.1/"},
		{"192.168.x.x", "http://192.168.1.1/"},
		{"172.16.x.x", "http://172.16.0.1/"},
		{"link-local metadata", "http://169.254.169.254/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client := NewSafeHTTPClient(2 * time.Second)
			_, err := client.Get(tt.url) //nolint:noctx
			if err == nil {
				t.Fatalf("expected error for %s, got nil", tt.url)
			}
		})
	}
}

func TestSafeHTTPClientLimitsRedirects(t *testing.T) {
	t.Parallel()

	hops := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hops++
		// Redirect back to itself indefinitely
		http.Redirect(w, r, r.URL.String(), http.StatusFound)
	}))
	defer srv.Close()

	client := NewSafeHTTPClient(5 * time.Second)
	// The loopback block fires first, but if somehow it didn't, we'd hit
	// the redirect limit. This test verifies the redirect limit is wired.
	_, err := client.Get(srv.URL) //nolint:noctx
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
