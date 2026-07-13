package control

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
)

const (
	Refresh      = "refresh"
	NextProvider = "next-provider"
	PrevProvider = "prev-provider"
	CycleWindow  = "cycle-window"
)

func SocketPath(cacheDir string) string { return filepath.Join(cacheDir, "aibar.sock") }
func PIDPath(cacheDir string) string    { return filepath.Join(cacheDir, "aibar.pid") }

type Server struct {
	listener net.Listener
	Actions  chan<- string
}

func (s *Server) Close() error { return s.listener.Close() }

func Listen(cacheDir string, actions chan<- string) (*Server, error) {
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return nil, err
	}
	path := SocketPath(cacheDir)
	if _, err := os.Stat(path); err == nil {
		if conn, dialErr := net.Dial("unix", path); dialErr == nil {
			conn.Close()
			return nil, fmt.Errorf("aibar daemon already running")
		}
		if err := os.Remove(path); err != nil {
			return nil, err
		}
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		listener.Close()
		return nil, err
	}
	return &Server{listener: listener, Actions: actions}, nil
}

func (s *Server) Run(ctx context.Context) error {
	defer s.listener.Close()
	go func() {
		<-ctx.Done()
		s.listener.Close()
	}()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		go s.handle(conn)
	}
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	command, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil && !errors.Is(err, os.ErrClosed) {
		return
	}
	command = strings.TrimSpace(command)
	if command == "" {
		return
	}
	switch command {
	case Refresh, NextProvider, PrevProvider, CycleWindow:
		s.Actions <- command
		_, _ = fmt.Fprintln(conn, "ok")
	default:
		_, _ = fmt.Fprintln(conn, "unknown command")
	}
}

func Send(cacheDir, command string) error {
	conn, err := net.Dial("unix", SocketPath(cacheDir))
	if err != nil {
		return fmt.Errorf("connect to aibar daemon: %w", err)
	}
	defer conn.Close()
	if _, err := fmt.Fprintln(conn, command); err != nil {
		return err
	}
	response, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return err
	}
	if strings.TrimSpace(response) != "ok" {
		return fmt.Errorf("aibar daemon rejected command: %s", strings.TrimSpace(response))
	}
	return nil
}

func WritePID(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0o600)
}

func RemoveRuntimeFiles(cacheDir string) {
	_ = os.Remove(SocketPath(cacheDir))
	_ = os.Remove(PIDPath(cacheDir))
}
