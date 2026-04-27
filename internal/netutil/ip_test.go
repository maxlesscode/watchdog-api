package netutil

import (
	"net"
	"testing"
)

func TestIsPrivateIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		ip      net.IP
		private bool
	}{
		// Private ranges
		{name: "10.0.0.1", ip: net.ParseIP("10.0.0.1"), private: true},
		{name: "10.255.255.255", ip: net.ParseIP("10.255.255.255"), private: true},
		{name: "172.16.0.1", ip: net.ParseIP("172.16.0.1"), private: true},
		{name: "172.31.255.255", ip: net.ParseIP("172.31.255.255"), private: true},
		{name: "192.168.1.1", ip: net.ParseIP("192.168.1.1"), private: true},
		{name: "192.168.0.0", ip: net.ParseIP("192.168.0.0"), private: true},
		// Loopback
		{name: "127.0.0.1", ip: net.ParseIP("127.0.0.1"), private: true},
		{name: "127.255.255.255", ip: net.ParseIP("127.255.255.255"), private: true},
		// Link-local IPv4 (cloud metadata: 169.254.169.254)
		{name: "169.254.0.1", ip: net.ParseIP("169.254.0.1"), private: true},
		{name: "169.254.169.254", ip: net.ParseIP("169.254.169.254"), private: true},
		// IPv6
		{name: "::1", ip: net.ParseIP("::1"), private: true},
		{name: "fe80::1", ip: net.ParseIP("fe80::1"), private: true},
		{name: "fc00::1", ip: net.ParseIP("fc00::1"), private: true},
		{name: "fd00::1", ip: net.ParseIP("fd00::1"), private: true},
		// Public
		{name: "8.8.8.8", ip: net.ParseIP("8.8.8.8"), private: false},
		{name: "1.1.1.1", ip: net.ParseIP("1.1.1.1"), private: false},
		{name: "93.184.216.34", ip: net.ParseIP("93.184.216.34"), private: false},
		{name: "2606:2800:220:1:248:1893:25c8:1946", ip: net.ParseIP("2606:2800:220:1:248:1893:25c8:1946"), private: false},
		// 172.16.0.0/12 boundaries
		{name: "172.15.255.255", ip: net.ParseIP("172.15.255.255"), private: false},
		{name: "172.32.0.0", ip: net.ParseIP("172.32.0.0"), private: false},
		// Nil is treated as private (fail-safe: unknown address is blocked)
		{name: "nil", ip: nil, private: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsPrivateIP(tt.ip); got != tt.private {
				t.Errorf("IsPrivateIP(%v) = %v, want %v", tt.ip, got, tt.private)
			}
		})
	}
}
