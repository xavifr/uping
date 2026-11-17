package main

import (
	"net"
	"testing"
)

// Regression: getIPFromInterface returned addrs[0] unconditionally, so an
// interface that listed its IPv6 address first produced an IPv6 source for an
// IPv4 target. IPv6 entries must now be skipped.
func TestPickIPv4SkipsIPv6(t *testing.T) {
	tests := []struct {
		name  string
		addrs []net.Addr
		want  string
	}{
		{
			name:  "ipv6 first",
			addrs: []net.Addr{ipNet("fe80::1", 64), ipNet("192.168.1.10", 24)},
			want:  "192.168.1.10",
		},
		{
			name:  "ipv4 first",
			addrs: []net.Addr{ipNet("192.168.1.10", 24), ipNet("fe80::1", 64)},
			want:  "192.168.1.10",
		},
		{
			name:  "only ipv6",
			addrs: []net.Addr{ipNet("fe80::1", 64), ipNet("::1", 128)},
			want:  "",
		},
		{
			name:  "ipaddr type",
			addrs: []net.Addr{ipAddr("::1"), ipAddr("10.0.0.5")},
			want:  "10.0.0.5",
		},
		{
			name:  "ipv4 mapped ipv6 is normalised",
			addrs: []net.Addr{ipNet("::ffff:192.168.1.10", 120)},
			want:  "192.168.1.10",
		},
		{
			name:  "no addresses",
			addrs: nil,
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pickIPv4(tt.addrs)

			if tt.want == "" {
				if got != nil {
					t.Fatalf("got %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Fatalf("got nil, want %s", tt.want)
			}

			if got.String() != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

// The real helper must still resolve a local interface, and what it returns
// must be IPv4 when it returns anything at all.
func TestGetIPFromInterfaceReturnsIPv4(t *testing.T) {
	got := getIPFromInterface("lo")
	if got == "" {
		t.Skip("loopback has no IPv4 address")
	}

	ip := net.ParseIP(got)
	if ip == nil || ip.To4() == nil {
		t.Errorf("getIPFromInterface(lo) = %q, want an IPv4 address", got)
	}
}

func TestGetIPFromInterfaceUnknownInterface(t *testing.T) {
	if got := getIPFromInterface("this-interface-does-not-exist"); got != "" {
		t.Errorf("getIPFromInterface(unknown) = %q, want \"\"", got)
	}
}

func ipNet(ip string, bits int) *net.IPNet {
	return &net.IPNet{IP: net.ParseIP(ip), Mask: net.CIDRMask(bits, 128)}
}

func ipAddr(ip string) *net.IPAddr {
	return &net.IPAddr{IP: net.ParseIP(ip)}
}
