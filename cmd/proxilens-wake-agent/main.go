// proxilens-wake-agent watches logind resume events and the logind LockedHint
// state. Normal screen-lock transitions are observed only; Duo is triggered
// exclusively after resume from sleep, and only if the session remains locked.
package main

import (
	"bufio"
	"context"
	"log"
	"net"
	"net/url"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/local/proxilens/internal/authelia"
	"github.com/local/proxilens/internal/configstore"
	"github.com/local/proxilens/internal/session"
)

type coordinator struct {
	mu        sync.Mutex
	sessionID string
	locked    bool
	known     bool
	triggered bool // one resume authentication attempt per lock cycle
	running   bool
}

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)
	log.Println("ProxiLens experimental agent started; watching logind resume and LockedHint")
	c := &coordinator{}
	go c.watchLockedHint()
	go c.watchDuoButton()
	c.watchLogind()
}

func (c *coordinator) setSessionState(s session.State) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.sessionID != s.ID {
		c.sessionID = s.ID
		c.triggered = false
	}
	if !c.known {
		// Initial observation must not erase a resume trigger that raced
		// with the first poll.
		c.locked = s.Locked
		c.known = true
		log.Printf("logind LockedHint initial: Sitzung %s gesperrt=%t", s.ID, s.Locked)
		return
	}
	if c.locked != s.Locked {
		c.locked = s.Locked
		c.triggered = false
		if s.Locked {
			log.Printf("logind LockedHint: Sitzung %s ist gesperrt", s.ID)
		} else {
			log.Printf("logind LockedHint: Sitzung %s ist entsperrt", s.ID)
		}
	}
}

func (c *coordinator) watchLockedHint() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	var lastError string
	for {
		s, err := session.Current()
		if err != nil {
			message := err.Error()
			if message != lastError {
				log.Printf("logind-Sperrstatus momentan nicht verfügbar: %v", err)
				lastError = message
			}
		} else {
			lastError = ""
			c.setSessionState(s)
		}
		<-ticker.C
	}
}

func (c *coordinator) watchLogind() {
	for {
		cmd := exec.Command("dbus-monitor", "--system", "type='signal',interface='org.freedesktop.login1.Manager',member='PrepareForSleep'")
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			log.Printf("logind monitor stdout: %v", err)
			time.Sleep(3 * time.Second)
			continue
		}
		if err = cmd.Start(); err != nil {
			log.Printf("logind monitor start: %v", err)
			time.Sleep(3 * time.Second)
			continue
		}
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if strings.TrimSpace(scanner.Text()) == "boolean false" {
				// Give NetworkManager, DNS and GNOME time to restore after resume.
				go func() {
					time.Sleep(2 * time.Second)
					c.triggerResume()
				}()
			}
		}
		_ = cmd.Wait()
		if err := scanner.Err(); err != nil {
			log.Printf("logind monitor read: %v", err)
		}
		log.Println("logind monitor stopped; reconnecting")
		time.Sleep(3 * time.Second)
	}
}

// watchDuoButton listens on the user's session bus for an explicit click on
// the GNOME unlock-dialog button. It never reacts to ordinary lock/unlock.
func (c *coordinator) watchDuoButton() {
	for {
		cmd := exec.Command("dbus-monitor", "--session", "type='signal',path='/org/proxilens/Agent',interface='org.proxilens.Agent',member='DuoUnlockRequested'")
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			log.Printf("Duo-Button-Monitor stdout: %v", err)
			time.Sleep(3 * time.Second)
			continue
		}
		if err = cmd.Start(); err != nil {
			log.Printf("Duo-Button-Monitor Start: %v", err)
			time.Sleep(3 * time.Second)
			continue
		}
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), "member=DuoUnlockRequested") || strings.Contains(scanner.Text(), "member=\"DuoUnlockRequested\"") {
				go c.triggerButtonUnlock()
			}
		}
		_ = cmd.Wait()
		if err := scanner.Err(); err != nil {
			log.Printf("Duo-Button-Monitor read: %v", err)
		}
		log.Println("Duo-Button-Monitor beendet; erneuter Verbindungsversuch")
		time.Sleep(3 * time.Second)
	}
}

func (c *coordinator) triggerButtonUnlock() {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		log.Println("Duo-Button-Klick ignoriert: Authentifizierung läuft bereits")
		return
	}
	c.running = true
	c.mu.Unlock()
	go func() {
		defer func() {
			c.mu.Lock()
			c.running = false
			c.mu.Unlock()
		}()
		log.Println("Duo-Button auf dem GNOME-Sperrbildschirm betätigt")
		if err := authenticateAndUnlock(); err != nil {
			log.Printf("Duo-Entsperrung über Sperrbildschirm-Button fehlgeschlagen: %v", err)
		}
	}()
}

// triggerResume is the Sleep/Wake authentication trigger in this version. A normal
// Super+L lock never starts Duo. authenticateAndUnlock independently verifies
// LockedHint before contacting Authelia and again before unlocking.
func (c *coordinator) triggerResume() {
	c.mu.Lock()
	if c.running || c.triggered {
		c.mu.Unlock()
		return
	}
	c.running = true
	c.triggered = true
	c.mu.Unlock()

	go func() {
		defer func() {
			c.mu.Lock()
			c.running = false
			c.mu.Unlock()
		}()
		log.Println("Sleep/Wake erkannt; prüfe, ob die Sitzung weiterhin gesperrt ist")
		if err := authenticateAndUnlock(); err != nil {
			log.Printf("Duo nach Sleep/Wake nicht abgeschlossen: %v", err)
		}
	}()
}

func authenticateAndUnlock() error {
	// Confirm that this is still the active, locked Wayland user session.
	s, err := session.Current()
	if err != nil {
		return err
	}
	if !s.Locked {
		return fmtError("Sitzung ist nicht gesperrt; Sleep/Wake löst keine Authentifizierung aus")
	}

	dbPath, err := configstore.DefaultPath()
	if err != nil {
		return err
	}
	store, err := configstore.Open(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()
	cfg, err := store.Load()
	if err != nil {
		return err
	}
	if cfg.AutheliaURL == "" || cfg.Username == "" || cfg.Password == "" {
		return fmtError("Authelia URL, Benutzername oder gespeichertes Passwort fehlt; bitte zuerst ProxiLens öffnen und Einstellungen prüfen")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := waitForAutheliaNetwork(ctx, cfg.AutheliaURL); err != nil {
		return err
	}
	if err := authelia.AuthenticateDuo(ctx, cfg.AutheliaURL, cfg.Username, cfg.Password, func(msg string) { log.Printf("Authentifizierung: %s", msg) }); err != nil {
		return err
	}
	// Re-check the exact session state immediately before unlocking.
	current, err := session.Current()
	if err != nil {
		return err
	}
	if current.ID != s.ID || !current.Locked {
		return fmtError("Sitzung hat sich während der Duo-Prüfung geändert; keine Entsperrung")
	}
	return session.UnlockCurrent()
}

type simpleError string

func (e simpleError) Error() string { return string(e) }
func fmtError(s string) error       { return simpleError(s) }

// waitForAutheliaNetwork waits until DNS resolution and a TCP connection to
// the configured Authelia host succeed. Resume may precede network readiness.
func waitForAutheliaNetwork(ctx context.Context, rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil || u.Hostname() == "" {
		return fmtError("ungültige Authelia-URL in den Einstellungen")
	}
	port := u.Port()
	if port == "" {
		switch strings.ToLower(u.Scheme) {
		case "https":
			port = "443"
		case "http":
			port = "80"
		default:
			return fmtError("Authelia-URL muss http oder https verwenden")
		}
	}
	address := net.JoinHostPort(u.Hostname(), port)
	delays := []time.Duration{0, 2 * time.Second, 4 * time.Second, 8 * time.Second, 15 * time.Second, 15 * time.Second, 15 * time.Second}
	var lastErr error
	for attempt, delay := range delays {
		if delay > 0 {
			log.Printf("Netzwerk noch nicht bereit; nächster Versuch in %s", delay)
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return fmtError("Zeitlimit beim Warten auf die Netzwerkverbindung zu Authelia")
			case <-timer.C:
			}
		}
		dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		conn, dialErr := (&net.Dialer{}).DialContext(dialCtx, "tcp", address)
		cancel()
		if dialErr == nil {
			_ = conn.Close()
			log.Printf("Netzwerkverbindung zu Authelia ist bereit (Versuch %d)", attempt+1)
			return nil
		}
		lastErr = dialErr
		log.Printf("Authelia noch nicht erreichbar (Versuch %d/%d): %v", attempt+1, len(delays), dialErr)
	}
	return fmtError("Authelia nicht erreichbar: " + lastErr.Error())
}
