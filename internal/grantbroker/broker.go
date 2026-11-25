// Package grantbroker exposes ProxiLens' non-privileged, per-user grant state
// over a private Unix-domain socket. It is a development broker only: it does
// not authorize operating-system actions and must never be trusted by sudo,
// PAM, Polkit, or a privileged process.
package grantbroker

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

const DefaultDuration = 15 * time.Minute

type Status struct {
	Active    bool      `json:"active"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	Scope     string    `json:"scope"`
}

type request struct {
	Op       string        `json:"op"`
	Duration time.Duration `json:"duration,omitempty"`
}
type response struct {
	OK     bool   `json:"ok"`
	Error  string `json:"error,omitempty"`
	Status Status `json:"status"`
}

type Broker struct {
	mu       sync.Mutex
	expires  time.Time
	listener net.Listener
	socket   string
	closed   chan struct{}
}

// RuntimeSocket returns the private per-user socket path.
func RuntimeSocket() (string, error) {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(base, "proxilens", "run")
	} else {
		dir = filepath.Join(dir, "proxilens")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "grant.sock"), nil
}

// StartServer starts the per-user status broker. The socket is private to the
// current user; it is not a privileged authorization endpoint.
func StartServer() (*Broker, error) {
	path, err := RuntimeSocket()
	if err != nil {
		return nil, fmt.Errorf("grant broker runtime directory: %w", err)
	}
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			return nil, fmt.Errorf("refusing to replace non-socket at %s", path)
		}
		// Do not unlink a live broker belonging to another ProxiLens instance.
		conn, dialErr := net.DialTimeout("unix", path, 300*time.Millisecond)
		if dialErr == nil {
			conn.Close()
			return nil, fmt.Errorf("grant broker already running at %s", path)
		}
		if err := os.Remove(path); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0600); err != nil {
		ln.Close()
		os.Remove(path)
		return nil, err
	}
	b := &Broker{listener: ln, socket: path, closed: make(chan struct{})}
	go b.serve()
	return b, nil
}

func (b *Broker) Start(duration time.Duration) time.Time {
	if duration <= 0 {
		duration = DefaultDuration
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.expires = time.Now().Add(duration)
	return b.expires
}

func (b *Broker) Revoke() {
	b.mu.Lock()
	b.expires = time.Time{}
	b.mu.Unlock()
}

func (b *Broker) Status() Status {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !time.Now().Before(b.expires) {
		b.expires = time.Time{}
		return Status{Scope: "local-test-only"}
	}
	return Status{Active: true, ExpiresAt: b.expires, Scope: "local-test-only"}
}

func (b *Broker) serve() {
	defer close(b.closed)
	for {
		conn, err := b.listener.Accept()
		if err != nil {
			return
		}
		go b.handle(conn)
	}
}

func (b *Broker) handle(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	var req request
	dec := json.NewDecoder(bufio.NewReader(conn))
	if err := dec.Decode(&req); err != nil {
		_ = json.NewEncoder(conn).Encode(response{Error: "invalid request"})
		return
	}
	switch req.Op {
	case "status":
		_ = json.NewEncoder(conn).Encode(response{OK: true, Status: b.Status()})
	case "start-test-grant":
		// Development-only operation. This socket must never be used as an
		// authorization endpoint by privileged processes.
		b.Start(req.Duration)
		_ = json.NewEncoder(conn).Encode(response{OK: true, Status: b.Status()})
	case "revoke-test-grant":
		b.Revoke()
		_ = json.NewEncoder(conn).Encode(response{OK: true, Status: b.Status()})
	default:
		_ = json.NewEncoder(conn).Encode(response{Error: "unsupported operation"})
	}
}

// Close shuts down the socket and removes it. It does not persist grant state.
func (b *Broker) Close() error {
	if b == nil || b.listener == nil {
		return nil
	}
	err := b.listener.Close()
	<-b.closed
	removeErr := os.Remove(b.socket)
	if errors.Is(removeErr, os.ErrNotExist) {
		removeErr = nil
	}
	if err != nil && !errors.Is(err, net.ErrClosed) {
		return err
	}
	return removeErr
}

// QueryStatus reads status from a running per-user broker.
func QueryStatus() (Status, error) {
	path, err := RuntimeSocket()
	if err != nil {
		return Status{}, err
	}
	conn, err := net.DialTimeout("unix", path, time.Second)
	if err != nil {
		return Status{}, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if err := json.NewEncoder(conn).Encode(request{Op: "status"}); err != nil {
		return Status{}, err
	}
	var resp response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return Status{}, err
	}
	if !resp.OK {
		return Status{}, errors.New(resp.Error)
	}
	return resp.Status, nil
}

// StartTestGrant requests a local development-only grant from the running broker.
func StartTestGrant(duration time.Duration) (Status, error) {
	if duration <= 0 {
		duration = DefaultDuration
	}
	return call(request{Op: "start-test-grant", Duration: duration})
}

// RevokeTestGrant revokes the local development-only grant.
func RevokeTestGrant() (Status, error) {
	return call(request{Op: "revoke-test-grant"})
}

func call(req request) (Status, error) {
	path, err := RuntimeSocket()
	if err != nil {
		return Status{}, err
	}
	conn, err := net.DialTimeout("unix", path, time.Second)
	if err != nil {
		return Status{}, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return Status{}, err
	}
	var resp response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return Status{}, err
	}
	if !resp.OK {
		return Status{}, errors.New(resp.Error)
	}
	return resp.Status, nil
}

// Serve runs the broker until its listener is closed by the caller.
func Serve() error {
	b, err := StartServer()
	if err != nil {
		return err
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	select {
	case <-b.closed:
		return nil
	case <-signals:
		return b.Close()
	}
}
