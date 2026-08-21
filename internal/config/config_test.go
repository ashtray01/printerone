package config

import "testing"

func TestDefaultConfigEnablesNetworkPrinting(t *testing.T) {
	c := Default()
	if c.ListenAddress != "0.0.0.0" || !c.LANEnabled {
		t.Fatalf("default config must listen on the local network: %#v", c)
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("default config must be valid: %v", err)
	}
}

func TestLANModeSupportsStandardRawPrintClients(t *testing.T) {
	c := Default()
	if err := c.Validate(); err != nil {
		t.Fatalf("LAN mode must not require a proprietary token: %v", err)
	}
}
