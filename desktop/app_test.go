package main

import (
	"io"
	"net"
	"strconv"
	"testing"
	"time"
)

func localTCPServer(t *testing.T) (string, int, <-chan []byte) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	host, portText, _ := net.SplitHostPort(listener.Addr().String())
	port, _ := strconv.Atoi(portText)
	received := make(chan []byte, 1)
	go func() {
		defer listener.Close()
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			received <- nil
			return
		}
		defer conn.Close()
		data, _ := io.ReadAll(conn)
		received <- data
	}()
	return host, port, received
}

func TestConnectionDoesNotSendPrintData(t *testing.T) {
	host, port, received := localTCPServer(t)
	app := NewApp()
	if _, err := app.TestConnection(host, port); err != nil {
		t.Fatal(err)
	}
	select {
	case data := <-received:
		if len(data) != 0 {
			t.Fatalf("connection test sent %d bytes", len(data))
		}
	case <-time.After(time.Second):
		t.Fatal("test server did not observe the connection")
	}
}

func TestSendTestDataWritesPayload(t *testing.T) {
	host, port, received := localTCPServer(t)
	app := NewApp()
	written, err := app.SendTestData(host, port)
	if err != nil {
		t.Fatal(err)
	}
	data := <-received
	if written == 0 || len(data) != written {
		t.Fatalf("written=%d received=%d", written, len(data))
	}
}
