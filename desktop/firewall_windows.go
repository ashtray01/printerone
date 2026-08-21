//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"syscall"
)

func validateFirewallPort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	return nil
}

func firewallRuleName(port int) string { return fmt.Sprintf("PrinterOne-TCP-%d", port) }

// FirewallStatus reports whether PrinterOne's inbound rule exists for port.
func (a *App) FirewallStatus(port int) (bool, error) {
	if err := validateFirewallPort(port); err != nil {
		return false, err
	}
	script := fmt.Sprintf("if (Get-NetFirewallRule -DisplayName '%s' -ErrorAction SilentlyContinue) { exit 0 } else { exit 1 }", firewallRuleName(port))
	command := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	err := command.Run()
	if err == nil {
		return true, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("check Windows Firewall rule: %w", err)
}

func runElevatedNetsh(arguments ...string) error {
	quoted := make([]string, len(arguments))
	for i, argument := range arguments {
		quoted[i] = fmt.Sprintf("'%s'", argument)
	}
	script := fmt.Sprintf("$p=Start-Process -FilePath \"$env:SystemRoot\\System32\\netsh.exe\" -ArgumentList @(%s) -Verb RunAs -WindowStyle Hidden -Wait -PassThru; exit $p.ExitCode", joinComma(quoted))
	command := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-ExecutionPolicy", "Bypass", "-Command", script)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("Windows Firewall command failed: %w (%s)", err, output)
	}
	return nil
}

func joinComma(values []string) string {
	result := ""
	for i, value := range values {
		if i > 0 {
			result += ","
		}
		result += value
	}
	return result
}

// OpenFirewallPort creates an inbound TCP allow rule for private networks.
// Windows displays a UAC confirmation before applying the change.
func (a *App) OpenFirewallPort(port int) error {
	if err := validateFirewallPort(port); err != nil {
		return err
	}
	if open, err := a.FirewallStatus(port); err == nil && open {
		return nil
	}
	return runElevatedNetsh("advfirewall", "firewall", "add", "rule", "name="+firewallRuleName(port), "dir=in", "action=allow", "protocol=TCP", fmt.Sprintf("localport=%d", port), "profile=private")
}

// CloseFirewallPort removes only the rule created by PrinterOne.
func (a *App) CloseFirewallPort(port int) error {
	if err := validateFirewallPort(port); err != nil {
		return err
	}
	return runElevatedNetsh("advfirewall", "firewall", "delete", "rule", "name="+firewallRuleName(port))
}
