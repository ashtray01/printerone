package receiver

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/ashtray01/printerone/internal/config"
)

type PrintFunc func([]byte) error
type LogFunc func(string)

var errJobTooLarge = errors.New("print job exceeds configured size limit")

type Server struct {
	mu       sync.Mutex
	config   config.Config
	listener net.Listener
	stop     chan struct{}
	print    PrintFunc
	log      LogFunc
	active   int
	jobs     int
}

func New(c config.Config, print PrintFunc, logger ...LogFunc) *Server {
	s := &Server{config: c, print: print}
	if len(logger) > 0 {
		s.log = logger[0]
	}
	return s
}

func (s *Server) writeLog(message string) {
	if s.log != nil {
		s.log(message)
	}
}

func (s *Server) Running() bool { s.mu.Lock(); defer s.mu.Unlock(); return s.listener != nil }

func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		return nil
	}
	if err := s.config.Validate(); err != nil {
		return err
	}
	address := s.config.ListenAddress
	listener, err := net.Listen("tcp", net.JoinHostPort(address, fmt.Sprint(s.config.Port)))
	if err != nil {
		return err
	}
	s.listener, s.stop = listener, make(chan struct{})
	s.writeLog(fmt.Sprintf("[INFO] Server listening on %s", listener.Addr()))
	go s.accept(listener, s.stop)
	return nil
}

func (s *Server) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		s.writeLog("[INFO] Server stopped")
		s.listener.Close()
		close(s.stop)
		s.listener = nil
	}
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
	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-stop:
				return
			default:
				s.writeLog(fmt.Sprintf("[ERROR] Accept connection: %v", err))
				continue
			}
		}
		s.mu.Lock()
		c := s.config
		if s.active >= c.MaxConnections {
			s.mu.Unlock()
			s.writeLog(fmt.Sprintf("[REJECT] Connection limit reached: %s", conn.RemoteAddr()))
			_ = conn.Close()
			continue
		}
		s.active++
		s.mu.Unlock()
		go func() {
			defer func() {
				s.mu.Lock()
				s.active--
				s.mu.Unlock()
			}()
			s.handle(conn, c)
		}()
	}
}

func (s *Server) handle(conn net.Conn, c config.Config) {
	defer conn.Close()
	remote := conn.RemoteAddr().String()
	s.writeLog(fmt.Sprintf("[CONNECT] Client connected: %s", remote))
	defer s.writeLog(fmt.Sprintf("[DISCONNECT] Client disconnected: %s", remote))
	data, idleTimeout, err := readJob(conn, c.MaxJobBytes, time.Duration(c.ReadTimeoutSeconds)*time.Second)
	if err != nil {
		if errors.Is(err, errJobTooLarge) {
			s.writeLog(fmt.Sprintf("[REJECT] Job from %s exceeds the %d byte limit", remote, c.MaxJobBytes))
			return
		}
		s.writeLog(fmt.Sprintf("[ERROR] Read from %s: %v", remote, err))
		return
	}
	if len(data) == 0 {
		s.writeLog(fmt.Sprintf("[INFO] Connection test completed: %s", remote))
		return
	}
	if idleTimeout {
		s.writeLog(fmt.Sprintf("[INFO] Idle read timeout from %s after %d bytes; processing buffered job", remote, len(data)))
	}
	s.writeLog(fmt.Sprintf("[RECEIVE] Print job from %s: %d bytes", remote, len(data)))
	s.mu.Lock()
	if s.jobs >= c.MaxQueuedJobs {
		s.mu.Unlock()
		s.writeLog(fmt.Sprintf("[REJECT] Print queue limit reached for %s", remote))
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
			s.writeLog(fmt.Sprintf("[ERROR] Print job from %s failed: %v", remote, err))
			return
		}
		s.writeLog(fmt.Sprintf("[PRINT] Job from %s accepted by Windows spooler", remote))
	}
}

// readJob treats an idle timeout after at least one byte as an end-of-job
// marker. The deadline is refreshed after every successful read, so active
// large transfers are not truncated by an absolute connection timeout.
func readJob(conn net.Conn, maxBytes int64, idleTimeout time.Duration) ([]byte, bool, error) {
	var data bytes.Buffer
	chunk := make([]byte, 32*1024)
	for {
		if err := conn.SetReadDeadline(time.Now().Add(idleTimeout)); err != nil {
			return nil, false, err
		}
		n, err := conn.Read(chunk)
		if n > 0 {
			if int64(data.Len()+n) > maxBytes {
				return nil, false, errJobTooLarge
			}
			_, _ = data.Write(chunk[:n])
		}
		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) {
			return data.Bytes(), false, nil
		}
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() && data.Len() > 0 {
			return data.Bytes(), true, nil
		}
		return nil, false, err
	}
}
