package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

const AppName = "PrinterOne-XP"

// Config mirrors the modern configuration while remaining Go 1.10 compatible.
type Config struct {
	PrinterName        string `json:"printer_name"`
	Port               int    `json:"port"`
	ListenAddress      string `json:"listen_address"`
	LANEnabled         bool   `json:"lan_enabled"`
	SharedToken        string `json:"shared_token"`
	Language           string `json:"language"`
	MinimizeToTray     bool   `json:"minimize_to_tray"`
	AutoStart          bool   `json:"auto_start"`
	StartWithWindows   bool   `json:"start_with_windows"`
	MaxJobBytes        int64  `json:"max_job_bytes"`
	MaxConnections     int    `json:"max_connections"`
	MaxQueuedJobs      int    `json:"max_queued_jobs"`
	ReadTimeoutSeconds int    `json:"read_timeout_seconds"`
	LegacyTCPEnabled   bool   `json:"legacy_tcp_enabled"`
}

func Default() Config {
	return Config{
		Port: 9100, ListenAddress: "0.0.0.0", LANEnabled: true,
		Language: "ru", MinimizeToTray: true, MaxJobBytes: 32 << 20,
		MaxConnections: 10, MaxQueuedJobs: 20, ReadTimeoutSeconds: 30,
	}
}

func (c Config) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}
	if c.ListenAddress == "" {
		return errors.New("listen address must not be empty")
	}
	if c.MaxJobBytes < 1 || c.MaxJobBytes > 1<<30 {
		return errors.New("max job bytes must be between 1 and 1073741824")
	}
	if c.MaxConnections < 1 || c.MaxConnections > 1000 {
		return errors.New("max connections must be between 1 and 1000")
	}
	if c.MaxQueuedJobs < 1 || c.MaxQueuedJobs > 10000 {
		return errors.New("max queued jobs must be between 1 and 10000")
	}
	if c.ReadTimeoutSeconds < 1 || c.ReadTimeoutSeconds > 3600 {
		return errors.New("read timeout must be between 1 and 3600 seconds")
	}
	return nil
}

func Directory() (string, error) {
	dir := os.Getenv("APPDATA")
	if dir == "" {
		return "", errors.New("APPDATA is not set")
	}
	return filepath.Join(dir, AppName), nil
}

func Path() (string, error) {
	dir, err := Directory()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func Load() (Config, error) {
	path, err := Path()
	if err != nil {
		return Config{}, err
	}
	data, err := ioutil.ReadFile(path)
	if os.IsNotExist(err) {
		return Default(), nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read config: %v", err)
	}
	c := Default()
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("parse config: %v", err)
	}
	return c, c.Validate()
}

func Save(c Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	path, err := Path()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config directory: %v", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %v", err)
	}
	tmp, err := ioutil.TempFile(dir, "config-")
	if err != nil {
		return fmt.Errorf("create config temp file: %v", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return fmt.Errorf("write config: %v", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close config: %v", err)
	}
	_ = os.Remove(path)
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace config: %v", err)
	}
	return nil
}
