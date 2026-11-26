// Package policyservice provides a hardened, observation-only privileged IPC probe.
// It never authorizes or performs privileged actions.
package policyservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const SocketPath = "/run/proxilens-policy/probe.sock"
const maxLine = 1024
const maxConcurrent = 8
const requestTimeout = 3 * time.Second

type request struct {
	Operation string `json:"operation"`
}
type response struct {
	OK           bool   `json:"ok"`
	Mode         string `json:"mode"`
	PID          int    `json:"pid"`
	UID          uint32 `json:"uid"`
	GID          uint32 `json:"gid"`
	SessionID    string `json:"session_id,omitempty"`
	SessionType  string `json:"session_type,omitempty"`
	SessionState string `json:"session_state,omitempty"`
	Authorized   bool   `json:"authorized"`
	Message      string `json:"message"`
}

// Serve starts a root-owned, read-only diagnostic socket. Access is limited to
// root and members of the local sudo group. It always reports Authorized=false.
func Serve() error {
	if os.Geteuid() != 0 {
		return errors.New("policy probe service must run as root")
	}
	grp, err := user.LookupGroup("sudo")
	if err != nil {
		return fmt.Errorf("required local group sudo not found: %w", err)
	}
	gid64, err := strconv.ParseUint(grp.Gid, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid sudo group GID: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(SocketPath), 0755); err != nil {
		return err
	}
	if err := os.Chown(filepath.Dir(SocketPath), 0, int(gid64)); err != nil {
		return err
	}
	if err := os.Chmod(filepath.Dir(SocketPath), 0750); err != nil {
		return err
	}
	if err := os.Remove(SocketPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	ln, err := net.Listen("unix", SocketPath)
	if err != nil {
		return err
	}
	defer ln.Close()
	defer os.Remove(SocketPath)
	if err := os.Chown(SocketPath, 0, int(gid64)); err != nil {
		return err
	}
	if err := os.Chmod(SocketPath, 0660); err != nil {
		return err
	}
	sem := make(chan struct{}, maxConcurrent)
	for {
		c, err := ln.Accept()
		if err != nil {
			return err
		}
		select {
		case sem <- struct{}{}:
			go func() { defer func() { <-sem }(); handle(c) }()
		default:
			_ = c.SetWriteDeadline(time.Now().Add(250 * time.Millisecond))
			write(c, response{Mode: "observation-only", Authorized: false, Message: "busy; request rejected"})
			_ = c.Close()
		}
	}
}

func handle(c net.Conn) {
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(requestTimeout))
	uc, ok := c.(*net.UnixConn)
	if !ok {
		return
	}
	raw, err := uc.SyscallConn()
	if err != nil {
		return
	}
	var cred *syscall.Ucred
	var sockErr error
	if err = raw.Control(func(fd uintptr) {
		cred, sockErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	}); err != nil || sockErr != nil || cred == nil {
		return
	}
	startTime, err := processStartTime(int(cred.Pid))
	if err != nil {
		write(c, response{Mode: "observation-only", PID: int(cred.Pid), UID: cred.Uid, GID: cred.Gid, Authorized: false, Message: "peer process identity unavailable"})
		return
	}
	line, err := readBoundedLine(c, maxLine)
	if err != nil {
		write(c, response{Mode: "observation-only", PID: int(cred.Pid), UID: cred.Uid, GID: cred.Gid, Authorized: false, Message: "invalid or oversized request"})
		return
	}
	var req request
	dec := json.NewDecoder(strings.NewReader(line))
	dec.DisallowUnknownFields()
	if dec.Decode(&req) != nil || req.Operation != "inspect-session" {
		write(c, response{Mode: "observation-only", PID: int(cred.Pid), UID: cred.Uid, GID: cred.Gid, Authorized: false, Message: "request rejected; only inspect-session is supported"})
		return
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		write(c, response{Mode: "observation-only", PID: int(cred.Pid), UID: cred.Uid, GID: cred.Gid, Authorized: false, Message: "trailing JSON data rejected"})
		return
	}
	r := response{OK: true, Mode: "observation-only", PID: int(cred.Pid), UID: cred.Uid, GID: cred.Gid, Authorized: false, Message: "peer credentials read from SO_PEERCRED; no authorization granted"}
	sid, typ, state, e := findSession(int(cred.Pid), cred.Uid)
	if e == nil {
		currentStartTime, identityErr := processStartTime(int(cred.Pid))
		if identityErr != nil || currentStartTime != startTime {
			e = errors.New("peer process identity changed during inspection")
		}
	}
	if e == nil {
		r.SessionID, r.SessionType, r.SessionState = sid, typ, state
		r.Message = "session matched via logind and process cgroup; no authorization granted"
	} else {
		r.OK = false
		r.Message = "no matching logind session: " + e.Error()
	}
	write(c, r)
}

func readBoundedLine(r io.Reader, limit int) (string, error) {
	if limit < 2 {
		return "", errors.New("invalid message limit")
	}
	b, err := io.ReadAll(io.LimitReader(r, int64(limit+1)))
	if err != nil {
		return "", err
	}
	if len(b) == 0 || len(b) > limit {
		return "", errors.New("request too large or empty")
	}
	if b[len(b)-1] != '\n' || strings.Count(string(b), "\n") != 1 {
		return "", errors.New("request must contain exactly one newline-terminated frame")
	}
	return string(b), nil
}

func write(c net.Conn, v response) { b, _ := json.Marshal(v); _, _ = c.Write(append(b, '\n')) }

// processStartTime reads field 22 from /proc/PID/stat. The value changes if
// the PID is reused, allowing the caller to detect identity changes mid-request.
func processStartTime(pid int) (uint64, error) {
	if pid <= 0 {
		return 0, errors.New("invalid peer PID")
	}
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, err
	}
	// comm (field 2) is parenthesized and may itself contain spaces/parentheses.
	close := strings.LastIndexByte(string(b), ')')
	if close < 0 || close+2 >= len(b) {
		return 0, errors.New("malformed process stat")
	}
	fields := strings.Fields(string(b[close+2:]))
	// Remaining fields start at field 3; starttime is field 22 => index 19.
	if len(fields) <= 19 {
		return 0, errors.New("incomplete process stat")
	}
	return strconv.ParseUint(fields[19], 10, 64)
}

func findSession(pid int, uid uint32) (string, string, string, error) {
	if pid <= 0 {
		return "", "", "", errors.New("invalid peer PID")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "loginctl", "list-sessions", "--no-legend", "--no-pager").Output()
	if err != nil {
		return "", "", "", err
	}
	cg, err := os.ReadFile(fmt.Sprintf("/proc/%d/cgroup", pid))
	if err != nil {
		return "", "", "", err
	}
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) < 5 {
			continue
		}
		sid := f[0]
		suid, e := strconv.ParseUint(f[1], 10, 32)
		if e != nil || uint32(suid) != uid {
			continue
		}
		props, e := exec.CommandContext(ctx, "loginctl", "show-session", sid, "-p", "Type", "-p", "State", "-p", "User", "--value").Output()
		if e != nil {
			continue
		}
		vals := strings.Split(strings.TrimSpace(string(props)), "\n")
		if len(vals) < 3 {
			continue
		}
		if !hasSessionScope(string(cg), sid) || vals[2] != strconv.FormatUint(uint64(uid), 10) {
			continue
		}
		return sid, vals[0], vals[1], nil
	}
	return "", "", "", errors.New("process is not in a matching user session cgroup")
}

func hasSessionScope(cgroupData, sid string) bool {
	want := "session-" + sid + ".scope"
	for _, line := range strings.Split(cgroupData, "\n") {
		parts := strings.SplitN(line, ":", 3)
		if len(parts) != 3 {
			continue
		}
		for _, component := range strings.Split(parts[2], "/") {
			if component == want {
				return true
			}
		}
	}
	return false
}

// Probe queries the local observation-only service.
func Probe() (string, error) {
	c, err := net.DialTimeout("unix", SocketPath, 2*time.Second)
	if err != nil {
		return "", err
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(requestTimeout))
	if _, err = c.Write([]byte("{\"operation\":\"inspect-session\"}\n")); err != nil {
		return "", err
	}
	if err = c.(*net.UnixConn).CloseWrite(); err != nil {
		return "", err
	}
	b, err := io.ReadAll(io.LimitReader(c, maxLine))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}
