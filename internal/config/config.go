package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const AppName = "PrinterOne"

// Config contains only user-controlled application settings. Secrets are
// deliberately kept out of log output and never stored beside the executable.
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
		Port:               9100,
		ListenAddress:      "0.0.0.0",
		LANEnabled:         true,
		Language:           "ru",
		MinimizeToTray:     true,
		MaxJobBytes:        32 << 20,
		MaxConnections:     10,
		MaxQueuedJobs:      20,
		ReadTimeoutSeconds: 30,
	}
}

func (c Config) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
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

func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("get user config directory: %w", err)
	}
	return filepath.Join(dir, AppName, "config.json"), nil
}

func Load() (Config, error) {
	path, err := Path()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	c := Default()
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	// Migrate the former loopback-only default. PrinterOne is a network print
	// server, so LAN listening is now the default; users can still disable it.
	if !c.LANEnabled && c.ListenAddress == "127.0.0.1" && c.SharedToken == "" {
		c.LANEnabled = true
		c.ListenAddress = "0.0.0.0"
		_ = Save(c)
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
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "config-*.tmp")
	if err != nil {
		return fmt.Errorf("create config temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return fmt.Errorf("write config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close config: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}
