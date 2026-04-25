package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(body))
	}))
}

func TestFetchPrice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		html      string
		selector  string
		serverFn  func(w http.ResponseWriter, r *http.Request)
		wantPrice float64
		wantErr   bool
	}{
		{
			name: "json-ld price as float",
			html: `<html><head><script type="application/ld+json">{"offers":{"price":29.99}}</script></head></html>`,
			wantPrice: 29.99,
		},
		{
			name: "json-ld price as string",
			html: `<html><head><script type="application/ld+json">{"offers":{"price":"49.99"}}</script></head></html>`,
			wantPrice: 49.99,
		},
		{
			name:      "css selector fallback",
			html:      `<html><body><span class="price">29,99 €</span></body></html>`,
			selector:  "span.price",
			wantPrice: 29.99,
		},
		{
			name:      "eu price format via selector",
			html:      `<html><body><span id="p">1.299,99 €</span></body></html>`,
			selector:  "span#p",
			wantPrice: 1299.99,
		},
		{
			name:      "us price format via selector",
			html:      `<html><body><div class="price">$1,299.99</div></body></html>`,
			selector:  "div.price",
			wantPrice: 1299.99,
		},
		{
			name:     "no json-ld no selector",
			html:     `<html><body><span>29.99</span></body></html>`,
			selector: "",
			wantErr:  true,
		},
		{
			name:     "selector matches nothing",
			html:     `<html><body></body></html>`,
			selector: "div.nope",
			wantErr:  true,
		},
		{
			name: "http 500",
			serverFn: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var srv *httptest.Server
			if tt.serverFn != nil {
				srv = httptest.NewServer(http.HandlerFunc(tt.serverFn))
			} else {
				srv = newServer(t, tt.html)
			}
			defer srv.Close()

			scraper := NewHTMLScraper(srv.Client())
			price, err := scraper.FetchPrice(context.Background(), srv.URL, tt.selector)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got price %v", price)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if price != tt.wantPrice {
				t.Fatalf("got price %v, want %v", price, tt.wantPrice)
			}
		})
	}
}

func TestFetchPriceContextCancelled(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		_, _ = w.Write([]byte(`<html></html>`))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	scraper := NewHTMLScraper(srv.Client())
	_, err := scraper.FetchPrice(ctx, srv.URL, "")
	if err == nil {
		t.Fatal("expected error on cancelled context, got nil")
	}
}

func TestParsePrice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  float64
		err   bool
	}{
		{"29.99", 29.99, false},
		{"29,99", 29.99, false},
		{"€29.99", 29.99, false},
		{"$1,299.99", 1299.99, false},
		{"1.299,99 €", 1299.99, false},
		{"1 299,99", 1299.99, false},
		{"not a price", 0, true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got, err := parsePrice(tt.input)
			if tt.err {
				if err == nil {
					t.Fatalf("expected error for input %q", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("parsePrice(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
