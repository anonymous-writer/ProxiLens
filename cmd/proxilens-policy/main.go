// proxilens-policy is a root-owned Duo authorization broker for sudo's PAM stack.
// It fails closed on every configuration, peer-identity, and Duo error.
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
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

type config struct{ IKey, SKey, Host, DuoUser, PushType, LocalUser string }

func loadConfig() (config, error) {
	// Reject symlinks: the privileged daemon must only read the fixed root-owned file.
	st, err := os.Lstat(configPath)
	if err != nil {
		return config{}, err
	}
	if st.Mode()&os.ModeSymlink != 0 || !st.Mode().IsRegular() || st.Mode().Perm() != 0600 {
		return config{}, errors.New("Duo-Konfiguration muss eine reguläre Datei mit Modus 0600 sein")
	}
	if sys, ok := st.Sys().(*syscall.Stat_t); !ok || sys.Uid != 0 {
		return config{}, errors.New("Duo-Konfiguration muss root gehören")
	}
	f, err := os.OpenFile(configPath, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return config{}, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil {
		return config{}, err
	}
	if !os.SameFile(st, opened) {
		return config{}, errors.New("Duo-Konfiguration wurde während des Öffnens ausgetauscht")
	}
	if sys, ok := opened.Sys().(*syscall.Stat_t); !ok || sys.Uid != 0 {
		return config{}, errors.New("Duo-Konfiguration muss root gehören")
	}
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
		case "local_username":
			c.LocalUser = v
		}
	}
	if err := sc.Err(); err != nil {
		return config{}, err
	}
	if c.IKey == "" || c.SKey == "" || c.Host == "" || c.DuoUser == "" || c.LocalUser == "" {
		return config{}, errors.New("Duo-Konfiguration unvollständig")
	}
	if strings.ContainsAny(c.IKey+c.SKey+c.Host+c.DuoUser+c.PushType+c.LocalUser, "\r\n\x00") || strings.ContainsAny(c.Host, "/:@?# ") {
		return config{}, errors.New("Duo-Konfiguration enthält ungültige Zeichen")
	}
	return c, nil
}
func procStartTime(pid int32) (string, error) {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return "", err
	}
	// comm is parenthesized and may contain spaces; fields after its final ')' start at field 3.
	i := strings.LastIndexByte(string(b), ')')
	if i < 0 {
		return "", errors.New("ungueltiger proc-stat Datensatz")
	}
	f := strings.Fields(string(b)[i+1:])
	if len(f) <= 19 {
		return "", errors.New("proc-startzeit fehlt")
	}
	return f[19], nil // Linux stat field 22 (starttime).
}

func peerIsSudo(conn *net.UnixConn, pamUser, configuredLocalUser string) error {
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
	if cred == nil || cred.Pid <= 0 {
		return errors.New("SO_PEERCRED fehlt oder ungueltig")
	}
	start, err := procStartTime(cred.Pid)
	if err != nil {
		return err
	}
	exe, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", cred.Pid))
	if err != nil {
		return err
	}
	if filepath.Clean(exe) != "/usr/bin/sudo" &&
		filepath.Clean(exe) != "/bin/sudo" &&
		filepath.Clean(exe) != "/usr/lib/cargo/bin/sudo" {
		return fmt.Errorf("IPC-Aufrufer ist nicht sudo (erkannter Prozesspfad: %q)", exe)
	}
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", cred.Pid))
	if err != nil {
		return err
	}
	var realUID, effectiveUID uint64
	foundUID := false
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "Uid:") {
			f := strings.Fields(line)
			if len(f) < 3 {
				return errors.New("UID-Status unvollstaendig")
			}
			realUID, err = strconv.ParseUint(f[1], 10, 32)
			if err != nil {
				return err
			}
			effectiveUID, err = strconv.ParseUint(f[2], 10, 32)
			if err != nil {
				return err
			}
			foundUID = true
			break
		}
	}
	if !foundUID || effectiveUID != uint64(cred.Uid) {
		return errors.New("Kernel-Peer-UID stimmt nicht mit effektiver Prozess-UID ueberein")
	}
	if pamUser != configuredLocalUser {
		return errors.New("dieser lokale Benutzer ist nicht an das konfigurierte Duo-Konto gebunden")
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
		return errors.New("PAM-Benutzer stimmt nicht mit realer sudo-UID ueberein")
	}
	// Re-check executable and process start time to detect PID reuse/races while inspecting /proc.
	end, err := procStartTime(cred.Pid)
	if err != nil {
		return err
	}
	if end != start {
		return errors.New("sudo-Prozessidentitaet hat sich waehrend der Pruefung geaendert")
	}
	exe2, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", cred.Pid))
	if err != nil || exe2 != exe {
		return errors.New("sudo-Prozess wurde waehrend der Pruefung ausgetauscht")
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
	if err = peerIsSudo(c, name, cfg.LocalUser); err != nil {
		log.Printf("sudo-Peer-Prüfung abgelehnt: %v", err)
		return
	}
	dc := duo.Config{IKey: cfg.IKey, SKey: cfg.SKey, Host: cfg.Host, Username: cfg.DuoUser, PushType: cfg.PushType}
	ctx, cancel := context.WithTimeout(context.Background(), 95*time.Second)
	defer cancel()
	if err = dc.Push(ctx, nil); err != nil {
		log.Printf("Duo-Push fehlgeschlagen: %v", err)
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
	dir := filepath.Dir(socketPath)
	if err = os.Mkdir(dir, 0755); err != nil && !errors.Is(err, os.ErrExist) {
		panic(err)
	}
	dirInfo, err := os.Lstat(dir)
	if err != nil || dirInfo.Mode()&os.ModeSymlink != 0 || !dirInfo.IsDir() {
		fmt.Fprintln(os.Stderr, "Policy-Socket-Verzeichnis ist kein sicheres Verzeichnis")
		os.Exit(1)
	}
	if sys, ok := dirInfo.Sys().(*syscall.Stat_t); !ok || sys.Uid != 0 {
		fmt.Fprintln(os.Stderr, "Policy-Socket-Verzeichnis muss root gehören")
		os.Exit(1)
	}
	if dirInfo.Mode().Perm() != 0755 {
		if err = os.Chmod(dir, 0755); err != nil {
			panic(err)
		}
	}
	if st, e := os.Lstat(socketPath); e == nil {
		if st.Mode()&os.ModeSocket == 0 || st.Mode()&os.ModeSymlink != 0 {
			fmt.Fprintln(os.Stderr, "Socket-Pfad existiert und ist kein sicherer Socket")
			os.Exit(1)
		}
		if sys, ok := st.Sys().(*syscall.Stat_t); !ok || sys.Uid != 0 {
			fmt.Fprintln(os.Stderr, "Vorhandener Policy-Socket gehört nicht root")
			os.Exit(1)
		}
		if err = os.Remove(socketPath); err != nil {
			panic(err)
		}
	} else if !errors.Is(e, os.ErrNotExist) {
		panic(e)
	}
	oldMask := syscall.Umask(0077)
	defer syscall.Umask(oldMask)
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
