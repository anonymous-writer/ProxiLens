package ui

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/local/proxilens/internal/authelia"
	"github.com/local/proxilens/internal/configstore"
	"github.com/local/proxilens/internal/session"
)

const defaultAutheliaURL = "https://auth.example.com"

func Run() error {
	a := app.NewWithID("de.local.proxilens")
	a.Settings().SetTheme(theme.DarkTheme())
	w := a.NewWindow("ProxiLens · Session & Auth")
	w.Resize(fyne.NewSize(780, 680))

	sessionStatus := widget.NewLabel("Sitzungsstatus wird gelesen …")
	details := widget.NewLabel("")
	authStatus := widget.NewLabel("Authelia: Verbindung wird geprüft …")
	authDetails := widget.NewLabel("Authelia-HTTPS-API wird beim Start automatisch geprüft.")
	deviceFlowStatus := widget.NewLabel("Duo-Authentifizierung: noch nicht gestartet")

	var mu sync.Mutex
	var unlockArmed bool
	unlockArm := widget.NewCheck("Einmaligen Entsperrtest nach erfolgreicher Authelia-Prüfung scharf schalten", func(checked bool) {
		mu.Lock()
		unlockArmed = checked
		mu.Unlock()
	})
	unlockArm.SetChecked(false)
	defaultDB, err := configstore.DefaultPath()
	if err != nil {
		return err
	}
	dbPath := a.Preferences().StringWithFallback("database_path", defaultDB)
	store, err := configstore.Open(dbPath)
	if err != nil {
		return fmt.Errorf("ProxiLens-Datenbank: %w", err)
	}
	defer store.Close()
	cfg, err := store.Load()
	if err != nil {
		return fmt.Errorf("Einstellungen laden: %w", err)
	}
	if cfg.AutheliaURL == "" {
		cfg.AutheliaURL = defaultAutheliaURL
	}
	identityStatus := widget.NewLabel("Erwartete Authelia-Identität: " + cfg.Username + " (noch nicht authentifiziert)")
	urlEntry := widget.NewLabel(cfg.AutheliaURL)
	mode := widget.NewLabel("Sicherheitsmodus: Beobachten – Entsperrtest standardmäßig deaktiviert")

	var flowCancel context.CancelFunc
	var flowRunning bool
	appCtx, appCancel := context.WithCancel(context.Background())

	setFlowRunning := func(running bool) {
		mu.Lock()
		flowRunning = running
		mu.Unlock()
	}
	isFlowRunning := func() bool {
		mu.Lock()
		defer mu.Unlock()
		return flowRunning
	}
	cancelFlow := func() {
		mu.Lock()
		cancel := flowCancel
		flowCancel = nil
		flowRunning = false
		mu.Unlock()
		if cancel != nil {
			cancel()
		}
	}

	refreshSession := widget.NewButton("Sitzungsstatus aktualisieren", func() { updateSession(sessionStatus, details) })
	refreshSession.Importance = widget.HighImportance

	checkAuth := func() {
		base := cfg.AutheliaURL
		authStatus.SetText("Authelia: Verbindung wird geprüft …")
		go func() {
			ctx, cancel := context.WithTimeout(appCtx, 10*time.Second)
			defer cancel()
			result := authelia.Check(ctx, base)
			fyne.Do(func() {
				if result.Connected {
					authStatus.SetText("Authelia: verbunden")
					authDetails.SetText(result.Message)
				} else {
					authStatus.SetText("Authelia: nicht verbunden")
					authDetails.SetText(result.Message)
				}
			})
		}()
	}

	startButton := widget.NewButton("Mit Authelia + Duo Push anmelden", func() {
		if isFlowRunning() {
			return
		}
		if cfg.Password == "" {
			deviceFlowStatus.SetText("Bitte zuerst in den Einstellungen das Authelia-Passwort speichern.")
			return
		}
		ctx, cancel := context.WithCancel(appCtx)
		mu.Lock()
		if flowRunning {
			mu.Unlock()
			cancel()
			return
		}
		flowRunning = true
		flowCancel = cancel
		mu.Unlock()
		deviceFlowStatus.SetText("Authelia-Anmeldung wird gestartet …")
		identityStatus.SetText("Erwartete Identität: " + cfg.Username + " (Prüfung läuft)")
		base, username, password := cfg.AutheliaURL, cfg.Username, cfg.Password
		go func() {
			err := authelia.AuthenticateDuo(ctx, base, username, password, func(message string) { fyne.Do(func() { deviceFlowStatus.SetText(message) }) })
			password = ""
			if err != nil {
				log.Printf("Duo-Authentifizierung fehlgeschlagen: %v", err)
			} else {
				log.Printf("Duo-Authentifizierung und Identitätsprüfung erfolgreich")
			}
			mu.Lock()
			armed := unlockArmed
			unlockArmed = false
			mu.Unlock()
			if err == nil && armed {
				err = session.UnlockCurrent()
				if err != nil {
					err = fmt.Errorf("Identität bestätigt, Entsperrtest aber nicht ausgeführt: %w", err)
				} else {
					log.Printf("Sicherheit: einmaliger Entsperrtest für geprüfte Identität ausgeführt")
				}
			}
			fyne.Do(func() {
				setFlowRunning(false)
				unlockArm.SetChecked(false)
				if err != nil {
					deviceFlowStatus.SetText("Authentifizierung/Entsperrtest fehlgeschlagen: " + err.Error())
					identityStatus.SetText("Identität nicht bestätigt")
				} else {
					deviceFlowStatus.SetText("Duo Push erfolgreich – Identität geprüft")
					identityStatus.SetText("Authentifiziert als " + username)
				}
			})
		}()
	})
	startButton.Importance = widget.HighImportance

	cancelButton := widget.NewButton("Anmeldung abbrechen", func() {
		cancelFlow()
		deviceFlowStatus.SetText("Autorisierung abgebrochen")
		identityStatus.SetText("Identität nicht bestätigt")
	})
	checkButton := widget.NewButton("Verbindung erneut prüfen", checkAuth)
	settingsButton := widget.NewButtonWithIcon("Einstellungen", theme.SettingsIcon(), func() {
		urlField := widget.NewEntry()
		urlField.SetText(cfg.AutheliaURL)
		urlField.SetPlaceHolder("https://auth.example.com")
		userField := widget.NewEntry()
		userField.SetText(cfg.Username)
		passField := widget.NewPasswordEntry()
		passField.SetPlaceHolder("Leer lassen, um gespeichertes Passwort beizubehalten")
		pathField := widget.NewEntry()
		pathField.SetText(dbPath)
		choosePath := widget.NewButton("Datenbankpfad auswählen …", func() {
			dlg := dialog.NewFileSave(func(uri fyne.URIWriteCloser, e error) {
				if e != nil || uri == nil {
					return
				}
				pathField.SetText(uri.URI().Path())
				_ = uri.Close()
			}, w)
			dlg.SetFileName(filepath.Base(pathField.Text))
			dlg.Show()
		})
		form := dialog.NewForm("ProxiLens-Einstellungen", "Speichern", "Abbrechen", []*widget.FormItem{
			widget.NewFormItem("Authelia-Server", urlField), widget.NewFormItem("Authelia-Benutzer", userField),
			widget.NewFormItem("Authelia-Passwort", passField), widget.NewFormItem("SQLite-Datenbank", container.NewVBox(pathField, choosePath)),
		}, func(ok bool) {
			if !ok {
				passField.SetText("")
				return
			}
			newPath := strings.TrimSpace(pathField.Text)
			if newPath == "" {
				dialog.ShowError(fmt.Errorf("Datenbankpfad darf nicht leer sein"), w)
				return
			}
			newStore, e := configstore.Open(newPath)
			if e != nil {
				dialog.ShowError(e, w)
				return
			}
			newCfg := configstore.Settings{AutheliaURL: strings.TrimSpace(urlField.Text), Username: strings.TrimSpace(userField.Text), Password: passField.Text}
			if newCfg.Password == "" && filepath.Clean(newPath) == filepath.Clean(dbPath) {
				newCfg.Password = cfg.Password
			}
			if e = newStore.Save(newCfg); e != nil {
				newStore.Close()
				dialog.ShowError(e, w)
				return
			}
			loaded, e := newStore.Load()
			newStore.Close()
			if e != nil {
				dialog.ShowError(e, w)
				return
			}
			if loaded.Password == "" {
				dialog.ShowError(fmt.Errorf("Passwort konnte nicht gespeichert werden"), w)
				return
			}
			cfg = loaded
			dbPath = newPath
			a.Preferences().SetString("database_path", dbPath)
			urlEntry.SetText(cfg.AutheliaURL)
			authDetails.SetText("Einstellungen gespeichert · Datenbank: " + dbPath)
			checkAuth()
			passField.SetText("")
		}, w)
		form.Resize(fyne.NewSize(560, 340))
		form.Show()
	})

	info := widget.NewLabel("ProxiLens prüft das Authelia-Passwort und fordert anschließend direkt Duo Push an. Die Identität wird an den fest konfigurierten Authelia-Benutzer gebunden. Eine Entsperrung erfolgt nur nach ausdrücklicher einmaliger Freigabe und erfolgreicher Duo-Bestätigung; dafür muss die aktive Wayland-Sitzung bereits gesperrt sein. Das Passwort wird AES-256-GCM-verschlüsselt in SQLite gespeichert; der Schlüssel liegt im GNOME-Schlüsselbund. PAM-, GDM- und GNOME-Einstellungen bleiben unverändert.")
	info.Wrapping = fyne.TextWrapWord

	content := container.NewVBox(
		widget.NewLabelWithStyle("ProxiLens", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Session Monitor · Ubuntu 24.04 / GNOME Wayland"), widget.NewSeparator(),
		widget.NewLabelWithStyle("GNOME-Sitzung", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		sessionStatus, details, refreshSession, widget.NewSeparator(),
		widget.NewLabelWithStyle("Authelia-Verbindung", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Server-Adresse"), urlEntry, authStatus, authDetails, container.NewHBox(checkButton, settingsButton), widget.NewSeparator(),
		widget.NewLabelWithStyle("Direkte Duo-Push-Authentifizierung", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		deviceFlowStatus, container.NewHBox(startButton, cancelButton),
		unlockArm, identityStatus, widget.NewSeparator(), mode, widget.NewSeparator(), info,
	)
	w.SetContent(container.NewPadded(content))

	// During an active device flow, the window close button hides the UI
	// without cancelling authorization. The window is shown again when the
	// flow finishes. Outside an active flow, closing still asks for confirmation.
	w.SetCloseIntercept(func() {
		log.Printf("UI: Schließanforderung erhalten")
		if isFlowRunning() {
			// Keep the window visible while authorization is pending.
			log.Printf("UI: Duo-Anmeldung läuft; Schließen wird ignoriert")
			return
		}
		dialog.NewConfirm(
			"ProxiLens beenden?",
			"Soll ProxiLens wirklich beendet werden?",
			func(confirmed bool) {
				if !confirmed {
					log.Printf("UI: Schließen abgebrochen; Anwendung bleibt geöffnet")
					return
				}
				log.Printf("UI: Beenden bestätigt")
				cancelFlow()
				appCancel()
				w.SetCloseIntercept(nil)
				w.Close()
			},
			w,
		).Show()
	})

	updateSession(sessionStatus, details)
	checkAuth()
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-appCtx.Done():
				return
			case <-ticker.C:
				fyne.Do(func() { updateSession(sessionStatus, details) })
			}
		}
	}()

	log.Printf("ProxiLens: UI wird gestartet")
	w.ShowAndRun()
	appCancel()
	cancelFlow()
	log.Printf("ProxiLens: UI wurde regulär beendet")
	return nil
}

func updateSession(status, details *widget.Label) {
	s, err := session.Current()
	if err != nil {
		status.SetText("Status nicht verfügbar: " + err.Error())
		details.SetText("")
		return
	}
	lock := "nicht gesperrt"
	if s.Locked {
		lock = "gesperrt (LockedHint=yes)"
	}
	active := "inaktiv"
	if s.Active {
		active = "aktiv"
	}
	status.SetText(fmt.Sprintf("Sitzung %s · %s", s.ID, lock))
	details.SetText(fmt.Sprintf("Benutzer: %s\nSitz: %s · Typ: %s · Klasse: %s\nAktiv: %s", s.User, s.Seat, s.Type, s.Class, active))
}
