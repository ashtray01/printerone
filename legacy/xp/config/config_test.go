package config

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoad(t *testing.T) {
	dir, err := ioutil.TempDir("", "printerone-xp-config-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	old := os.Getenv("APPDATA")
	defer os.Setenv("APPDATA", old)
	os.Setenv("APPDATA", dir)
	c := Default()
	c.PrinterName = "Generic / Text Only"
	c.Port = 19100
	if err := Save(c); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.PrinterName != c.PrinterName || loaded.Port != c.Port {
		t.Fatalf("unexpected config: %#v", loaded)
	}
	want := filepath.Join(dir, AppName, "config.json")
	if path, _ := Path(); path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
}
