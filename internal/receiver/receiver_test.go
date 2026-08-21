package receiver

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/ashtray01/printerone/internal/config"
)

func freeTCPPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func TestServerLogsConnectionJobAndPrint(t *testing.T) {
	c := config.Default()
	c.ListenAddress = "127.0.0.1"
	c.Port = freeTCPPort(t)
	logLines := make(chan string, 20)
	printed := make(chan []byte, 1)
	server := New(c, func(data []byte) error {
		printed <- append([]byte(nil), data...)
		return nil
	}, func(line string) { logLines <- line })
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	defer server.Stop()

	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", c.Port))
	if err != nil {
		t.Fatal(err)
	}
	_, _ = conn.Write([]byte("test print job"))
	_ = conn.Close()

	select {
	case <-printed:
	case <-time.After(2 * time.Second):
		t.Fatal("print callback was not called")
	}

	wanted := map[string]bool{"[CONNECT]": false, "[RECEIVE]": false, "[PRINT]": false, "[DISCONNECT]": false}
	deadline := time.After(2 * time.Second)
	for {
		complete := true
		for _, found := range wanted {
			complete = complete && found
		}
		if complete {
			return
		}
		select {
		case line := <-logLines:
			for marker := range wanted {
				if strings.Contains(line, marker) {
					wanted[marker] = true
				}
			}
		case <-deadline:
			t.Fatalf("missing log events: %#v", wanted)
		}
	}
}

func TestReadJobProcessesBufferedDataAfterIdleTimeout(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	go func() { _, _ = client.Write([]byte("complete job without EOF")) }()
	data, timedOut, err := readJob(server, 1024, 40*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if !timedOut {
		t.Fatal("expected idle timeout to terminate buffered job")
	}
	if got := string(data); got != "complete job without EOF" {
		t.Fatalf("data=%q", got)
	}
}

func TestReadJobRejectsOversizedJob(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	go func() {
		_, _ = client.Write([]byte("12345"))
		_ = client.Close()
	}()
	_, _, err := readJob(server, 4, time.Second)
	if !errors.Is(err, errJobTooLarge) {
		t.Fatalf("err=%v", err)
	}
}
