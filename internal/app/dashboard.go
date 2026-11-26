package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image/color"
	"image/png"
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/local/proxilens/internal/authflow"
	"github.com/local/proxilens/internal/buildinfo"
	"github.com/local/proxilens/internal/configstore"
	"github.com/local/proxilens/internal/duo"
	"github.com/local/proxilens/internal/grantbroker"
	"github.com/local/proxilens/internal/i18n"
	"github.com/local/proxilens/internal/session"
	staticassets "github.com/local/proxilens/static"
)

func Run() error {
	a := app.NewWithID("de.local.proxilens")
	a.Settings().SetTheme(theme.DarkTheme())
	w := a.NewWindow("ProxiLens")
	lang := a.Preferences().StringWithFallback("language", "de")
	if !i18n.HasLanguage(lang) {
		lang = "de"
	}
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
	if strings.TrimSpace(cfg.DuoPushType) == "" {
		cfg.DuoPushType = "ProxiLens Approval"
	}

	logoBytes, err := staticassets.FS.ReadFile("wlm-logo.png")
	if err != nil {
		return err
	}
	statusIconBytes, err := staticassets.FS.ReadFile("icons/proxilens-128px.png")
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

	// Brand wordmark styled like the LeaseLens parallel project:
	// "Proxi" in the normal foreground and "Lens" in the shared blue accent.
	proxiTitle := canvas.NewText("Proxi", theme.Color(theme.ColorNameForeground))
	lensTitle := canvas.NewText("Lens", color.NRGBA{R: 79, G: 159, B: 248, A: 255})
	for _, part := range []*canvas.Text{proxiTitle, lensTitle} {
		part.TextSize = 30
		part.TextStyle.Bold = true
	}
	title := container.NewHBox(proxiTitle, lensTitle)
	subtitle := widget.NewLabel("Direkte Duo-Authentifizierung")
	header := container.NewHBox(logoImage, container.NewVBox(title, subtitle))
	var applyLanguage func(string)
	var refreshSession func()
	languageLabels := make(map[string]string, len(i18n.Languages()))
	languageCodes := make(map[string]string, len(i18n.Languages()))
	languageOptions := make([]string, 0, len(i18n.Languages()))
	for _, code := range i18n.Languages() {
		label := i18n.LanguageLabel(code)
		languageLabels[code], languageCodes[label] = label, code
		languageOptions = append(languageOptions, label)
	}
	// Build Select directly instead of using widget.NewSelect: Fyne's constructor
	// translates its built-in "(Select one)" placeholder immediately. Fyne 2.8.2
	// has no German catalogue entry for that string, which logs a translation
	// failure even though this selector is preselected below. A localized
	// placeholder avoids the missing built-in translation entirely.
	languageSelect := &widget.Select{
		Options:     languageOptions,
		PlaceHolder: i18n.Translations(lang)["languageSelectPlaceholder"],
		OnChanged: func(choice string) {
			selected := languageCodes[choice]
			if selected != "" {
				lang = selected
				a.Preferences().SetString("language", lang)
				if applyLanguage != nil {
					applyLanguage(lang)
				}
			}
		},
	}
	languageSelect.ExtendBaseWidget(languageSelect)
	languageSelect.SetSelected(languageLabels[lang])

	sessionValue := widget.NewLabel(i18n.Translations(lang)["gnomeChecking"])
	authValue := widget.NewLabel(i18n.Translations(lang)["checking"])
	duoValue := widget.NewLabel(i18n.Translations(lang)["duoReady"])
	sessionName := boldLabel("GNOME session")
	authName := boldLabel("Duo API")
	duoName := boldLabel("Duo")
	statusCard := widget.NewCard("Systemstatus", "Aktueller Zustand von ProxiLens", container.NewVBox(
		container.NewBorder(nil, nil, sessionName, nil, sessionValue),
		container.NewBorder(nil, nil, authName, nil, authValue),
		container.NewBorder(nil, nil, duoName, nil, duoValue),
	))

	duoIKeyEntry, duoSKeyEntry, duoHostEntry, duoUserEntry := widget.NewEntry(), widget.NewPasswordEntry(), widget.NewEntry(), widget.NewEntry()
	duoIKeyEntry.SetText(cfg.DuoIKey)
	duoHostEntry.SetText(cfg.DuoHost)
	duoUserEntry.SetText(cfg.DuoUsername)
	duoSKeyEntry.SetPlaceHolder(i18n.Translations(lang)["duoSecretPlaceholder"])
	duoHostEntry.SetPlaceHolder(i18n.Translations(lang)["duoHostPlaceholder"])
	duoUserEntry.SetPlaceHolder(i18n.Translations(lang)["duoUserPlaceholder"])
	pushTypeEntry := widget.NewEntry()
	pushTypeEntry.SetText(cfg.DuoPushType)
	dbEntry := widget.NewEntry()
	dbEntry.SetText(dbPath)
	databaseName := boldLabel("Database file")
	saveStatus := widget.NewLabel("")
	saveButton := widget.NewButtonWithIcon("Einstellungen speichern", theme.DocumentSaveIcon(), func() {
		newPath := strings.TrimSpace(dbEntry.Text)
		if newPath == "" {
			dialog.ShowError(fmt.Errorf("%s", i18n.Translations(lang)["saveErrorDB"]), w)
			return
		}
		next, e := configstore.Open(newPath)
		if e != nil {
			dialog.ShowError(e, w)
			return
		}
		duoSecret := strings.TrimSpace(duoSKeyEntry.Text)
		if duoSecret == "" && filepath.Clean(newPath) == filepath.Clean(dbPath) {
			duoSecret = cfg.DuoSKey
		}
		pushType := strings.TrimSpace(pushTypeEntry.Text)
		if pushType == "" {
			dialog.ShowError(fmt.Errorf("%s", i18n.Translations(lang)["duoPushTypeRequired"]), w)
			return
		}
		nextCfg := configstore.Settings{
			DuoIKey: strings.TrimSpace(duoIKeyEntry.Text), DuoSKey: duoSecret,
			DuoHost: strings.TrimSpace(duoHostEntry.Text), DuoUsername: strings.TrimSpace(duoUserEntry.Text),
			DuoPushType: pushType,
		}
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
		if loaded.DuoSKey == "" {
			dialog.ShowError(fmt.Errorf("%s", i18n.Translations(lang)["duoSaveKeyError"]), w)
			return
		}
		cfg, dbPath = loaded, newPath
		a.Preferences().SetString("database_path", dbPath)
		duoSKeyEntry.SetText("")
		saveStatus.SetText(i18n.Translations(lang)["saved"])
		authValue.SetText(i18n.Translations(lang)["authConfigured"])
	})
	duoAuthTitle := widget.NewLabelWithStyle(i18n.Translations(lang)["duoAuthTitle"], fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	duoIKeyLabel, duoSKeyLabel := widget.NewLabel(i18n.Translations(lang)["duoIKey"]), widget.NewLabel(i18n.Translations(lang)["duoSKey"])
	duoHostLabel, duoUsernameLabel := widget.NewLabel(i18n.Translations(lang)["duoHost"]), widget.NewLabel(i18n.Translations(lang)["duoUsername"])
	duoKeyHelp := widget.NewLabel(i18n.Translations(lang)["duoKeyHelp"])
	pushTextTitle := widget.NewLabel(i18n.Translations(lang)["duoPushTextTitle"])
	pushTextLabel := widget.NewLabel(i18n.Translations(lang)["duoPushTextLabel"])
	duoFields := container.NewVBox(
		duoAuthTitle,
		duoIKeyLabel, duoIKeyEntry,
		duoSKeyLabel, duoSKeyEntry,
		duoHostLabel, duoHostEntry,
		duoUsernameLabel, duoUserEntry,
		widget.NewSeparator(), pushTextTitle,
		pushTextLabel, pushTypeEntry,
		duoKeyHelp,
		widget.NewSeparator(), container.NewVBox(databaseName, dbEntry), saveButton, saveStatus,
	)
	settingsCard := widget.NewCard("Authentifizierung", "Direkte Duo-Push-Authentifizierung", duoFields)

	// System authentication integration is intentionally disabled in this safe
	// development build. ProxiLens must not modify PAM, Polkit, sudoers or
	// system services while Duo and local grant behavior are being validated.
	safetyNotice := widget.NewCard("Systemauthentifizierung deaktiviert", "Diese Entwicklungsfassung testet Duo unabhängig. sudo, PAM und Polkit werden nicht verändert.", widget.NewLabel("Es wird keine privilegierte Freigabe erteilt. Die lokale Freigabe ist zunächst nur ein Testzustand und ersetzt keine Ubuntu-Authentifizierung."))

	grantStatus := widget.NewLabel("Testfreigabe: inaktiv")
	pushButton := widget.NewButtonWithIcon("Duo Push starten", theme.ConfirmIcon(), nil)
	cancelPushButton := widget.NewButtonWithIcon("Duo-Anfrage abbrechen", theme.CancelIcon(), nil)
	passwordFallbackButton := widget.NewButtonWithIcon("Manuell auf Ubuntu-Passwort wechseln (Backup)", theme.LoginIcon(), nil)
	grantButton := widget.NewButtonWithIcon("15-Minuten-Testfreigabe starten", theme.InfoIcon(), nil)
	revokeButton := widget.NewButtonWithIcon("Testfreigabe widerrufen", theme.DeleteIcon(), nil)
	grantButton.Disable()
	passwordFallbackButton.Disable()
	cancelPushButton.Disable()
	authState := authflow.New()
	revokeButton.Disable()
	var cancelPush context.CancelFunc
	pushButton.OnTapped = func() {
		if cfg.DuoIKey == "" || cfg.DuoSKey == "" || cfg.DuoHost == "" || cfg.DuoUsername == "" {
			dialog.ShowError(fmt.Errorf("Bitte zuerst Duo-Zugangsdaten speichern."), w)
			return
		}
		if err := authState.StartDuo(); err != nil {
			dialog.ShowError(err, w)
			return
		}
		pushButton.Disable()
		passwordFallbackButton.Disable()
		grantButton.Disable()
		cancelPushButton.Enable()
		grantStatus.SetText("Duo Push wird gestartet …")
		pushCfg := duo.Config{IKey: cfg.DuoIKey, SKey: cfg.DuoSKey, Host: cfg.DuoHost, Username: cfg.DuoUsername, PushType: cfg.DuoPushType}
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
		cancelPush = cancel
		go func() {
			err := pushCfg.Push(ctx, func(message string) {
				fyne.Do(func() { grantStatus.SetText(message) })
			})
			cancel()
			fyne.Do(func() {
				cancelPush = nil
				pushButton.Enable()
				cancelPushButton.Disable()
				passwordFallbackButton.Disable()
				if err != nil {
					if errors.Is(err, context.Canceled) {
						_ = authState.CancelDuo()
					} else {
						_ = authState.CompleteDuo(false)
					}
					passwordFallbackButton.Enable() // manual backup choice only; no automatic fallback
					if errors.Is(err, context.Canceled) {
						grantStatus.SetText("Duo-Anfrage abgebrochen. Keine Freigabe erteilt. Du kannst jetzt manuell das Ubuntu-Passwort als Backup auswählen.")
						return
					}
					grantStatus.SetText("Duo nicht verfügbar/fehlgeschlagen. Keine Freigabe erteilt. Passwort-Backup nur nach manueller Auswahl möglich: " + err.Error())
					dialog.ShowError(err, w)
					return
				}
				_ = authState.CompleteDuo(true)
				grantStatus.SetText("Duo bestätigt. Passwort-Backup bleibt deaktiviert. Die lokale Testfreigabe ist weiterhin nur eine Simulation.")
				passwordFallbackButton.Disable()
				grantButton.Enable()
			})
		}()
	}
	cancelPushButton.OnTapped = func() {
		if cancelPush != nil {
			cancelPush()
			cancelPushButton.Disable()
			grantStatus.SetText("Abbruch wird ausgeführt …")
		}
	}
	passwordFallbackButton.OnTapped = func() {
		if err := authState.SelectPassword(); err != nil {
			dialog.ShowError(err, w)
			return
		}
		passwordFallbackButton.Disable()
		dialog.ShowInformation(
			"Manueller Passwort-Fallback ausgewählt – PAM noch nicht integriert",
			"Du hast den Backup-Weg ausdrücklich ausgewählt. In dieser Entwicklungsfassung wird noch keine Passwortauthentifizierung durchgeführt. Es wird kein Passwort abgefragt, gespeichert oder an Duo übertragen. Die echte lokale Authentifizierung wird erst mit dem separat geprüften PAM-Adapter aktiviert.",
			w,
		)
	}
	grantButton.OnTapped = func() {
		grantButton.Disable()
		go func() {
			status, err := grantbroker.StartTestGrant(15 * time.Minute)
			fyne.Do(func() {
				if err != nil {
					grantStatus.SetText("Grant Broker nicht erreichbar: " + err.Error())
					grantButton.Enable()
					return
				}
				grantStatus.SetText("SIMULATION aktiv bis " + status.ExpiresAt.Format("15:04:05") + " – keine Systemberechtigung")
				revokeButton.Enable()
			})
		}()
	}
	revokeButton.OnTapped = func() {
		revokeButton.Disable()
		go func() {
			_, err := grantbroker.RevokeTestGrant()
			fyne.Do(func() {
				if err != nil {
					grantStatus.SetText("Widerruf fehlgeschlagen: " + err.Error())
					revokeButton.Enable()
					return
				}
				grantStatus.SetText("Testfreigabe widerrufen – keine Systemberechtigung")
				grantButton.Disable()
			})
		}()
	}
	// The GUI reads status from the independently running user broker.
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			status, err := grantbroker.QueryStatus()
			fyne.Do(func() {
				if err != nil {
					grantStatus.SetText("Grant Broker nicht erreichbar – Testdienst starten")
					return
				}
				if status.Active {
					grantStatus.SetText("SIMULATION aktiv bis " + status.ExpiresAt.Format("15:04:05") + " – keine Systemberechtigung")
					revokeButton.Enable()
				} else if strings.Contains(grantStatus.Text, "SIMULATION aktiv") {
					grantStatus.SetText("Testfreigabe abgelaufen – keine Systemberechtigung")
					grantButton.Enable()
					revokeButton.Disable()
				}
			})
		}
	}()

	checkButton := widget.NewButtonWithIcon("Verbindung prüfen", theme.ViewRefreshIcon(), func() {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			err := (duo.Config{IKey: cfg.DuoIKey, SKey: cfg.DuoSKey, Host: cfg.DuoHost, Username: cfg.DuoUsername}).Check(ctx)
			fyne.Do(func() {
				if err != nil {
					authValue.SetText(i18n.Translations(lang)["duoCheckFailed"] + err.Error())
					dialog.ShowError(err, w)
				} else {
					authValue.SetText(i18n.Translations(lang)["duoCheckOK"])
					dialog.ShowInformation(i18n.Translations(lang)["duoCheckDialogTitle"], i18n.Translations(lang)["duoCheckDialogOK"], w)
				}
			})
		}()
	})

	// Installationseinrichtung: Die beiden Komponenten laufen unabhängig von diesem Fenster.
	gnomeInstallStatus := widget.NewLabel(i18n.Translations(lang)["gnomeChecking"])
	serviceInstallStatus := widget.NewLabel(i18n.Translations(lang)["serviceChecking"])
	gnomeInstallButton := widget.NewButtonWithIcon("GNOME-Erweiterung (später)", theme.DownloadIcon(), nil)
	serviceInstallButton := widget.NewButtonWithIcon("Hintergrunddienst (später)", theme.SettingsIcon(), nil)
	var refreshInstallStatus func()
	gnomeInstallButton.Disable()
	serviceInstallButton.Disable()
	gnomeInstallStatus.SetText("In dieser sicheren Entwicklungsfassung nicht installiert.")
	serviceInstallStatus.SetText("In dieser sicheren Entwicklungsfassung nicht installiert.")
	refreshInstallStatus = func() {
		gnomeInstallButton.Disable()
		serviceInstallButton.Disable()
	}
	setupCard := widget.NewCard("Einrichtung", "Einmalige Installation der GNOME-Erweiterung und des Hintergrunddienstes", container.NewVBox(
		gnomeInstallButton, gnomeInstallStatus,
		widget.NewSeparator(), serviceInstallButton, serviceInstallStatus,
	))
	topRight := widget.NewLabel("Ubuntu · GNOME Wayland")
	headerRight := container.NewHBox(languageSelect, topRight)
	headerRow := container.NewBorder(nil, nil, nil, headerRight, header)
	githubStatus := widget.NewLabel(i18n.Translations(lang)["githubLoading"])
	var refreshGitHub func()
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
	// Keep the footer focused on useful GitHub information; the redundant
	// product/version/status row has been removed.
	footer := container.NewVBox(
		container.NewHBox(layout.NewSpacer(), githubStatus, githubLinks),
	)

	applyLanguage = func(language string) {
		t := i18n.Translations(language)
		subtitle.SetText(t["subtitle"])
		duoValue.SetText(t["duoReady"])
		statusCard.Title, statusCard.Subtitle = t["statusTitle"], t["statusSubtitle"]
		settingsCard.Title, settingsCard.Subtitle = t["settingsTitle"], t["settingsSubtitle"]
		setupCard.Title, setupCard.Subtitle = t["setupTitle"], t["setupSubtitle"]
		saveButton.SetText(t["save"])
		checkButton.SetText(t["check"])
		gnomeInstallButton.SetText(t["gnomeInstall"])
		serviceInstallButton.SetText(t["serviceInstall"])
		topRight.SetText(t["topRight"])
		databaseName.SetText(t["database"])
		sessionName.SetText(t["session"])
		authName.SetText("Duo API")
		duoName.SetText(t["duo"])
		duoAuthTitle.SetText(t["duoAuthTitle"])
		duoIKeyLabel.SetText(t["duoIKey"])
		duoSKeyLabel.SetText(t["duoSKey"])
		duoHostLabel.SetText(t["duoHost"])
		duoUsernameLabel.SetText(t["duoUsername"])
		duoKeyHelp.SetText(t["duoKeyHelp"])
		pushTextTitle.SetText(t["duoPushTextTitle"])
		pushTextLabel.SetText(t["duoPushTextLabel"])
		duoSKeyEntry.SetPlaceHolder(t["duoSecretPlaceholder"])
		duoHostEntry.SetPlaceHolder(t["duoHostPlaceholder"])
		duoUserEntry.SetPlaceHolder(t["duoUserPlaceholder"])
		if strings.TrimSpace(saveStatus.Text) != "" {
			saveStatus.SetText(t["saved"])
		}
		// Re-read all live states so status labels are regenerated in the
		// newly selected language instead of retaining old localized strings.
		if refreshSession != nil {
			refreshSession()
		}
		if refreshInstallStatus != nil {
			refreshInstallStatus()
		}
		if refreshGitHub != nil {
			githubStatus.SetText(t["githubLoading"])
			refreshGitHub()
		}
		if cfg.DuoIKey != "" && cfg.DuoHost != "" && cfg.DuoUsername != "" {
			authValue.SetText(i18n.Translations(lang)["authConfigured"])
		} else {
			authValue.SetText(i18n.Translations(lang)["authNotConfigured"])
		}
		// Card title/subtitle are plain fields, not data-bound labels. Refresh the
		// widgets themselves after changing them so their renderers pick up the
		// new language immediately (canvas.Refresh alone is not sufficient).
		statusCard.Refresh()
		settingsCard.Refresh()
		setupCard.Refresh()
		languageSelect.PlaceHolder = t["languageSelectPlaceholder"]
		languageSelect.Refresh()
		for _, obj := range []fyne.CanvasObject{
			statusCard, settingsCard, safetyNotice, setupCard, headerRow, footer,
			subtitle, languageSelect, topRight, sessionValue, authValue, duoValue,
			sessionName, authName, duoName,
			databaseName, saveButton, saveStatus, checkButton, pushButton, cancelPushButton, passwordFallbackButton, grantButton, revokeButton, grantStatus, pushTextTitle, pushTextLabel, pushTypeEntry,
			gnomeInstallButton, gnomeInstallStatus, serviceInstallButton, serviceInstallStatus,
			githubStatus,
		} {
			canvas.Refresh(obj)
		}
	}

	applyLanguage(lang)
	content := container.NewBorder(
		headerRow,
		footer,
		nil, nil,
		container.NewVScroll(container.NewVBox(statusCard, setupCard, settingsCard, safetyNotice, checkButton, pushButton, cancelPushButton, passwordFallbackButton, grantStatus, container.NewHBox(grantButton, revokeButton))),
	)
	w.SetContent(container.NewPadded(content))

	refreshGitHub = func() {
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
				latest = release.TagName
			}
			if err := fetch("https://api.github.com/repos/anonymous-writer/ProxiLens/actions/runs?per_page=20", &runs); err == nil {
				for _, run := range runs.WorkflowRuns {
					if run.Conclusion == "success" {
						if parsed, err := time.Parse(time.RFC3339, run.UpdatedAt); err == nil {
							ci = parsed.UTC().Format("02.01.2006 15:04") + " UTC"
						}
						break
					}
				}
			}
			fyne.Do(func() {
				t := i18n.Translations(lang)
				message := t["githubReachable"] + t["installedBuild"] + buildinfo.DisplayVersion()
				if latest != "" {
					message += t["githubLatest"] + latest
				}
				if ci != "" {
					message += t["githubCI"] + ci
				}
				if latest == "" && ci == "" {
					message = t["githubUnavailable"] + t["installedBuild"] + buildinfo.DisplayVersion()
				}
				githubStatus.SetText(message)
			})
		}()
	}

	refreshSession = func() {
		s, e := session.Current()
		if e != nil {
			sessionValue.SetText(i18n.Translations(lang)["sessionUnavailable"] + ": " + e.Error())
			return
		}
		lock := i18n.Translations(lang)["unlocked"]
		if s.Locked {
			lock = i18n.Translations(lang)["locked"]
		}
		sessionValue.SetText(fmt.Sprintf(i18n.Translations(lang)["sessionFormat"], s.ID, lock, s.Type))
	}
	refreshSession()
	refreshGitHub()
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			refreshGitHub()
		}
	}()
	if cfg.DuoIKey != "" && cfg.DuoHost != "" && cfg.DuoUsername != "" {
		authValue.SetText(i18n.Translations(lang)["authConfigured"])
	} else {
		authValue.SetText(i18n.Translations(lang)["authNotConfigured"])
	}
	go func() {
		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			fyne.Do(refreshSession)
		}
	}()
	log.Println("ProxiLens UI gestartet")
	w.ShowAndRun()
	return nil
}

func boldLabel(text string) *widget.Label {
	return widget.NewLabelWithStyle(text, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
}

func statusLine(label string, value *widget.Label) fyne.CanvasObject {
	name := widget.NewLabelWithStyle(label, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	return container.NewBorder(nil, nil, name, nil, value)
}
func field(label string, input fyne.CanvasObject) fyne.CanvasObject {
	return container.NewVBox(widget.NewLabelWithStyle(label, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), input)
}
func commandOutput(name string, args ...string) string {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out)
	}
	return string(out)
}
