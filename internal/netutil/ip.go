package netutil

import "net"

// privateRanges lists IP ranges that must never be the target of outbound requests.
var privateRanges = []net.IPNet{
	{IP: net.ParseIP("10.0.0.0"), Mask: net.CIDRMask(8, 32)},
	{IP: net.ParseIP("172.16.0.0"), Mask: net.CIDRMask(12, 32)},
	{IP: net.ParseIP("192.168.0.0"), Mask: net.CIDRMask(16, 32)},
	{IP: net.ParseIP("127.0.0.0"), Mask: net.CIDRMask(8, 32)},
	{IP: net.ParseIP("169.254.0.0"), Mask: net.CIDRMask(16, 32)}, // link-local IPv4 / cloud metadata
	{IP: net.ParseIP("::1"), Mask: net.CIDRMask(128, 128)},        // IPv6 loopback
	{IP: net.ParseIP("fe80::"), Mask: net.CIDRMask(10, 128)},      // IPv6 link-local
	{IP: net.ParseIP("fc00::"), Mask: net.CIDRMask(7, 128)},       // IPv6 unique local
}

// IsPrivateIP reports whether ip falls within a private, loopback, or link-local range.
// A nil IP is treated as private to fail safe.
func IsPrivateIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	for _, r := range privateRanges {
		if r.Contains(ip) {
			return true
		}
	}
	return false
}
