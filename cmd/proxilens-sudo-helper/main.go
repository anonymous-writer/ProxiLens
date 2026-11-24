//go:build linux

// proxilens-sudo-helper is both the privileged setup utility and the PAM
// authentication helper. It is installed root:root and must never be
// writable by an unprivileged user.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/local/proxilens/internal/duo"
)

const (
	configPath  = "/etc/proxilens/sudo.json"
	statusPath  = "/etc/proxilens/sudo-status.json"
	pamPath     = "/etc/pam.d/sudo"
	pamMarker   = "# ProxiLens sudo Duo authentication"
	pamLine     = pamMarker + "\nauth required pam_exec.so quiet seteuid /usr/lib/proxilens/proxilens-sudo-helper pam-auth"
	sudoersPath = "/etc/sudoers.d/proxilens"
	helperDir   = "/run/proxilens/sudo-grants"
)

type config struct {
	Enabled     bool   `json:"enabled"`
	DurationMin int    `json:"duration_minutes"`
	IKey        string `json:"ikey"`
	SKey        string `json:"skey"`
	Host        string `json:"host"`
	Username    string `json:"username"`
	LocalUser   string `json:"local_user"`
	PushType    string `json:"push_type"`
}

type status struct {
	Enabled     bool   `json:"enabled"`
	DurationMin int    `json:"duration_minutes"`
	LocalUser   string `json:"local_user"`
}

type grant struct {
	UID       int       `json:"uid"`
	Scope     string    `json:"session_scope"`
	GrantedAt time.Time `json:"granted_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

func main() {
	if os.Geteuid() != 0 {
		fatal("Dieses Hilfsprogramm muss über pkexec mit Administratorrechten gestartet werden.")
	}
	if len(os.Args) < 2 {
		fatal("Verwendung: configure | disable | status | pam-auth")
	}
	var err error
	switch os.Args[1] {
	case "configure":
		err = configure()
	case "disable":
		err = disable()
	case "status":
		err = printStatus()
	case "pam-auth":
		err = pamAuth()
	default:
		err = fmt.Errorf("unbekannte Aktion")
	}
	if err != nil {
		fatal(err.Error())
	}
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, "ProxiLens:", message)
	os.Exit(1)
}

func configure() error {
	dec := json.NewDecoder(io.LimitReader(os.Stdin, 16*1024))
	dec.DisallowUnknownFields()
	var cfg config
	if err := dec.Decode(&cfg); err != nil {
		return fmt.Errorf("Konfiguration konnte nicht gelesen werden: %w", err)
	}
	if !cfg.Enabled {
		return errors.New("Aktivierung wurde nicht bestätigt")
	}
	if cfg.DurationMin < 1 || cfg.DurationMin > 120 {
		return errors.New("Freigabedauer muss zwischen 1 und 120 Minuten liegen")
	}
	if !validLocalUsername(cfg.LocalUser) {
		return errors.New("lokaler Ubuntu-Benutzername fehlt oder ist ungültig")
	}
	if strings.TrimSpace(cfg.PushType) == "" {
		cfg.PushType = "ProxiLens sudo approval"
	}
	duoCfg := duo.Config{IKey: cfg.IKey, SKey: cfg.SKey, Host: cfg.Host, Username: cfg.Username, PushType: cfg.PushType}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := duoCfg.Check(ctx); err != nil {
		return fmt.Errorf("Duo-Konfiguration konnte nicht geprüft werden: %w", err)
	}

	pamOriginal, err := os.ReadFile(pamPath)
	if err != nil {
		return fmt.Errorf("PAM-Konfiguration lesen: %w", err)
	}
	if !strings.Contains(string(pamOriginal), "@include common-auth") {
		return errors.New("/etc/pam.d/sudo enthält keine erwartete @include common-auth-Zeile; aus Sicherheitsgründen keine automatische Änderung")
	}
	pamUpdated := addPAMLine(string(pamOriginal))
	if !strings.Contains(pamUpdated, pamMarker) {
		return errors.New("PAM-Regel konnte nicht sicher eingefügt werden")
	}
	sudoersUpdated := []byte(fmt.Sprintf("# Managed by ProxiLens; do not edit.\nDefaults:%s timestamp_timeout=0\n", cfg.LocalUser))
	if err := validateSudoers(sudoersUpdated); err != nil {
		return err
	}

	oldConfig, configErr := os.ReadFile(configPath)
	configExisted := configErr == nil
	if configErr != nil && !errors.Is(configErr, os.ErrNotExist) {
		return configErr
	}
	oldStatus, statusErr := os.ReadFile(statusPath)
	statusExisted := statusErr == nil
	if statusErr != nil && !errors.Is(statusErr, os.ErrNotExist) {
		return statusErr
	}
	oldSudoers, sudoersErr := os.ReadFile(sudoersPath)
	sudoersExisted := sudoersErr == nil
	if sudoersErr != nil && !errors.Is(sudoersErr, os.ErrNotExist) {
		return sudoersErr
	}

	rollback := func() {
		restoreFile(configPath, oldConfig, configExisted, 0600)
		restoreFile(statusPath, oldStatus, statusExisted, 0644)
		restoreFile(sudoersPath, oldSudoers, sudoersExisted, 0440)
		restoreFile(pamPath, pamOriginal, true, 0644)
	}
	cfg.Enabled = true
	if err := ensureProtectedDir(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("sudo-Konfigurationsverzeichnis: %w", err)
	}
	if err := ensureProtectedDir(helperDir, 0700); err != nil {
		return fmt.Errorf("Duo-Freigabespeicher: %w", err)
	}
	if err := writeJSON(configPath, cfg, 0600); err != nil {
		return fmt.Errorf("Root-Konfiguration schreiben: %w", err)
	}
	// Publish the non-secret status and sudoers policy before adding the PAM
	// hook. Thus PAM is never left pointing at a missing status/config file.
	if err := writeJSON(statusPath, status{Enabled: true, DurationMin: cfg.DurationMin, LocalUser: cfg.LocalUser}, 0644); err != nil {
		rollback()
		return fmt.Errorf("Statusdatei schreiben: %w", err)
	}
	if err := atomicWrite(sudoersPath, sudoersUpdated, 0440); err != nil {
		rollback()
		return fmt.Errorf("sudo-Konfiguration aktualisieren: %w", err)
	}
	if err := atomicWrite(pamPath, []byte(pamUpdated), 0644); err != nil {
		rollback()
		return fmt.Errorf("PAM-Konfiguration aktualisieren: %w", err)
	}
	fmt.Println("ProxiLens sudo-Schutz wurde aktiviert.")
	return nil
}

func disable() error {
	pamOriginal, err := os.ReadFile(pamPath)
	if err != nil {
		return err
	}
	pamUpdated := removePAMLine(string(pamOriginal))
	if err := atomicWrite(pamPath, []byte(pamUpdated), 0644); err != nil {
		return err
	}
	if err := os.Remove(sudoersPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		_ = atomicWrite(pamPath, pamOriginal, 0644)
		return fmt.Errorf("sudoers-Regel entfernen: %w", err)
	}
	_ = os.Remove(configPath)
	_ = os.Remove(statusPath)
	_ = os.RemoveAll(helperDir)
	fmt.Println("ProxiLens sudo-Schutz wurde deaktiviert. Andere PAM-Regeln wurden nicht verändert.")
	return nil
}

func printStatus() error {
	data, err := os.ReadFile(statusPath)
	if errors.Is(err, os.ErrNotExist) {
		fmt.Println(`{"enabled":false,"duration_minutes":15}`)
		return nil
	}
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(data)
	return err
}

func pamAuth() error {
	if os.Getenv("PAM_SERVICE") != "sudo" || os.Getenv("PAM_TYPE") != "auth" {
		return errors.New("PAM-Aufruf außerhalb des sudo-Authentifizierungsstapels verweigert")
	}
	username := strings.TrimSpace(os.Getenv("PAM_USER"))
	tty := strings.TrimSpace(os.Getenv("PAM_TTY"))
	if username == "" {
		return errors.New("sudo-Duo benötigt einen eindeutig zugeordneten Benutzer")
	}
	dirInfo, dirErr := os.Lstat(filepath.Dir(configPath))
	if dirErr != nil || !dirInfo.IsDir() || dirInfo.Mode()&os.ModeSymlink != 0 {
		return errors.New("sudo-Duo-Konfigurationsverzeichnis fehlt oder ist unsicher")
	}
	if dirStat, ok := dirInfo.Sys().(*syscall.Stat_t); !ok || dirStat.Uid != 0 || dirInfo.Mode().Perm()&0022 != 0 {
		return errors.New("sudo-Duo-Konfigurationsverzeichnis ist nicht root-geschützt")
	}
	statusInfo, statusStatErr := os.Lstat(statusPath)
	if statusStatErr != nil || !statusInfo.Mode().IsRegular() || statusInfo.Mode().Perm()&0022 != 0 {
		return errors.New("sudo-Duo-Status fehlt oder ist unsicher")
	}
	if statusStat, ok := statusInfo.Sys().(*syscall.Stat_t); !ok || statusStat.Uid != 0 {
		return errors.New("sudo-Duo-Status gehört nicht root")
	}
	statusData, statusReadErr := os.ReadFile(statusPath)
	if statusReadErr != nil {
		return errors.New("sudo-Duo-Status konnte nicht gelesen werden")
	}
	var configuredStatus status
	if json.Unmarshal(statusData, &configuredStatus) != nil || !configuredStatus.Enabled || !validLocalUsername(configuredStatus.LocalUser) {
		return errors.New("sudo-Duo-Status ist ungültig")
	}
	if username != configuredStatus.LocalUser {
		return nil
	}
	u, err := user.Lookup(username)
	if err != nil {
		return errors.New("PAM-Benutzer konnte nicht aufgelöst werden")
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil || uid <= 0 {
		return errors.New("ungültige Benutzer-ID")
	}
	if strings.Contains(tty, "..") || strings.ContainsRune(tty, 0) {
		return errors.New("ungültige Terminalkennung")
	}
	configInfo, statErr := os.Lstat(configPath)
	if statErr != nil || !configInfo.Mode().IsRegular() || configInfo.Mode().Perm()&0077 != 0 {
		return errors.New("sudo-Duo-Konfiguration fehlt oder hat unsichere Dateirechte")
	}
	if stat, ok := configInfo.Sys().(*syscall.Stat_t); !ok || stat.Uid != 0 {
		return errors.New("sudo-Duo-Konfiguration gehört nicht root")
	}
	cfgData, err := os.ReadFile(configPath)
	if err != nil {
		return errors.New("sudo-Duo ist nicht konfiguriert")
	}
	var cfg config
	if err := json.Unmarshal(cfgData, &cfg); err != nil || !cfg.Enabled {
		return errors.New("sudo-Duo-Konfiguration ist ungültig oder deaktiviert")
	}
	if cfg.DurationMin < 1 || cfg.DurationMin > 120 || !validLocalUsername(cfg.LocalUser) || cfg.LocalUser != configuredStatus.LocalUser {
		return errors.New("ungültige sudo-Freigabedauer oder Benutzerbindung")
	}
	if username != cfg.LocalUser {
		return nil // Die ProxiLens-Regel gilt nur für den Benutzer, der sie aktiviert hat.
	}
	scope, err := authenticationSessionScope(tty, strings.TrimSpace(os.Getenv("XDG_SESSION_ID")))
	if err != nil {
		return errors.New("PAM-Sitzung konnte nicht eindeutig zugeordnet werden")
	}
	if err := ensureProtectedDir(helperDir, 0700); err != nil {
		return errors.New("Duo-Freigabespeicher nicht verfügbar oder nicht root-geschützt")
	}
	lock, err := os.OpenFile(filepath.Join(helperDir, "lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return errors.New("Duo-Freigabe kann nicht gesperrt werden")
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return errors.New("Duo-Freigabe kann nicht exklusiv gesperrt werden")
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)

	hash := sha256.Sum256([]byte(scope))
	grantPath := filepath.Join(helperDir, fmt.Sprintf("%d-%s.json", uid, hex.EncodeToString(hash[:8])))
	if data, err := os.ReadFile(grantPath); err == nil {
		var g grant
		if json.Unmarshal(data, &g) == nil && g.UID == uid && g.Scope == scope && time.Now().Before(g.ExpiresAt) {
			return nil
		}
		_ = os.Remove(grantPath)
	}

	duoCfg := duo.Config{IKey: cfg.IKey, SKey: cfg.SKey, Host: cfg.Host, Username: cfg.Username, PushType: cfg.PushType}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()
	if err := duoCfg.Push(ctx, nil); err != nil {
		return fmt.Errorf("Duo-Bestätigung fehlgeschlagen: %w", err)
	}
	now := time.Now()
	g := grant{UID: uid, Scope: scope, GrantedAt: now, ExpiresAt: now.Add(time.Duration(cfg.DurationMin) * time.Minute)}
	if err := writeJSON(grantPath, g, 0600); err != nil {
		return errors.New("Duo erfolgreich, aber Freigabe konnte nicht sicher gespeichert werden")
	}
	return nil
}

func validLocalUsername(name string) bool {
	if name == "" || len(name) > 32 {
		return false
	}
	for i, r := range name {
		letter := r >= 'a' && r <= 'z'
		underscore := r == '_'
		digit := r >= '0' && r <= '9'
		hyphen := r == '-'
		if i == 0 && !(letter || underscore) {
			return false
		}
		if i > 0 && !(letter || underscore || digit || hyphen) {
			return false
		}
	}
	return true
}

// authenticationSessionScope prefers the logind session ID exported by
// pam_systemd. This shares the approval across terminals in one GNOME login
// session. On systems without XDG_SESSION_ID, it falls back to the kernel
// process-session ID plus PAM_TTY, preventing cross-terminal reuse.
func authenticationSessionScope(tty, logindID string) (string, error) {
	if logindID != "" {
		for _, r := range logindID {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.') {
				return "", errors.New("invalid logind session id")
			}
		}
		return "logind:" + logindID, nil
	}
	if tty == "" || tty == "unknown" || strings.Contains(tty, "..") || strings.ContainsRune(tty, 0) {
		return "", errors.New("missing terminal and logind session id")
	}
	sid, err := processSessionID()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("process:%d|tty:%s", sid, tty), nil
}

// processSessionID reads the kernel session ID of the PAM helper process.
// It binds a grant to the login session as well as the user and terminal.
func processSessionID() (int, error) {
	data, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return 0, err
	}
	end := strings.LastIndex(string(data), ")")
	if end < 0 {
		return 0, errors.New("invalid proc stat")
	}
	fields := strings.Fields(string(data)[end+1:])
	// The first field after comm is state (field 3); session is field 6.
	if len(fields) < 4 {
		return 0, errors.New("missing session id")
	}
	id, err := strconv.Atoi(fields[3])
	if err != nil || id <= 0 {
		return 0, errors.New("invalid session id")
	}
	return id, nil
}

func addPAMLine(contents string) string {
	if strings.Contains(contents, pamMarker) {
		return contents
	}
	lines := strings.Split(contents, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "@include common-auth" {
			insert := []string{pamMarker, strings.TrimPrefix(pamLine, pamMarker+"\n")}
			lines = append(lines[:i+1], append(insert, lines[i+1:]...)...)
			return strings.Join(lines, "\n")
		}
	}
	return contents
}

func removePAMLine(contents string) string {
	lines := strings.Split(contents, "\n")
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == pamMarker {
			if i+1 < len(lines) && strings.Contains(lines[i+1], "proxilens-sudo-helper pam-auth") {
				i++
			}
			continue
		}
		out = append(out, lines[i])
	}
	return strings.Join(out, "\n")
}

func validateSudoers(data []byte) error {
	f, err := os.CreateTemp("/etc/sudoers.d", ".proxilens-check-*")
	if err != nil {
		return fmt.Errorf("sudoers-Prüfdatei erstellen: %w", err)
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(0440); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	cmd := exec.Command("/usr/sbin/visudo", "-cf", name)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("sudoers-Regel ist ungültig: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

func ensureProtectedDir(path string, mode os.FileMode) error {
	if err := os.MkdirAll(path, mode); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("not a real directory")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != 0 || info.Mode().Perm()&0022 != 0 {
		return errors.New("directory is not root-owned and protected")
	}
	if err := os.Chmod(path, mode); err != nil {
		return err
	}
	return nil
}

func writeJSON(path string, value any, mode os.FileMode) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(path, append(data, '\n'), mode)
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".proxilens-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func restoreFile(path string, data []byte, existed bool, mode os.FileMode) {
	if existed {
		_ = atomicWrite(path, data, mode)
	} else {
		_ = os.Remove(path)
	}
}
