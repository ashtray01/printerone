package sessionlog

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ashtray01/printerone/legacy/xp/config"
)

type Log struct {
	mu   sync.Mutex
	file *os.File
	Path string
}

func Open() (*Log, error) {
	dir, err := config.Directory()
	if err != nil {
		return nil, err
	}
	dir = filepath.Join(dir, "logs")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "printerone-xp-"+time.Now().Format("20060102-150405")+".log")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	return &Log{file: file, Path: path}, nil
}

func (l *Log) Write(message string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		_, _ = fmt.Fprintf(l.file, "%s %s\r\n", time.Now().Format("2006-01-02 15:04:05.000"), message)
	}
}

func (l *Log) Close() {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		_ = l.file.Close()
		l.file = nil
	}
}
