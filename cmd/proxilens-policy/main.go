// proxilens-policy is a root-owned Duo authorization broker for sudo's PAM stack.
// It fails closed on every configuration, peer-identity, and Duo error.
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/local/proxilens/internal/duo"
)

const socketPath = "/run/proxilens-policy/auth.sock"
const configPath = "/etc/proxilens/duo.conf"

type config struct{ IKey, SKey, Host, DuoUser, PushType string }

func loadConfig() (config, error) {
	st, err := os.Stat(configPath)
	if err != nil {
		return config{}, err
	}
	if !st.Mode().IsRegular() || st.Mode().Perm()&0077 != 0 {
		return config{}, errors.New("Duo-Konfiguration muss eine reguläre Datei mit Modus 0600 sein")
	}
	if sys, ok := st.Sys().(*syscall.Stat_t); !ok || sys.Uid != 0 {
		return config{}, errors.New("Duo-Konfiguration muss root gehören")
	}
	f, err := os.Open(configPath)
	if err != nil {
		return config{}, err
	}
	defer f.Close()
	var c config
	section := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			continue
		}
		if section != "duo" {
			continue
		}
		p := strings.SplitN(line, "=", 2)
		if len(p) != 2 {
			continue
		}
		k, v := strings.ToLower(strings.TrimSpace(p[0])), strings.TrimSpace(p[1])
		switch k {
		case "ikey":
			c.IKey = v
		case "skey":
			c.SKey = v
		case "host":
			c.Host = v
		case "username":
			c.DuoUser = v
		case "push_type":
			c.PushType = v
		}
	}
	if err := sc.Err(); err != nil {
		return config{}, err
	}
	if c.IKey == "" || c.SKey == "" || c.Host == "" || c.DuoUser == "" {
		return config{}, errors.New("Duo-Konfiguration unvollständig")
	}
	return c, nil
}
func peerIsSudo(conn *net.UnixConn, pamUser string) error {
	raw, err := conn.SyscallConn()
	if err != nil {
		return err
	}
	var cred *syscall.Ucred
	var sockErr error
	if err = raw.Control(func(fd uintptr) {
		cred, sockErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	}); err != nil {
		return err
	}
	if sockErr != nil {
		return sockErr
	}
	if cred == nil {
		return errors.New("SO_PEERCRED fehlt")
	}
	exe, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", cred.Pid))
	if err != nil {
		return err
	}
	if filepath.Clean(exe) != "/usr/bin/sudo" && filepath.Clean(exe) != "/bin/sudo" {
		return errors.New("IPC-Aufrufer ist nicht sudo")
	}
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", cred.Pid))
	if err != nil {
		return err
	}
	var realUID uint64
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "Uid:") {
			f := strings.Fields(line)
			if len(f) < 2 {
				return errors.New("UID-Status unvollständig")
			}
			realUID, err = strconv.ParseUint(f[1], 10, 32)
			if err != nil {
				return err
			}
			break
		}
	}
	u, err := user.Lookup(pamUser)
	if err != nil {
		return err
	}
	want, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return err
	}
	if realUID != want {
		return errors.New("PAM-Benutzer stimmt nicht mit realer sudo-UID überein")
	}
	return nil
}
func handle(c *net.UnixConn, cfg config) {
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(100 * time.Second))
	rd := bufio.NewReaderSize(c, 256)
	name, err := rd.ReadString('\n')
	if err != nil || len(name) > 130 || !strings.HasSuffix(name, "\n") {
		return
	}
	name = strings.TrimSuffix(name, "\n")
	if name == "" || strings.ContainsAny(name, "\r\x00") {
		return
	}
	if err = peerIsSudo(c, name); err != nil {
		return
	}
	dc := duo.Config{IKey: cfg.IKey, SKey: cfg.SKey, Host: cfg.Host, Username: cfg.DuoUser, PushType: cfg.PushType}
	ctx, cancel := context.WithTimeout(context.Background(), 95*time.Second)
	defer cancel()
	if err = dc.Push(ctx, nil); err != nil {
		return
	}
	_, _ = c.Write([]byte("ALLOW\n"))
}
func main() {
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "proxilens-policy muss als root laufen")
		os.Exit(1)
	}
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "ProxiLens Duo-Konfiguration abgelehnt:", err)
		os.Exit(1)
	}
	if err = os.MkdirAll(filepath.Dir(socketPath), 0755); err != nil {
		panic(err)
	}
	_ = os.Chown(filepath.Dir(socketPath), 0, 0)
	_ = os.Chmod(filepath.Dir(socketPath), 0755)
	if st, e := os.Lstat(socketPath); e == nil {
		if st.Mode()&os.ModeSocket == 0 {
			fmt.Fprintln(os.Stderr, "Socket-Pfad existiert und ist kein Socket")
			os.Exit(1)
		}
		_ = os.Remove(socketPath)
	}
	ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: socketPath, Net: "unix"})
	if err != nil {
		panic(err)
	}
	defer os.Remove(socketPath)
	_ = os.Chown(socketPath, 0, 0)
	_ = os.Chmod(socketPath, 0600)
	for {
		c, e := ln.AcceptUnix()
		if e != nil {
			continue
		}
		go handle(c, cfg)
	}
}
