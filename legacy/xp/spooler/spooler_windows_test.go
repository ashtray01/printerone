//go:build windows
// +build windows

package spooler

import "testing"

func TestDescribeStatus(t *testing.T) {
	got := describeStatus(jobStatusSpooling | jobStatusPrinting)
	if got != "spooling, printing" {
		t.Fatalf("unexpected status: %q", got)
	}
}

func TestUnsupportedVirtualPrinter(t *testing.T) {
	if !unsupportedVirtualPrinter("Microsoft XPS Document Writer") {
		t.Fatal("XPS writer must be rejected")
	}
	if unsupportedVirtualPrinter("Generic / Text Only") {
		t.Fatal("physical/raw printer was rejected")
	}
}
