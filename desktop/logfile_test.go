package main

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestSessionLogWritesMetadata(t *testing.T) {
	log, err := openSessionLogIn(t.TempDir(), time.Date(2026, 8, 21, 10, 20, 30, 0, time.Local))
	if err != nil {
		t.Fatal(err)
	}
	if err := log.write("[SPOOL] Job 42 accepted"); err != nil {
		t.Fatal(err)
	}
	if err := log.file.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(log.path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "[SPOOL] Job 42 accepted") {
		t.Fatalf("log=%q", data)
	}
}
