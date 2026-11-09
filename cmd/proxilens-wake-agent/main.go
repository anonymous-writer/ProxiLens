// proxilens-wake-agent is an opt-in experimental logind resume watcher.
// It only starts Duo when the current Wayland session is already locked.
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

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)
	log.Println("ProxiLens experimental wake agent started; waiting for logind resume")
	var mu sync.Mutex
	running := false
	for {
		cmd := exec.Command("dbus-monitor", "--system", "type='signal',interface='org.freedesktop.login1.Manager',member='PrepareForSleep'")
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			log.Printf("dbus-monitor stdout: %v", err)
			time.Sleep(3 * time.Second)
			continue
		}
		if err = cmd.Start(); err != nil {
			log.Printf("dbus-monitor start: %v", err)
			time.Sleep(3 * time.Second)
			continue
		}
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			// logind emits PrepareForSleep(false) after resume.
			if line == "boolean false" {
				mu.Lock()
				if !running {
					running = true
					go func() {
						defer func() { mu.Lock(); running = false; mu.Unlock() }()
						if err := onResume(); err != nil {
							log.Printf("resume authentication not completed: %v", err)
						}
					}()
				}
				mu.Unlock()
			}
		}
		_ = cmd.Wait()
		if err := scanner.Err(); err != nil {
			log.Printf("dbus-monitor read: %v", err)
		}
		log.Println("logind monitor stopped; reconnecting")
		time.Sleep(3 * time.Second)
	}
}

func onResume() error {
	// Allow logind/GNOME to restore the session state before inspecting it.
	time.Sleep(2 * time.Second)
	s, err := session.Current()
	if err != nil {
		return err
	}
	if !s.Locked {
		return nil
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
// the configured Authelia host succeed. Resume can be delivered before Wi-Fi,
// routing, or the local DNS stub is ready.
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
			log.Printf("Netzwerk nach Resume noch nicht bereit; nächster Versuch in %s", delay)
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
	return fmtError("Authelia nach Resume nicht erreichbar: " + lastErr.Error())
}
