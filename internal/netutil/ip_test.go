package netutil

import (
	"net"
	"testing"
)

func TestIsPrivateIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		ip      string
		private bool
	}{
		// Private ranges
		{"10.0.0.1", true},
		{"10.255.255.255", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"192.168.1.1", true},
		{"192.168.0.0", true},
		// Loopback
		{"127.0.0.1", true},
		{"127.255.255.255", true},
		// Link-local IPv4 (cloud metadata: 169.254.169.254)
		{"169.254.0.1", true},
		{"169.254.169.254", true},
		// IPv6
		{"::1", true},
		{"fe80::1", true},
		{"fc00::1", true},
		{"fd00::1", true},
		// Public
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"93.184.216.34", false}, // example.com
		{"2606:2800:220:1:248:1893:25c8:1946", false},
		// Boundary cases
		{"172.15.255.255", false}, // just below lower bound of 172.16.0.0/12
		{"172.32.0.0", false},     // just above upper bound of 172.31.255.255/12
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			t.Parallel()
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("net.ParseIP(%q) returned nil", tt.ip)
			}
			if got := IsPrivateIP(ip); got != tt.private {
				t.Errorf("IsPrivateIP(%s) = %v, want %v", tt.ip, got, tt.private)
			}
		})
	}
}

func TestIsPrivateIP_Nil(t *testing.T) {
	// A nil IP is not a valid address; treat it as private to fail safe.
	if got := IsPrivateIP(nil); !got {
		t.Error("IsPrivateIP(nil) = false, want true")
	}
}
