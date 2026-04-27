package scraper

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"
)

// ErrCircuitOpen is returned when a domain's circuit breaker is open.
var ErrCircuitOpen = errors.New("circuit open: too many consecutive failures")

// ResilientConfig controls retry and circuit-breaker behaviour.
type ResilientConfig struct {
	MaxAttempts      int           // number of attempts before giving up (default 3)
	InitialDelay     time.Duration // first backoff delay; doubles each retry (default 1s)
	CircuitThreshold int           // consecutive failures to open the circuit (default 5)
	CircuitCooldown  time.Duration // how long circuit stays open before reset (default 5m)
}

func (c *ResilientConfig) setDefaults() {
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = 3
	}
	if c.InitialDelay <= 0 {
		c.InitialDelay = time.Second
	}
	if c.CircuitThreshold <= 0 {
		c.CircuitThreshold = 5
	}
	if c.CircuitCooldown <= 0 {
		c.CircuitCooldown = 5 * time.Minute
	}
}

type circuitState struct {
	failures  int
	openUntil time.Time
}

// ResilientScraper wraps a Scraper with per-domain circuit breaking and
// exponential-backoff retries.
type ResilientScraper struct {
	inner  Scraper
	cfg    ResilientConfig
	mu     sync.Mutex
	states map[string]*circuitState
}

// NewResilientScraper returns a ResilientScraper wrapping inner.
func NewResilientScraper(inner Scraper, cfg ResilientConfig) *ResilientScraper {
	cfg.setDefaults()
	return &ResilientScraper{
		inner:  inner,
		cfg:    cfg,
		states: make(map[string]*circuitState),
	}
}

// FetchPrice implements Scraper with retries and circuit breaking.
func (s *ResilientScraper) FetchPrice(ctx context.Context, rawURL, selector string) (float64, error) {
	domain := extractDomain(rawURL)

	if s.isOpen(domain) {
		return 0, fmt.Errorf("scrape %s: %w", domain, ErrCircuitOpen)
	}

	var lastErr error
	delay := s.cfg.InitialDelay

	for attempt := 1; attempt <= s.cfg.MaxAttempts; attempt++ {
		price, err := s.inner.FetchPrice(ctx, rawURL, selector)
		if err == nil {
			s.recordSuccess(domain)
			return price, nil
		}
		lastErr = err

		if attempt < s.cfg.MaxAttempts {
			t := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				t.Stop()
				return 0, ctx.Err()
			case <-t.C:
			}
			t.Stop()
			delay *= 2
		}
	}

	s.recordFailure(domain)
	return 0, lastErr
}

func (s *ResilientScraper) isOpen(domain string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.states[domain]
	if !ok {
		return false
	}
	return st.failures >= s.cfg.CircuitThreshold && time.Now().Before(st.openUntil)
}

func (s *ResilientScraper) recordSuccess(domain string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if st, ok := s.states[domain]; ok {
		st.failures = 0
	}
}

func (s *ResilientScraper) recordFailure(domain string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.states[domain]
	if !ok {
		st = &circuitState{}
		s.states[domain] = st
	}
	st.failures++
	if st.failures >= s.cfg.CircuitThreshold {
		st.openUntil = time.Now().Add(s.cfg.CircuitCooldown)
	}
}

func extractDomain(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return u.Host
}
