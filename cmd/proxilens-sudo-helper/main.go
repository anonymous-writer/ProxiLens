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
	configPath              = "/etc/proxilens/sudo.json"
	statusPath              = "/etc/proxilens/sudo-status.json"
	pamBackupPath           = "/etc/proxilens/sudo-pam.original"
	polkitPAMBackupPath     = "/etc/proxilens/polkit-1-pam.original"
	polkitPAMWasAbsentPath  = "/etc/proxilens/polkit-1-pam.was-absent"
	pamPath                 = "/etc/pam.d/sudo"
	polkitPAMPath           = "/etc/pam.d/polkit-1"
	polkitPAMTemplatePath   = "/usr/lib/pam.d/polkit-1"
	pamMarker               = "# ProxiLens sudo Duo authentication"
	pamPasswordBypassMarker = "# ProxiLens Duo-only: skip Ubuntu password only for the configured user."
	pamLine                 = pamMarker + "\nauth required pam_exec.so quiet /usr/lib/proxilens/proxilens-sudo-helper pam-auth"
	sudoersPath             = "/etc/sudoers.d/proxilens"
	helperDir               = "/run/proxilens/sudo-grants"
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

	pamCurrent, err := os.ReadFile(pamPath)
	if err != nil {
		return fmt.Errorf("PAM-Konfiguration lesen: %w", err)
	}
	// Preserve the exact pre-ProxiLens PAM file so disable can restore the
	// normal Ubuntu password stack. On upgrades, reuse the existing backup.
	pamBase := removePAMLine(string(pamCurrent))
	if backup, backupErr := os.ReadFile(pamBackupPath); backupErr == nil {
		if !safeRootFile(pamBackupPath, 0600) {
			return errors.New("PAM-Sicherungsdatei fehlt oder ist nicht root-geschützt")
		}
		pamBase = string(backup)
	} else if !errors.Is(backupErr, os.ErrNotExist) {
		return fmt.Errorf("PAM-Sicherungsdatei lesen: %w", backupErr)
	}
	if !strings.Contains(pamBase, "@include common-auth") {
		return errors.New("ursprüngliche /etc/pam.d/sudo enthält keine erwartete @include common-auth-Zeile; keine automatische Änderung")
	}
	pamUpdated := duoOnlyPAM(pamBase, cfg.LocalUser)
	if !strings.Contains(pamUpdated, pamMarker) || strings.Contains(pamUpdated, "@include common-auth") || !strings.Contains(pamUpdated, "auth substack common-auth") {
		return errors.New("Duo-only-PAM-Regel konnte nicht sicher erstellt werden")
	}
	// Polkit is a separate PAM service. Prefer an existing /etc override; if
	// absent, derive it from the vendor template and remember to remove the
	// override on disable rather than modifying /usr/lib.
	polkitCurrent, polkitReadErr := os.ReadFile(polkitPAMPath)
	polkitWasAbsent := errors.Is(polkitReadErr, os.ErrNotExist)
	if polkitReadErr != nil && !polkitWasAbsent {
		return fmt.Errorf("Polkit-PAM lesen: %w", polkitReadErr)
	}
	if polkitWasAbsent {
		polkitCurrent, err = os.ReadFile(polkitPAMTemplatePath)
		if err != nil {
			return fmt.Errorf("Polkit-PAM-Vorlage lesen: %w", err)
		}
	}
	polkitBase := removePAMLine(string(polkitCurrent))
	if backup, e := os.ReadFile(polkitPAMBackupPath); e == nil {
		if !safeRootFile(polkitPAMBackupPath, 0600) {
			return errors.New("Polkit-PAM-Sicherung ist unsicher")
		}
		polkitBase = string(backup)
	} else if !errors.Is(e, os.ErrNotExist) {
		return e
	}
	polkitUpdated := duoOnlyPAM(polkitBase, cfg.LocalUser)
	if !strings.Contains(polkitUpdated, pamMarker) || strings.Contains(polkitUpdated, "@include common-auth") || !strings.Contains(polkitUpdated, "auth substack common-auth") {
		return errors.New("Polkit-Duo-PAM-Regel konnte nicht sicher erstellt werden")
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
	oldPAMBackup, pamBackupErr := os.ReadFile(pamBackupPath)
	pamBackupExisted := pamBackupErr == nil
	if pamBackupErr != nil && !errors.Is(pamBackupErr, os.ErrNotExist) {
		return pamBackupErr
	}
	oldPolkitPAM, polkitPAMErr := os.ReadFile(polkitPAMPath)
	polkitPAMExisted := polkitPAMErr == nil
	if polkitPAMErr != nil && !errors.Is(polkitPAMErr, os.ErrNotExist) {
		return polkitPAMErr
	}
	oldPolkitBackup, polkitBackupErr := os.ReadFile(polkitPAMBackupPath)
	polkitBackupExisted := polkitBackupErr == nil
	if polkitBackupErr != nil && !errors.Is(polkitBackupErr, os.ErrNotExist) {
		return polkitBackupErr
	}
	oldPolkitAbsent, polkitAbsentErr := os.ReadFile(polkitPAMWasAbsentPath)
	polkitAbsentExisted := polkitAbsentErr == nil
	if polkitAbsentErr != nil && !errors.Is(polkitAbsentErr, os.ErrNotExist) {
		return polkitAbsentErr
	}

	rollback := func() {
		restoreFile(configPath, oldConfig, configExisted, 0600)
		restoreFile(statusPath, oldStatus, statusExisted, 0644)
		restoreFile(sudoersPath, oldSudoers, sudoersExisted, 0440)
		restoreFile(pamPath, pamCurrent, true, 0644)
		restoreFile(pamBackupPath, oldPAMBackup, pamBackupExisted, 0600)
		restoreFile(polkitPAMPath, oldPolkitPAM, polkitPAMExisted, 0644)
		restoreFile(polkitPAMBackupPath, oldPolkitBackup, polkitBackupExisted, 0600)
		restoreFile(polkitPAMWasAbsentPath, oldPolkitAbsent, polkitAbsentExisted, 0600)
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
	if !pamBackupExisted {
		if err := atomicWrite(pamBackupPath, []byte(pamBase), 0600); err != nil {
			rollback()
			return fmt.Errorf("ursprüngliche PAM-Konfiguration sichern: %w", err)
		}
	}
	if !polkitBackupExisted {
		if err := atomicWrite(polkitPAMBackupPath, []byte(polkitBase), 0600); err != nil {
			rollback()
			return fmt.Errorf("ursprüngliche Polkit-PAM-Konfiguration sichern: %w", err)
		}
		if polkitWasAbsent {
			if err := atomicWrite(polkitPAMWasAbsentPath, []byte("yes\n"), 0600); err != nil {
				rollback()
				return fmt.Errorf("Polkit-PAM-Zustand sichern: %w", err)
			}
		}
	}
	if err := atomicWrite(polkitPAMPath, []byte(polkitUpdated), 0644); err != nil {
		rollback()
		return fmt.Errorf("Polkit-PAM aktualisieren: %w", err)
	}
	if err := atomicWrite(pamPath, []byte(pamUpdated), 0644); err != nil {
		rollback()
		return fmt.Errorf("PAM-Konfiguration aktualisieren: %w", err)
	}
	fmt.Println("ProxiLens Duo-Schutz für sudo und grafische Polkit-Dialoge wurde aktiviert.")
	return nil
}

func disable() error {
	if backup, err := os.ReadFile(pamBackupPath); err == nil {
		if !safeRootFile(pamBackupPath, 0600) {
			return errors.New("PAM-Sicherungsdatei ist nicht root-geschützt; Deaktivierung abgebrochen")
		}
		if err := atomicWrite(pamPath, backup, 0644); err != nil {
			return fmt.Errorf("ursprüngliche PAM-Konfiguration wiederherstellen: %w", err)
		}
		if err := os.Remove(pamBackupPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("PAM-Sicherung entfernen: %w", err)
		}
	} else if errors.Is(err, os.ErrNotExist) {
		// Legacy installation without a backup: remove only the marked rule and
		// restore common-auth if the earlier version had commented it out.
		current, readErr := os.ReadFile(pamPath)
		if readErr != nil {
			return readErr
		}
		updated := restoreCommonAuth(removePAMLine(string(current)))
		if err := atomicWrite(pamPath, []byte(updated), 0644); err != nil {
			return err
		}
	} else {
		return err
	}
	if backup, err := os.ReadFile(polkitPAMBackupPath); err == nil {
		if !safeRootFile(polkitPAMBackupPath, 0600) {
			return errors.New("Polkit-PAM-Sicherung ist nicht root-geschützt; Deaktivierung abgebrochen")
		}
		if _, absentErr := os.Stat(polkitPAMWasAbsentPath); absentErr == nil {
			if err := os.Remove(polkitPAMPath); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("Polkit-PAM-Override entfernen: %w", err)
			}
		} else if err := atomicWrite(polkitPAMPath, backup, 0644); err != nil {
			return fmt.Errorf("Polkit-PAM wiederherstellen: %w", err)
		}
		_ = os.Remove(polkitPAMBackupPath)
		_ = os.Remove(polkitPAMWasAbsentPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Remove(sudoersPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("sudoers-Regel entfernen: %w", err)
	}
	_ = os.Remove(configPath)
	_ = os.Remove(statusPath)
	_ = os.RemoveAll(helperDir)
	fmt.Println("ProxiLens Duo-Schutz wurde deaktiviert; die ursprünglichen PAM-Regeln wurden wiederhergestellt.")
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

func validPAMAuthService(service, pamType string) bool {
	return (service == "sudo" || service == "polkit-1") && pamType == "auth"
}

func pamAuth() error {
	service := os.Getenv("PAM_SERVICE")
	if !validPAMAuthService(service, os.Getenv("PAM_TYPE")) {
		return errors.New("PAM-Aufruf außerhalb des Sudo-/Polkit-Authentifizierungsstapels verweigert")
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
	scope, err := activeLogindSessionScope(uid)
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

// activeLogindSessionScope resolves the single active foreground desktop
// session for this UID. It works for both sudo terminals and graphical Polkit
// prompts, whose PAM environment does not reliably contain XDG_SESSION_ID.
// Ambiguous multi-seat/multi-session cases fail closed.
func activeLogindSessionScope(uid int) (string, error) {
	if uid <= 0 {
		return "", errors.New("invalid uid")
	}
	out, err := exec.Command("/usr/bin/loginctl", "show-user", strconv.Itoa(uid), "-p", "Sessions", "--value").Output()
	if err != nil {
		return "", fmt.Errorf("logind user sessions unavailable: %w", err)
	}
	ids := strings.Fields(string(out))
	var active []string
	for _, id := range ids {
		if id == "" || strings.ContainsAny(id, "/\\") {
			continue
		}
		props, e := exec.Command("/usr/bin/loginctl", "show-session", id, "-p", "User", "-p", "Seat", "-p", "Type", "-p", "Class", "-p", "State", "--value").Output()
		if e != nil {
			continue
		}
		f := strings.Split(strings.TrimSpace(string(props)), "\n")
		if len(f) != 5 {
			continue
		}
		if f[0] == strconv.Itoa(uid) && f[1] == "seat0" && (f[2] == "wayland" || f[2] == "x11") && f[3] == "user" && f[4] == "active" {
			active = append(active, id)
		}
	}
	if len(active) != 1 {
		return "", fmt.Errorf("expected exactly one active desktop session, found %d", len(active))
	}
	return "logind:" + active[0], nil
}

// authenticationSessionScope is retained for unit tests and rejects terminal-only
// fallback: a TTY is not a stable cross-service desktop-session identity.
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

func duoOnlyPAM(contents, localUser string) string {
	// An already-installed Duo-only stack is idempotent.
	if strings.Contains(contents, pamPasswordBypassMarker) && strings.Contains(contents, pamMarker) {
		return contents
	}
	// Remove previous ProxiLens lines and replace Ubuntu's password stack
	// with the Duo-only gate. Account and session policy remain untouched.
	contents = removePAMLine(contents)
	lines := strings.Split(contents, "\n")
	out := make([]string, 0, len(lines)+2)
	found := false
	for _, line := range lines {
		if strings.TrimSpace(line) == "@include common-auth" {
			if found {
				return contents
			}
			found = true
			out = append(out, pamPasswordBypassMarker)
			out = append(out, "auth [success=1 default=ignore] pam_succeed_if.so user = "+localUser)
			out = append(out, "auth substack common-auth")
			out = append(out, pamMarker, strings.TrimPrefix(pamLine, pamMarker+"\n"))
			continue
		}
		out = append(out, line)
	}
	if !found {
		return contents
	}
	return strings.Join(out, "\n")
}

func safeRootFile(path string, mode os.FileMode) bool {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != mode {
		return false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == 0
}

// restoreCommonAuth reverses the legacy Duo-only PAM transformation when no
// pre-ProxiLens backup exists. It removes only ProxiLens-owned directives and
// restores Ubuntu's standard common-auth include.
func restoreCommonAuth(contents string) string {
	lines := strings.Split(contents, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == pamPasswordBypassMarker {
			continue
		}
		if strings.HasPrefix(trimmed, "auth [success=1 default=ignore] pam_succeed_if.so user = ") {
			continue
		}
		if trimmed == "auth substack common-auth" {
			out = append(out, "@include common-auth")
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
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
