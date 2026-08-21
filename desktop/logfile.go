package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ashtray01/printerone/internal/config"
)

type sessionLog struct {
	file *os.File
	path string
}

func openSessionLog() (*sessionLog, error) {
	configPath, err := config.Path()
	if err != nil {
		return nil, err
	}
	return openSessionLogIn(filepath.Join(filepath.Dir(configPath), "logs"), time.Now())
}

func openSessionLogIn(dir string, now time.Time) (*sessionLog, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}
	path := filepath.Join(dir, "printerone-"+now.Format("20060102-150405")+".log")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}
	return &sessionLog{file: file, path: path}, nil
}

func (l *sessionLog) write(message string) error {
	_, err := fmt.Fprintf(l.file, "%s %s\r\n", time.Now().Format("2006-01-02 15:04:05.000"), message)
	return err
}

func (a *App) closeSessionLog() {
	a.logsMu.Lock()
	defer a.logsMu.Unlock()
	if a.fileLog != nil {
		_ = a.fileLog.file.Close()
		a.fileLog = nil
	}
}
