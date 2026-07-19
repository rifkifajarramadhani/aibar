// Package control is the inbound control plane: a private Unix socket that
// carries refresh and navigation commands from the aibar CLI to a running
// daemon. It also enforces single-instance startup. It implements the
// daemon.ControlServer port.
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

	"github.com/overhaul/aibar/internal/daemon"
)

func SocketPath(cacheDir string) string { return filepath.Join(cacheDir, "aibar.sock") }
func PIDPath(cacheDir string) string    { return filepath.Join(cacheDir, "aibar.pid") }

// Server accepts control commands on a Unix socket and forwards them to the
// daemon through the Actions channel.
type Server struct {
	listener net.Listener
	cacheDir string
	Actions  chan<- string
}

var _ daemon.ControlServer = (*Server)(nil)

// Listen binds the control socket, refusing to start when a live daemon already
// owns it, and records the PID file. Close releases both.
func Listen(cacheDir string, actions chan<- string) (*Server, error) {
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return nil, err
	}

	path := SocketPath(cacheDir)
	if _, err := os.Stat(path); err == nil {
		if conn, dialErr := net.Dial("unix", path); dialErr == nil {
			_ = conn.Close()
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
		_ = listener.Close()
		return nil, err
	}

	if err := writePID(PIDPath(cacheDir)); err != nil {
		_ = listener.Close()
		return nil, err
	}

	return &Server{listener: listener, cacheDir: cacheDir, Actions: actions}, nil
}

// Close stops accepting connections and removes the socket and PID file.
func (s *Server) Close() error {
	err := s.listener.Close()
	removeRuntimeFiles(s.cacheDir)

	return err
}

func (s *Server) Run(ctx context.Context) error {
	defer func() { _ = s.listener.Close() }()

	go func() {
		<-ctx.Done()

		_ = s.listener.Close()
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
	defer func() { _ = conn.Close() }()

	command, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil && !errors.Is(err, os.ErrClosed) {
		return
	}

	command = strings.TrimSpace(command)
	if command == "" {
		return
	}

	if daemon.ValidCommand(command) {
		s.Actions <- command

		_, _ = fmt.Fprintln(conn, "ok")

		return
	}

	_, _ = fmt.Fprintln(conn, "unknown command")
}

// Send delivers a single command to a running daemon and waits for its ack.
func Send(cacheDir, command string) error {
	conn, err := net.Dial("unix", SocketPath(cacheDir))
	if err != nil {
		return fmt.Errorf("connect to aibar daemon: %w", err)
	}

	defer func() { _ = conn.Close() }()

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

func writePID(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	return os.WriteFile(path, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0o600)
}

func removeRuntimeFiles(cacheDir string) {
	_ = os.Remove(SocketPath(cacheDir))
	_ = os.Remove(PIDPath(cacheDir))
}
