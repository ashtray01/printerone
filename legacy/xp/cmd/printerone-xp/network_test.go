package main

import (
	"net"
	"testing"
)

func TestDisplayListenAddressKeepsExplicitAddress(t *testing.T) {
	if got := displayListenAddress("192.168.1.25"); got != "192.168.1.25" {
		t.Fatalf("got %q", got)
	}
}

func TestBestIPv4PrefersPrivateLANAddress(t *testing.T) {
	got := bestIPv4([]net.IP{
		net.ParseIP("127.0.0.1"),
		net.ParseIP("169.254.10.20"),
		net.ParseIP("203.0.113.8"),
		net.ParseIP("192.168.50.12"),
	})
	if got == nil || got.String() != "192.168.50.12" {
		t.Fatalf("got %v", got)
	}
}

func TestBestIPv4FallsBackToLinkLocalAddress(t *testing.T) {
	got := bestIPv4([]net.IP{net.ParseIP("169.254.10.20")})
	if got == nil || got.String() != "169.254.10.20" {
		t.Fatalf("got %v", got)
	}
}

func TestBestIPv4RejectsUnusableAddresses(t *testing.T) {
	got := bestIPv4([]net.IP{
		net.ParseIP("0.0.0.0"),
		net.ParseIP("127.0.0.1"),
		net.ParseIP("224.0.0.1"),
		net.ParseIP("::1"),
	})
	if got != nil {
		t.Fatalf("got %v", got)
	}
}
