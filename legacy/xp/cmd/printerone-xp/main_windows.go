//go:build windows
// +build windows

package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/ashtray01/printerone/legacy/xp/config"
	"github.com/ashtray01/printerone/legacy/xp/platform"
	"github.com/ashtray01/printerone/legacy/xp/receiver"
	"github.com/ashtray01/printerone/legacy/xp/sessionlog"
	"github.com/ashtray01/printerone/legacy/xp/spooler"
)

type application struct {
	cfgMu     sync.Mutex
	cfg       config.Config
	server    *receiver.Server
	fileLog   *sessionlog.Log
	instance  *platform.Instance
	language  string
	hwnd      uintptr
	controls  controls
	pendingMu sync.Mutex
	pending   []string
	trayAdded bool
	exiting   bool
}

var app *application

func main() {
	instance, err := platform.AcquireInstance()
	if err != nil {
		messageBox(0, err.Error(), "PrinterOne XP")
		return
	}
	defer instance.Close()

	cfg, err := config.Load()
	if err != nil {
		cfg = config.Default()
	}
	// Keep the XP runtime isolated while preserving the modern application's
	// LAN-listening behaviour for configurations created from older defaults.
	cfg.LANEnabled = true
	cfg.SharedToken = ""
	if cfg.ListenAddress == "" || cfg.ListenAddress == "127.0.0.1" {
		cfg.ListenAddress = "0.0.0.0"
	}
	if startup, startupErr := platform.StartupEnabled(); startupErr == nil {
		cfg.StartWithWindows = startup
	}
	cfg.Language = normalizeLanguage(cfg.Language)
	app = &application{cfg: cfg, instance: instance, language: cfg.Language}
	if saveErr := config.Save(cfg); saveErr != nil && err == nil {
		err = saveErr
	}
	if log, logErr := sessionlog.Open(); logErr == nil {
		app.fileLog = log
		defer log.Close()
		app.addLog("[INFO] Log file: " + log.Path)
	}
	if err != nil {
		app.addLog("[WARN] Configuration reset to defaults: " + err.Error())
	}
	app.server = receiver.New(cfg, app.printJob, app.addLog)
	if clearErr := app.clearStaleJobs(); clearErr != nil {
		app.addLog("[WARN] Could not remove stale print jobs: " + clearErr.Error())
	}
	app.addLog("[INFO] PrinterOne XP is ready")
	if runErr := app.runWindow(); runErr != nil {
		messageBox(0, runErr.Error(), "PrinterOne XP")
	}
}

func (a *application) currentConfig() config.Config {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	return a.cfg
}

func (a *application) printJob(data []byte) error {
	cfg := a.currentConfig()
	_, err := spooler.PrintRaw(cfg.PrinterName, data, spooler.LogFunc(a.addLog))
	return err
}

func (a *application) addLog(message string) {
	if a.fileLog != nil {
		a.fileLog.Write(message)
	}
	line := fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), message)
	a.pendingMu.Lock()
	a.pending = append(a.pending, line)
	if len(a.pending) > 500 {
		a.pending = append([]string(nil), a.pending[len(a.pending)-500:]...)
	}
	a.pendingMu.Unlock()
	if a.hwnd != 0 {
		postMessage(a.hwnd, wmAppLog, 0, 0)
	}
}

func (a *application) drainLogs() []string {
	a.pendingMu.Lock()
	defer a.pendingMu.Unlock()
	lines := append([]string(nil), a.pending...)
	a.pending = nil
	return lines
}

func (a *application) saveFromWindow() error {
	next, err := a.readConfigFromControls()
	if err != nil {
		return err
	}
	old := a.currentConfig()
	// Language has its own small Apply button. General settings must never
	// silently commit a value that is merely highlighted in the language list.
	next.Language = old.Language
	if next.StartWithWindows != old.StartWithWindows {
		if err := platform.ConfigureStartup(next.StartWithWindows); err != nil {
			return fmt.Errorf("configure startup: %v", err)
		}
	}
	if err := config.Save(next); err != nil {
		return err
	}
	a.cfgMu.Lock()
	a.cfg = next
	a.cfgMu.Unlock()
	if err := a.server.Apply(next); err != nil {
		return err
	}
	a.addLog("[OK] Settings saved")
	a.updateStatus()
	return nil
}

func (a *application) applyLanguage() error {
	index := comboIndex(a.controls.language)
	if index < 0 || index >= len(languageOptions) {
		return fmt.Errorf("select a language")
	}
	next := a.currentConfig()
	next.Language = languageOptions[index].code
	if next.Language == a.language {
		return nil
	}
	if err := config.Save(next); err != nil {
		return err
	}
	a.cfgMu.Lock()
	a.cfg = next
	a.cfgMu.Unlock()
	a.addLog("[OK] Language saved: " + next.Language)
	return a.restart()
}

func (a *application) startServer() error {
	if err := a.saveFromWindow(); err != nil {
		return err
	}
	if a.currentConfig().PrinterName == "" {
		return fmt.Errorf("select a printer first")
	}
	if err := a.server.Start(); err != nil {
		return err
	}
	a.updateStatus()
	return nil
}

func (a *application) clearStaleJobs() error {
	cfg := a.currentConfig()
	return spooler.ClearPendingPrinterOneJobs(cfg.PrinterName, spooler.LogFunc(a.addLog))
}

func (a *application) restart() error {
	if a.server != nil {
		a.server.Stop()
	}
	if err := a.clearStaleJobs(); err != nil {
		a.addLog("[WARN] Could not remove stale print jobs: " + err.Error())
	}
	a.instance.Close()
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	process, err := os.StartProcess(executable, []string{executable}, &os.ProcAttr{Dir: filepath.Dir(executable), Files: []*os.File{nil, nil, nil}})
	if err != nil {
		instance, acquireErr := platform.AcquireInstance()
		if acquireErr == nil {
			a.instance = instance
		}
		return fmt.Errorf("restart PrinterOne XP: %v", err)
	}
	_ = process.Release()
	a.exiting = true
	procDestroyWindow.Call(a.hwnd)
	return nil
}

func (a *application) stopServer() { a.server.Stop(); a.updateStatus() }

func (a *application) testConnection(sendData bool) error {
	cfg := a.currentConfig()
	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(cfg.Port))
	conn, err := net.DialTimeout("tcp", address, 3*time.Second)
	if err != nil {
		return fmt.Errorf("connect to %s: %v", address, err)
	}
	defer conn.Close()
	if !sendData {
		a.addLog("[OK] Connection established: " + address)
		return nil
	}
	_ = conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	data := []byte("PrinterOne XP Test Data\r\n\r\nThe network print server is working correctly.\r\n\f")
	n, err := conn.Write(data)
	if err != nil {
		return fmt.Errorf("send test data: %v", err)
	}
	a.addLog(fmt.Sprintf("[OK] Test data sent: %d bytes", n))
	return nil
}

func fatalIf(err error) {
	if err == nil {
		return
	}
	if app != nil {
		app.addLog("[ERROR] " + err.Error())
		messageBox(app.hwnd, err.Error(), "PrinterOne XP")
	} else {
		_, _ = fmt.Fprintln(os.Stderr, err)
	}
}
