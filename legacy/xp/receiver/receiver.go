package receiver

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/ashtray01/printerone/legacy/xp/config"
)

type PrintFunc func([]byte) error
type LogFunc func(string)

var ErrJobTooLarge = errors.New("print job exceeds configured size limit")

type Server struct {
	mu       sync.Mutex
	config   config.Config
	listener net.Listener
	stop     chan struct{}
	print    PrintFunc
	log      LogFunc
	active   int
	jobs     int
	workers  sync.WaitGroup
	clients  map[net.Conn]struct{}
}

func New(c config.Config, print PrintFunc, log LogFunc) *Server {
	return &Server{config: c, print: print, log: log, clients: make(map[net.Conn]struct{})}
}

func (s *Server) writeLog(format string, args ...interface{}) {
	if s.log != nil {
		s.log(fmt.Sprintf(format, args...))
	}
}

func (s *Server) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listener != nil
}

func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		return nil
	}
	if err := s.config.Validate(); err != nil {
		return err
	}
	listener, err := net.Listen("tcp", net.JoinHostPort(s.config.ListenAddress, fmt.Sprint(s.config.Port)))
	if err != nil {
		return err
	}
	s.listener = listener
	s.stop = make(chan struct{})
	s.writeLog("[INFO] Server listening on %s", listener.Addr())
	s.workers.Add(1)
	go s.accept(listener, s.stop)
	return nil
}

func (s *Server) Stop() {
	s.mu.Lock()
	if s.listener != nil {
		s.writeLog("[INFO] Server stopped")
		close(s.stop)
		_ = s.listener.Close()
		s.listener = nil
	}
	for client := range s.clients {
		_ = client.Close()
	}
	s.mu.Unlock()
	// Shutdown cleanup must run after the last accepted connection has lost
	// the ability to create a Windows spool job.
	s.workers.Wait()
}

func (s *Server) Apply(c config.Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	running := s.listener != nil
	restart := s.config.Port != c.Port || s.config.ListenAddress != c.ListenAddress
	s.config = c
	s.mu.Unlock()
	if running && restart {
		s.Stop()
		return s.Start()
	}
	return nil
}

func (s *Server) accept(listener net.Listener, stop <-chan struct{}) {
	defer s.workers.Done()
	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-stop:
				return
			default:
				s.writeLog("[ERROR] Accept connection: %v", err)
				continue
			}
		}
		s.mu.Lock()
		c := s.config
		if s.active >= c.MaxConnections {
			s.mu.Unlock()
			s.writeLog("[REJECT] Connection limit reached: %s", conn.RemoteAddr())
			_ = conn.Close()
			continue
		}
		s.active++
		s.clients[conn] = struct{}{}
		s.workers.Add(1)
		s.mu.Unlock()
		go s.serveConnection(conn, c)
	}
}

func (s *Server) serveConnection(conn net.Conn, c config.Config) {
	defer func() {
		s.mu.Lock()
		s.active--
		delete(s.clients, conn)
		s.mu.Unlock()
		s.workers.Done()
	}()
	s.handle(conn, c)
}

func (s *Server) handle(conn net.Conn, c config.Config) {
	defer conn.Close()
	remote := conn.RemoteAddr().String()
	s.writeLog("[CONNECT] Client connected: %s", remote)
	defer s.writeLog("[DISCONNECT] Client disconnected: %s", remote)
	data, idleTimeout, err := ReadJob(conn, c.MaxJobBytes, time.Duration(c.ReadTimeoutSeconds)*time.Second)
	if err != nil {
		if err == ErrJobTooLarge {
			s.writeLog("[REJECT] Job from %s exceeds the %d byte limit", remote, c.MaxJobBytes)
			return
		}
		s.writeLog("[ERROR] Read from %s: %v", remote, err)
		return
	}
	if len(data) == 0 {
		s.writeLog("[INFO] Connection test completed: %s", remote)
		return
	}
	if idleTimeout {
		s.writeLog("[INFO] Idle read timeout from %s after %d bytes; processing buffered job", remote, len(data))
	}
	s.writeLog("[RECEIVE] Print job from %s: %d bytes", remote, len(data))
	s.mu.Lock()
	if s.jobs >= c.MaxQueuedJobs {
		s.mu.Unlock()
		s.writeLog("[REJECT] Print queue limit reached for %s", remote)
		return
	}
	s.jobs++
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.jobs--
		s.mu.Unlock()
	}()
	if s.print != nil {
		if err := s.print(data); err != nil {
			s.writeLog("[ERROR] Print job from %s failed: %v", remote, err)
			return
		}
		s.writeLog("[PRINT] Job from %s accepted by Windows spooler", remote)
	}
}

func ReadJob(conn net.Conn, maxBytes int64, idleTimeout time.Duration) ([]byte, bool, error) {
	var data bytes.Buffer
	chunk := make([]byte, 32*1024)
	for {
		if err := conn.SetReadDeadline(time.Now().Add(idleTimeout)); err != nil {
			return nil, false, err
		}
		n, err := conn.Read(chunk)
		if n > 0 {
			if int64(data.Len()+n) > maxBytes {
				return nil, false, ErrJobTooLarge
			}
			_, _ = data.Write(chunk[:n])
		}
		if err == nil {
			continue
		}
		if err == io.EOF {
			return data.Bytes(), false, nil
		}
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() && data.Len() > 0 {
			return data.Bytes(), true, nil
		}
		return nil, false, err
	}
}
