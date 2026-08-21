package receiver

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/ashtray01/printerone/legacy/xp/config"
)

func TestServerReceivesCompleteJob(t *testing.T) {
	c := config.Default()
	c.ListenAddress = "127.0.0.1"
	c.Port = freePort(t)
	printed := make(chan []byte, 1)
	s := New(c, func(data []byte) error {
		printed <- append([]byte(nil), data...)
		return nil
	}, nil)
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	conn, err := net.Dial("tcp", net.JoinHostPort(c.ListenAddress, itoa(c.Port)))
	if err != nil {
		t.Fatal(err)
	}
	want := []byte("PrinterOne XP test\r\n\f")
	_, _ = conn.Write(want[:8])
	_, _ = conn.Write(want[8:])
	_ = conn.Close()
	select {
	case got := <-printed:
		if !bytes.Equal(got, want) {
			t.Fatalf("got %q, want %q", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("print callback was not called")
	}
}

func TestReadJobRejectsOversize(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	go func() {
		_, _ = client.Write([]byte("12345"))
		_ = client.Close()
	}()
	_, _, err := ReadJob(server, 4, time.Second)
	if err != ErrJobTooLarge {
		t.Fatalf("got %v, want ErrJobTooLarge", err)
	}
}

func TestStopDiscardsPartialConnections(t *testing.T) {
	c := config.Default()
	c.ListenAddress = "127.0.0.1"
	c.Port = freePort(t)
	printed := make(chan []byte, 1)
	s := New(c, func(data []byte) error {
		printed <- append([]byte(nil), data...)
		return nil
	}, nil)
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	conn, err := net.Dial("tcp", net.JoinHostPort(c.ListenAddress, itoa(c.Port)))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_, _ = conn.Write([]byte("incomplete job"))
	deadline := time.Now().Add(time.Second)
	for {
		s.mu.Lock()
		active := s.active
		s.mu.Unlock()
		if active > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("server did not register the connection")
		}
		time.Sleep(time.Millisecond)
	}
	s.Stop()
	select {
	case data := <-printed:
		t.Fatalf("partial job was printed during shutdown: %q", data)
	case <-time.After(50 * time.Millisecond):
	}
}

func freePort(t *testing.T) int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func itoa(value int) string {
	const digits = "0123456789"
	if value == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for value > 0 {
		i--
		buf[i] = digits[value%10]
		value /= 10
	}
	return string(buf[i:])
}
