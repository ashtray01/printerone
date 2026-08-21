package main

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/ashtray01/printerone/internal/config"
	"github.com/ashtray01/printerone/internal/printerwin"
	"github.com/ashtray01/printerone/internal/receiver"
)

// App struct
type App struct {
	ctx     context.Context
	config  config.Config
	server  *receiver.Server
	localIP string
	logsMu  sync.Mutex
	logs    []string
	lastLog string
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{config: config.Default()}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if loaded, err := config.Load(); err == nil {
		a.config = loaded
	}
	if enabled, err := systemStartupEnabled(); err == nil {
		a.config.StartWithWindows = enabled
	}
	a.config.LANEnabled = true
	if a.config.ListenAddress == "127.0.0.1" || a.config.ListenAddress == "" {
		a.config.ListenAddress = "0.0.0.0"
	}
	_ = config.Save(a.config)
	a.localIP = preferredLocalIP()
	a.server = receiver.New(a.config, func(data []byte) error { return printerwin.PrintRaw(a.config.PrinterName, data) }, a.addLog)
	a.addLog("[INFO] PrinterOne is ready")
	a.startTray()
	if a.config.AutoStart && a.config.PrinterName != "" {
		go func() { _ = a.server.Start() }()
	}
}

type AppState struct {
	Config  config.Config `json:"config"`
	Running bool          `json:"running"`
	LocalIP string        `json:"local_ip"`
	Logs    []string      `json:"logs"`
}

func (a *App) GetState() AppState {
	a.logsMu.Lock()
	logs := append([]string(nil), a.logs...)
	a.logsMu.Unlock()
	return AppState{Config: a.config, Running: a.server != nil && a.server.Running(), LocalIP: a.localIP, Logs: logs}
}

func (a *App) addLog(message string) {
	a.logsMu.Lock()
	defer a.logsMu.Unlock()
	if message == a.lastLog {
		return
	}
	a.lastLog = message
	a.logs = append(a.logs, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), message))
	if len(a.logs) > 500 {
		a.logs = append([]string(nil), a.logs[len(a.logs)-500:]...)
	}
}

func (a *App) ClearLogs() {
	a.logsMu.Lock()
	a.logs = nil
	a.lastLog = ""
	a.logsMu.Unlock()
}

func preferredLocalIP() string {
	if conn, err := net.Dial("udp", "8.8.8.8:80"); err == nil {
		defer conn.Close()
		if address, ok := conn.LocalAddr().(*net.UDPAddr); ok && address.IP.To4() != nil {
			return address.IP.String()
		}
	}
	interfaces, _ := net.Interfaces()
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, _ := iface.Addrs()
		for _, address := range addresses {
			ip, _, err := net.ParseCIDR(address.String())
			if err == nil && ip.To4() != nil && !ip.IsLoopback() {
				return ip.String()
			}
		}
	}
	return "127.0.0.1"
}

// SaveSettings persists the new limits immediately. The receiver will consume
// this same update through its Apply method without restarting the process.
func (a *App) SaveSettings(next config.Config) (AppState, error) {
	next.LANEnabled = true
	if next.ListenAddress == "127.0.0.1" || next.ListenAddress == "" {
		next.ListenAddress = "0.0.0.0"
	}
	if next.StartWithWindows != a.config.StartWithWindows {
		if err := configureSystemStartup(next.StartWithWindows); err != nil {
			return AppState{}, err
		}
	}
	if err := config.Save(next); err != nil {
		return AppState{}, err
	}
	a.config = next
	if a.server != nil {
		if err := a.server.Apply(next); err != nil {
			return AppState{}, err
		}
	}
	return a.GetState(), nil
}

func (a *App) ListPrinters() ([]string, error) { return printerwin.List() }

func (a *App) StartServer() (AppState, error) {
	if a.config.PrinterName == "" {
		return AppState{}, fmt.Errorf("select a printer first")
	}
	if err := a.server.Start(); err != nil {
		return AppState{}, err
	}
	return a.GetState(), nil
}

func (a *App) StopServer() AppState { a.server.Stop(); return a.GetState() }

// TestConnection verifies that a TCP endpoint accepts connections without
// sending a print job to it.
func (a *App) TestConnection(host string, port int) (string, error) {
	if host == "" {
		return "", fmt.Errorf("server address must not be empty")
	}
	if port < 1 || port > 65535 {
		return "", fmt.Errorf("port must be between 1 and 65535")
	}
	address := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", address, 3*time.Second)
	if err != nil {
		return "", fmt.Errorf("connect to %s: %w", address, err)
	}
	defer conn.Close()
	return address, nil
}

// SendTestData sends a small RAW text job, matching the test client from the
// original project. When pointed at PrinterOne, the selected printer receives it.
func (a *App) SendTestData(host string, port int) (int, error) {
	if host == "" {
		return 0, fmt.Errorf("server address must not be empty")
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("port must be between 1 and 65535")
	}
	address := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", address, 3*time.Second)
	if err != nil {
		return 0, fmt.Errorf("connect to %s: %w", address, err)
	}
	defer conn.Close()
	_ = conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	data := []byte("PrinterOne Test Data\r\n\r\nThe network print server is working correctly.\r\n\f")
	n, err := conn.Write(data)
	if err != nil {
		return n, fmt.Errorf("send test data: %w", err)
	}
	return n, nil
}

func (a *App) Version() string {
	return fmt.Sprintf("1.0.2 (Go)")
}
