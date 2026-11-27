// proxilens-duo-sync copies the already configured Duo credentials from the
// user's encrypted ProxiLens store to the root policy service via pkexec.
// The secret is sent only over stdin, never argv or logs.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"

	"github.com/local/proxilens/internal/configstore"
)

type payload struct {
	IKey     string `json:"ikey"`
	SKey     string `json:"skey"`
	Host     string `json:"host"`
	Username string `json:"username"`
	PushType string `json:"push_type"`
}

func main() {
	db := flag.String("db", "", "Pfad zur ProxiLens-Datenbank (optional)")
	flag.Parse()
	path := *db
	if path == "" {
		var err error
		path, err = configstore.DefaultPath()
		if err != nil {
			fatal(err)
		}
	}
	store, err := configstore.Open(path)
	if err != nil {
		fatal(err)
	}
	cfg, err := store.Load()
	_ = store.Close()
	if err != nil {
		fatal(err)
	}
	if cfg.DuoIKey == "" || cfg.DuoSKey == "" || cfg.DuoHost == "" || cfg.DuoUsername == "" {
		fatal(fmt.Errorf("Duo-Einstellungen in ProxiLens sind unvollständig"))
	}
	data, err := json.Marshal(payload{cfg.DuoIKey, cfg.DuoSKey, cfg.DuoHost, cfg.DuoUsername, cfg.DuoPushType})
	if err != nil {
		fatal(err)
	}
	cmd := exec.Command("pkexec", "/usr/libexec/proxilens/proxilens-duo-import")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		fatal(err)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err = cmd.Start(); err != nil {
		fatal(err)
	}
	if _, err = stdin.Write(data); err != nil {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		fatal(err)
	}
	if err = stdin.Close(); err != nil {
		fatal(err)
	}
	if err = cmd.Wait(); err != nil {
		fatal(fmt.Errorf("Root-Synchronisierung abgebrochen: %w", err))
	}
	for i := range data {
		data[i] = 0
	}
	fmt.Println("Duo-Konfiguration aus ProxiLens wurde sicher mit dem Root-Dienst synchronisiert.")
}

func fatal(err error) { fmt.Fprintln(os.Stderr, "ProxiLens Duo-Synchronisierung:", err); os.Exit(1) }
