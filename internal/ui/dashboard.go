package ui

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"image/png"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/local/proxilens/internal/authelia"
	"github.com/local/proxilens/internal/configstore"
	"github.com/local/proxilens/internal/session"
)

//go:embed assets/wlm-logo.png assets/icons/proxilens-128px.png
var logoFS embed.FS

func Run() error {
	a := app.NewWithID("de.local.proxilens")
	a.Settings().SetTheme(theme.DarkTheme())
	w := a.NewWindow("ProxiLens")
	w.Resize(fyne.NewSize(760, 620))
	w.SetMaster()

	defaultDB, err := configstore.DefaultPath()
	if err != nil {
		return err
	}
	dbPath := a.Preferences().StringWithFallback("database_path", defaultDB)
	store, err := configstore.Open(dbPath)
	if err != nil {
		return fmt.Errorf("ProxiLens-Datenbank: %w", err)
	}
	cfg, err := store.Load()
	store.Close()
	if err != nil {
		return fmt.Errorf("Einstellungen laden: %w", err)
	}

	logoBytes, err := logoFS.ReadFile("assets/wlm-logo.png")
	if err != nil {
		return err
	}
	statusIconBytes, err := logoFS.ReadFile("assets/icons/proxilens-128px.png")
	if err != nil {
		return err
	}
	a.SetIcon(fyne.NewStaticResource("proxilens-app-icon.png", statusIconBytes))
	logo, err := png.Decode(bytes.NewReader(logoBytes))
	if err != nil {
		return err
	}
	logoImage := canvas.NewImageFromImage(logo)
	logoImage.SetMinSize(fyne.NewSize(58, 72))
	logoImage.FillMode = canvas.ImageFillContain

	title := canvas.NewText("ProxiLens", theme.Color(theme.ColorNameForeground))
	title.TextSize = 30
	title.TextStyle.Bold = true
	subtitle := widget.NewLabel("Session Monitor · Authelia & Duo")
	header := container.NewHBox(logoImage, container.NewVBox(title, subtitle))

	sessionValue := widget.NewLabel("Wird ermittelt …")
	authValue := widget.NewLabel("Noch nicht geprüft")
	duoValue := widget.NewLabel("Bereit · wartet auf GNOME-Sperrbildschirm")
	versionValue := widget.NewLabel("0.0.2 · Alpha")
	statusCard := widget.NewCard("Systemstatus", "Aktueller Zustand von ProxiLens", container.NewVBox(
		statusLine("GNOME-Sitzung", sessionValue),
		statusLine("Authelia", authValue),
		statusLine("Duo", duoValue),
	))

	urlEntry := widget.NewEntry()
	urlEntry.SetPlaceHolder("https://auth.example.com")
	urlEntry.SetText(cfg.AutheliaURL)
	userEntry := widget.NewEntry()
	userEntry.SetPlaceHolder("Authelia-Benutzername")
	userEntry.SetText(cfg.Username)
	passEntry := widget.NewPasswordEntry()
	passEntry.SetPlaceHolder("Leer lassen, um das gespeicherte Passwort beizubehalten")
	dbEntry := widget.NewEntry()
	dbEntry.SetText(dbPath)

	saveStatus := widget.NewLabel("")
	saveButton := widget.NewButtonWithIcon("Einstellungen speichern", theme.DocumentSaveIcon(), func() {
		newPath := strings.TrimSpace(dbEntry.Text)
		if newPath == "" {
			dialog.ShowError(fmt.Errorf("Datenbankpfad darf nicht leer sein"), w)
			return
		}
		next, e := configstore.Open(newPath)
		if e != nil {
			dialog.ShowError(e, w)
			return
		}
		password := passEntry.Text
		if password == "" && filepath.Clean(newPath) == filepath.Clean(dbPath) {
			password = cfg.Password
		}
		nextCfg := configstore.Settings{AutheliaURL: strings.TrimSpace(urlEntry.Text), Username: strings.TrimSpace(userEntry.Text), Password: password}
		if e = next.Save(nextCfg); e != nil {
			next.Close()
			dialog.ShowError(e, w)
			return
		}
		loaded, e := next.Load()
		next.Close()
		if e != nil {
			dialog.ShowError(e, w)
			return
		}
		if loaded.Password == "" {
			dialog.ShowError(fmt.Errorf("Passwort wurde nicht gespeichert"), w)
			return
		}
		cfg, dbPath = loaded, newPath
		a.Preferences().SetString("database_path", dbPath)
		passEntry.SetText("")
		saveStatus.SetText("Einstellungen sicher gespeichert.")
		checkConnection(cfg.AutheliaURL, authValue)
	})
	settingsCard := widget.NewCard("Verbindungseinstellungen", "Zugangsdaten werden verschlüsselt gespeichert", container.NewVBox(
		field("Authelia-Server", urlEntry), field("Benutzername", userEntry),
		field("Passwort", passEntry), field("Datenbankdatei", dbEntry), saveButton, saveStatus,
	))

	checkButton := widget.NewButtonWithIcon("Verbindung prüfen", theme.ViewRefreshIcon(), func() { checkConnection(cfg.AutheliaURL, authValue) })
	topRight := widget.NewLabel("Ubuntu · GNOME Wayland")
	headerRow := container.NewBorder(nil, nil, nil, topRight, header)
	githubStatus := widget.NewLabel("GitHub-Status wird geladen …")
	repoURL, _ := url.Parse("https://github.com/anonymous-writer/ProxiLens")
	releasesURL, _ := url.Parse("https://github.com/anonymous-writer/ProxiLens/releases")
	actionsURL, _ := url.Parse("https://github.com/anonymous-writer/ProxiLens/actions")
	githubLinks := container.NewHBox(
		widget.NewHyperlink("GitHub", repoURL),
		widget.NewLabel("·"),
		widget.NewHyperlink("Releases", releasesURL),
		widget.NewLabel("·"),
		widget.NewHyperlink("Actions", actionsURL),
	)
	footer := container.NewVBox(
		container.NewHBox(layout.NewSpacer(), widget.NewLabel("ProxiLens"), widget.NewLabel("·"), versionValue, widget.NewLabel("· Status: aktiv")),
		container.NewHBox(layout.NewSpacer(), githubStatus, githubLinks),
	)
	content := container.NewBorder(
		headerRow,
		footer,
		nil, nil,
		container.NewVScroll(container.NewVBox(statusCard, settingsCard, checkButton)),
	)
	w.SetContent(container.NewPadded(content))

	showWindow := func() { w.Show(); w.RequestFocus() }
	if desk, ok := a.(desktop.App); ok {
		desk.SetSystemTrayMenu(fyne.NewMenu("ProxiLens",
			fyne.NewMenuItem("Einstellungen öffnen", showWindow),
			fyne.NewMenuItemSeparator(),
			fyne.NewMenuItem("ProxiLens beenden", func() { a.Quit() }),
		))
	}
	w.SetCloseIntercept(func() { w.Hide() })

	refreshGitHub := func() {
		go func() {
			client := &http.Client{Timeout: 6 * time.Second}
			fetch := func(endpoint string, target any) error {
				req, err := http.NewRequest(http.MethodGet, endpoint, nil)
				if err != nil {
					return err
				}
				req.Header.Set("Accept", "application/vnd.github+json")
				req.Header.Set("User-Agent", "ProxiLens")
				resp, err := client.Do(req)
				if err != nil {
					return err
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					return fmt.Errorf("GitHub HTTP %s", resp.Status)
				}
				return json.NewDecoder(resp.Body).Decode(target)
			}
			var release struct {
				TagName string `json:"tag_name"`
			}
			var runs struct {
				WorkflowRuns []struct {
					UpdatedAt  string `json:"updated_at"`
					Conclusion string `json:"conclusion"`
				} `json:"workflow_runs"`
			}
			var latest, ci string
			if err := fetch("https://api.github.com/repos/anonymous-writer/ProxiLens/releases/latest", &release); err == nil && release.TagName != "" {
				latest = " · Neueste Version: " + release.TagName
			}
			if err := fetch("https://api.github.com/repos/anonymous-writer/ProxiLens/actions/runs?per_page=20", &runs); err == nil {
				for _, run := range runs.WorkflowRuns {
					if run.Conclusion == "success" {
						if parsed, err := time.Parse(time.RFC3339, run.UpdatedAt); err == nil {
							ci = " · Letzter erfolgreicher CI-Lauf: " + parsed.UTC().Format("02.01.2006 15:04") + " UTC"
						}
						break
					}
				}
			}
			message := "GitHub erreichbar" + latest + ci
			if latest == "" && ci == "" {
				message = "GitHub-Status derzeit nicht verfügbar"
			}
			fyne.Do(func() { githubStatus.SetText(message) })
		}()
	}

	refresh := func() {
		s, e := session.Current()
		if e != nil {
			sessionValue.SetText("Nicht verfügbar: " + e.Error())
			return
		}
		lock := "Entsperrt"
		if s.Locked {
			lock = "Gesperrt"
		}
		sessionValue.SetText(fmt.Sprintf("Sitzung %s · %s · %s", s.ID, lock, s.Type))
	}
	refresh()
	refreshGitHub()
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			refreshGitHub()
		}
	}()
	checkConnection(cfg.AutheliaURL, authValue)
	go func() {
		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			fyne.Do(refresh)
		}
	}()
	log.Println("ProxiLens UI gestartet")
	w.ShowAndRun()
	return nil
}

func statusLine(label string, value *widget.Label) fyne.CanvasObject {
	name := widget.NewLabelWithStyle(label, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	return container.NewBorder(nil, nil, name, nil, value)
}
func field(label string, input fyne.CanvasObject) fyne.CanvasObject {
	return container.NewVBox(widget.NewLabelWithStyle(label, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), input)
}
func checkConnection(base string, target *widget.Label) {
	target.SetText("Verbindung wird geprüft …")
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		result := authelia.Check(ctx, base)
		fyne.Do(func() {
			if result.Connected {
				target.SetText("Verbunden · " + result.Message)
			} else {
				target.SetText("Nicht verbunden · " + result.Message)
			}
		})
	}()
}
