package scraper

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/maxlesscode/watchdog/internal/netutil"
)

// NewSafeHTTPClient returns an http.Client that blocks outbound connections to
// private, loopback, and link-local IP ranges, preventing SSRF. Redirects are
// followed but also subject to the same IP check via DialContext.
func NewSafeHTTPClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("split host port %s: %w", addr, err)
			}

			ips, err := net.DefaultResolver.LookupHost(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("resolve %s: %w", host, err)
			}
			if len(ips) == 0 {
				return nil, fmt.Errorf("no addresses for %s", host)
			}

			for _, raw := range ips {
				if ip := net.ParseIP(raw); ip != nil && netutil.IsPrivateIP(ip) {
					return nil, fmt.Errorf("blocked: %s resolves to private address %s", host, raw)
				}
			}

			// Connect directly to the first resolved IP to avoid a second DNS
			// lookup inside the standard dialer.
			return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0], port))
		},
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
}
