package scraper

import (
	"context"
	"errors"
	"testing"
	"time"
)

type mockScraper struct {
	calls int
	errs  []error
	price float64
}

func (m *mockScraper) FetchPrice(_ context.Context, _, _ string) (float64, error) {
	i := m.calls
	m.calls++
	if i < len(m.errs) {
		return 0, m.errs[i]
	}
	return m.price, nil
}

var errFetch = errors.New("fetch failed")

func fastConfig(maxAttempts, threshold int) ResilientConfig {
	return ResilientConfig{
		MaxAttempts:      maxAttempts,
		InitialDelay:     time.Millisecond,
		CircuitThreshold: threshold,
		CircuitCooldown:  time.Hour,
	}
}

func TestResilientScraper_Success(t *testing.T) {
	t.Parallel()

	inner := &mockScraper{price: 42.0}
	s := NewResilientScraper(inner, fastConfig(3, 5))

	price, err := s.FetchPrice(context.Background(), "https://example.com", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if price != 42.0 {
		t.Errorf("price = %v, want 42.0", price)
	}
	if inner.calls != 1 {
		t.Errorf("calls = %d, want 1", inner.calls)
	}
}

func TestResilientScraper_RetrySucceeds(t *testing.T) {
	t.Parallel()

	inner := &mockScraper{
		errs:  []error{errFetch, errFetch},
		price: 99.0,
	}
	s := NewResilientScraper(inner, fastConfig(3, 5))

	price, err := s.FetchPrice(context.Background(), "https://example.com", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if price != 99.0 {
		t.Errorf("price = %v, want 99.0", price)
	}
	if inner.calls != 3 {
		t.Errorf("calls = %d, want 3", inner.calls)
	}
}

func TestResilientScraper_AllAttemptsFail(t *testing.T) {
	t.Parallel()

	inner := &mockScraper{errs: []error{errFetch, errFetch, errFetch}}
	s := NewResilientScraper(inner, fastConfig(3, 5))

	_, err := s.FetchPrice(context.Background(), "https://example.com", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if inner.calls != 3 {
		t.Errorf("calls = %d, want 3", inner.calls)
	}
}

func TestResilientScraper_CircuitOpens(t *testing.T) {
	t.Parallel()

	// threshold=2: two complete failures → circuit opens
	inner := &mockScraper{errs: []error{
		errFetch, errFetch, errFetch, // first call exhausts 3 attempts
		errFetch, errFetch, errFetch, // second call exhausts 3 attempts → failures=2 → open
	}}
	s := NewResilientScraper(inner, fastConfig(3, 2))

	if _, err := s.FetchPrice(context.Background(), "https://example.com", ""); err == nil {
		t.Fatal("expected error on first call")
	}
	if _, err := s.FetchPrice(context.Background(), "https://example.com", ""); err == nil {
		t.Fatal("expected error on second call")
	}

	// third call: circuit is open — inner must not be called
	callsBefore := inner.calls
	_, err := s.FetchPrice(context.Background(), "https://example.com", "")
	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("err = %v, want ErrCircuitOpen", err)
	}
	if inner.calls != callsBefore {
		t.Error("inner scraper called despite open circuit")
	}
}

func TestResilientScraper_SuccessResetsCircuit(t *testing.T) {
	t.Parallel()

	inner := &mockScraper{
		errs:  []error{errFetch, errFetch, errFetch},
		price: 10.0,
	}
	s := NewResilientScraper(inner, fastConfig(3, 5))

	// accumulate one failure
	s.FetchPrice(context.Background(), "https://example.com", "") //nolint

	// succeed — failures should reset
	inner.errs = nil
	inner.calls = 0
	if _, err := s.FetchPrice(context.Background(), "https://example.com", ""); err != nil {
		t.Fatalf("unexpected error after reset: %v", err)
	}

	s.mu.Lock()
	st := s.states["example.com"]
	s.mu.Unlock()
	if st != nil && st.failures != 0 {
		t.Errorf("failures = %d after success, want 0", st.failures)
	}
}

func TestResilientScraper_ContextCancelledDuringBackoff(t *testing.T) {
	t.Parallel()

	inner := &mockScraper{errs: []error{errFetch, errFetch, errFetch}}
	s := NewResilientScraper(inner, ResilientConfig{
		MaxAttempts:      3,
		InitialDelay:     time.Hour, // long enough that cancel fires first
		CircuitThreshold: 5,
		CircuitCooldown:  time.Hour,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel

	_, err := s.FetchPrice(ctx, "https://example.com", "")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}
