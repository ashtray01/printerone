//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	startupRegistryPath = `Software\Microsoft\Windows\CurrentVersion\Run`
	startupValueName    = "PrinterOne"
)

func startupCommand() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("get executable path: %w", err)
	}
	return `"` + executable + `"`, nil
}

func systemStartupEnabled() (bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, startupRegistryPath, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("open Windows startup registry: %w", err)
	}
	defer key.Close()
	value, _, err := key.GetStringValue(startupValueName)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read Windows startup registry: %w", err)
	}
	expected, err := startupCommand()
	return err == nil && strings.EqualFold(value, expected), err
}

func configureSystemStartup(enabled bool) error {
	if enabled {
		command, err := startupCommand()
		if err != nil {
			return err
		}
		key, _, err := registry.CreateKey(registry.CURRENT_USER, startupRegistryPath, registry.SET_VALUE)
		if err != nil {
			return fmt.Errorf("open Windows startup registry: %w", err)
		}
		defer key.Close()
		if err := key.SetStringValue(startupValueName, command); err != nil {
			return fmt.Errorf("enable Windows startup: %w", err)
		}
		return nil
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, startupRegistryPath, registry.SET_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open Windows startup registry: %w", err)
	}
	defer key.Close()
	if err := key.DeleteValue(startupValueName); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return fmt.Errorf("disable Windows startup: %w", err)
	}
	return nil
}
