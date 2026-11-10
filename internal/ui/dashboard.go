package ui

import (
	"archive/zip"
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
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
	lang := a.Preferences().StringWithFallback("language", "de")
	if lang != "de" && lang != "en" && lang != "es" {
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
	var applyLanguage func(string)
	languageSelect := widget.NewSelect([]string{"🇩🇪 Deutsch", "🇬🇧 English", "🇪🇸 Español"}, func(choice string) {
		selected := map[string]string{"🇩🇪 Deutsch": "de", "🇬🇧 English": "en", "🇪🇸 Español": "es"}[choice]
		if selected != "" {
			lang = selected
			a.Preferences().SetString("language", lang)
			if applyLanguage != nil {
				applyLanguage(lang)
			}
		}
	})
	languageSelect.SetSelected(map[string]string{"de": "🇩🇪 Deutsch", "en": "🇬🇧 English", "es": "🇪🇸 Español"}[lang])

	sessionValue := widget.NewLabel("Wird ermittelt …")
	authValue := widget.NewLabel("Noch nicht geprüft")
	duoValue := widget.NewLabel("Bereit · wartet auf GNOME-Sperrbildschirm")
	versionValue := widget.NewLabel("v1.0.1 · Beta")
	sessionName := boldLabel("GNOME session")
	authName := boldLabel("Authelia")
	duoName := boldLabel("Duo")
	statusCard := widget.NewCard("Systemstatus", "Aktueller Zustand von ProxiLens", container.NewVBox(
		container.NewBorder(nil, nil, sessionName, nil, sessionValue),
		container.NewBorder(nil, nil, authName, nil, authValue),
		container.NewBorder(nil, nil, duoName, nil, duoValue),
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

	serverName := boldLabel("Authelia server")
	usernameName := boldLabel("Username")
	passwordName := boldLabel("Password")
	databaseName := boldLabel("Database file")
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
		container.NewVBox(serverName, urlEntry), container.NewVBox(usernameName, userEntry),
		container.NewVBox(passwordName, passEntry), container.NewVBox(databaseName, dbEntry), saveButton, saveStatus,
	))

	checkButton := widget.NewButtonWithIcon("Verbindung prüfen", theme.ViewRefreshIcon(), func() { checkConnection(cfg.AutheliaURL, authValue) })

	// Installationseinrichtung: Die beiden Komponenten laufen unabhängig von diesem Fenster.
	gnomeInstallStatus := widget.NewLabel("Status wird geprüft …")
	serviceInstallStatus := widget.NewLabel("Status wird geprüft …")
	gnomeInstallButton := widget.NewButtonWithIcon("GNOME-Erweiterung installieren", theme.DownloadIcon(), nil)
	serviceInstallButton := widget.NewButtonWithIcon("Hintergrunddienst installieren", theme.SettingsIcon(), nil)
	var refreshInstallStatus func()
	gnomeInstallButton.OnTapped = func() {
		runGitHubInstaller(w, "install-gnome-extension.sh", gnomeInstallStatus, gnomeInstallButton, refreshInstallStatus)
	}
	serviceInstallButton.OnTapped = func() {
		runGitHubInstaller(w, "install-wake-agent.sh", serviceInstallStatus, serviceInstallButton, refreshInstallStatus)
	}
	refreshInstallStatus = func() {
		go func() {
			const extensionUUID = "proxilens-input@local"
			extensionDir := filepath.Join(os.Getenv("HOME"), ".local", "share", "gnome-shell", "extensions", extensionUUID)
			_, metadataErr := os.Stat(filepath.Join(extensionDir, "metadata.json"))
			gnomeInstalled := metadataErr == nil
			enabledOutput := commandOutput("gnome-extensions", "list", "--enabled")
			gnomeEnabled := false
			for _, id := range strings.Split(enabledOutput, "\n") {
				if strings.TrimSpace(id) == extensionUUID {
					gnomeEnabled = true
					break
				}
			}
			serviceState := commandOutput("systemctl", "--user", "is-active", "proxilens-wake-agent.service")
			serviceInstalled := strings.TrimSpace(serviceState) == "active"
			fyne.Do(func() {
				if gnomeInstalled && gnomeEnabled {
					gnomeInstallStatus.SetText(translations(lang)["gnomeEnabled"])
					gnomeInstallButton.Disable()
				} else if gnomeInstalled {
					gnomeInstallStatus.SetText(translations(lang)["gnomeInstalledDisabled"])
					gnomeInstallButton.SetText(translations(lang)["gnomeEnable"])
					gnomeInstallButton.Enable()
				} else {
					gnomeInstallStatus.SetText(translations(lang)["notInstalled"])
					gnomeInstallButton.SetText(translations(lang)["gnomeInstall"])
					gnomeInstallButton.Enable()
				}
				if serviceInstalled {
					serviceInstallStatus.SetText(translations(lang)["serviceActive"])
					serviceInstallButton.Disable()
				} else {
					serviceInstallStatus.SetText(translations(lang)["serviceInactive"])
					serviceInstallButton.Enable()
				}
			})
		}()
	}
	refreshInstallStatus()
	setupCard := widget.NewCard("Einrichtung", "Einmalige Installation der GNOME-Erweiterung und des Hintergrunddienstes", container.NewVBox(
		gnomeInstallButton, gnomeInstallStatus,
		widget.NewSeparator(), serviceInstallButton, serviceInstallStatus,
	))
	topRight := widget.NewLabel("Ubuntu · GNOME Wayland")
	headerRight := container.NewHBox(languageSelect, topRight)
	headerRow := container.NewBorder(nil, nil, nil, headerRight, header)
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
	refreshInstallLabels := func(language string) {
		t := translations(language)
		// Current status labels are refreshed by refreshInstallStatus; this updates
		// their text immediately when the user changes language.
		if strings.Contains(gnomeInstallStatus.Text, "Installiert") || strings.Contains(gnomeInstallStatus.Text, "Installed") || strings.Contains(gnomeInstallStatus.Text, "Instalada") {
			gnomeInstallStatus.SetText(t["gnomeEnabled"])
		} else if strings.Contains(gnomeInstallStatus.Text, "Nicht installiert") || strings.Contains(gnomeInstallStatus.Text, "Not installed") || strings.Contains(gnomeInstallStatus.Text, "No instalada") {
			gnomeInstallStatus.SetText(t["notInstalled"])
		} else if strings.Contains(gnomeInstallStatus.Text, "nicht aktiviert") || strings.Contains(gnomeInstallStatus.Text, "not enabled") || strings.Contains(gnomeInstallStatus.Text, "no activada") {
			gnomeInstallStatus.SetText(t["gnomeInstalledDisabled"])
		}
		if strings.Contains(serviceInstallStatus.Text, "Aktiv") || strings.Contains(serviceInstallStatus.Text, "Active") {
			serviceInstallStatus.SetText(t["serviceActive"])
		} else if strings.Contains(serviceInstallStatus.Text, "Nicht") || strings.Contains(serviceInstallStatus.Text, "Not") {
			serviceInstallStatus.SetText(t["notInstalled"])
		}
	}
	applyLanguage = func(language string) {
		t := translations(language)
		subtitle.SetText(t["subtitle"])
		statusCard.Title, statusCard.Subtitle = t["statusTitle"], t["statusSubtitle"]
		settingsCard.Title, settingsCard.Subtitle = t["settingsTitle"], t["settingsSubtitle"]
		setupCard.Title, setupCard.Subtitle = t["setupTitle"], t["setupSubtitle"]
		saveButton.SetText(t["save"])
		checkButton.SetText(t["check"])
		gnomeInstallButton.SetText(t["gnomeInstall"])
		serviceInstallButton.SetText(t["serviceInstall"])
		topRight.SetText(t["topRight"])
		serverName.SetText(t["server"])
		usernameName.SetText(t["username"])
		passwordName.SetText(t["password"])
		databaseName.SetText(t["database"])
		sessionName.SetText(t["session"])
		authName.SetText(t["authelia"])
		duoName.SetText(t["duo"])
		urlEntry.SetPlaceHolder(t["urlPlaceholder"])
		userEntry.SetPlaceHolder(t["usernamePlaceholder"])
		passEntry.SetPlaceHolder(t["passwordPlaceholder"])
		if strings.TrimSpace(saveStatus.Text) != "" {
			saveStatus.SetText(t["saved"])
		}
		canvas.Refresh(statusCard)
		canvas.Refresh(settingsCard)
		canvas.Refresh(setupCard)
		canvas.Refresh(headerRow)
		refreshInstallLabels(language)
	}

	applyLanguage(lang)
	content := container.NewBorder(
		headerRow,
		footer,
		nil, nil,
		container.NewVScroll(container.NewVBox(statusCard, setupCard, settingsCard, checkButton)),
	)
	w.SetContent(container.NewPadded(content))

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

func translations(language string) map[string]string {
	all := map[string]map[string]string{
		"de": {"subtitle": "Monitor für Sitzungen · Authelia & Duo", "statusTitle": "Systemstatus", "statusSubtitle": "Aktueller Zustand von ProxiLens", "settingsTitle": "Verbindungseinstellungen", "settingsSubtitle": "Zugangsdaten werden verschlüsselt gespeichert", "save": "Konfiguration speichern", "check": "Verbindung prüfen", "setupTitle": "Einrichtung", "setupSubtitle": "Einmalige Installation der GNOME-Erweiterung und des Hintergrunddienstes", "gnomeInstall": "GNOME-Erweiterung installieren", "serviceInstall": "Hintergrunddienst installieren", "server": "Authelia-Server", "username": "Benutzername", "password": "Passwort", "database": "Datenbankdatei", "session": "GNOME-Sitzung", "authelia": "Authelia", "duo": "Duo", "topRight": "Ubuntu · GNOME Wayland", "urlPlaceholder": "https://auth.example.com", "usernamePlaceholder": "Authelia-Benutzername", "passwordPlaceholder": "Leer lassen, um das gespeicherte Passwort beizubehalten", "saved": "Einstellungen sicher gespeichert.", "gnomeEnabled": "Installiert und aktiviert", "notInstalled": "Nicht installiert", "serviceActive": "Aktiv · läuft unabhängig vom Einstellungsfenster", "serviceInactive": "Nicht installiert oder nicht aktiv", "gnomeInstalledDisabled": "Installiert, aber nicht aktiviert", "gnomeEnable": "GNOME-Erweiterung aktivieren"},
		"en": {"subtitle": "Session monitor · Authelia & Duo", "statusTitle": "System status", "statusSubtitle": "Current ProxiLens status", "settingsTitle": "Connection settings", "settingsSubtitle": "Credentials are stored encrypted", "save": "Save configuration", "check": "Check connection", "setupTitle": "Setup", "setupSubtitle": "One-time installation of the GNOME extension and background service", "gnomeInstall": "Install GNOME extension", "serviceInstall": "Install background service", "server": "Authelia server", "username": "Username", "password": "Password", "database": "Database file", "session": "GNOME session", "authelia": "Authelia", "duo": "Duo", "topRight": "Ubuntu · GNOME Wayland", "urlPlaceholder": "https://auth.example.com", "usernamePlaceholder": "Authelia username", "passwordPlaceholder": "Leave empty to keep the saved password", "saved": "Settings saved securely.", "gnomeEnabled": "Installed and enabled", "notInstalled": "Not installed", "serviceActive": "Active · runs independently of the settings window", "serviceInactive": "Not installed or inactive", "gnomeInstalledDisabled": "Installed, but not enabled", "gnomeEnable": "Enable GNOME extension"},
		"es": {"subtitle": "Monitor de sesión · Authelia y Duo", "statusTitle": "Estado del sistema", "statusSubtitle": "Estado actual de ProxiLens", "settingsTitle": "Configuración de conexión", "settingsSubtitle": "Las credenciales se guardan cifradas", "save": "Guardar configuración", "check": "Comprobar conexión", "setupTitle": "Instalación", "setupSubtitle": "Instalación única de la extensión GNOME y del servicio en segundo plano", "gnomeInstall": "Instalar extensión GNOME", "serviceInstall": "Instalar servicio en segundo plano", "server": "Servidor Authelia", "username": "Nombre de usuario", "password": "Contraseña", "database": "Archivo de base de datos", "session": "Sesión GNOME", "authelia": "Authelia", "duo": "Duo", "topRight": "Ubuntu · GNOME Wayland", "urlPlaceholder": "https://auth.example.com", "usernamePlaceholder": "Usuario de Authelia", "passwordPlaceholder": "Déjelo vacío para conservar la contraseña guardada", "saved": "Configuración guardada de forma segura.", "gnomeEnabled": "Instalada y activada", "notInstalled": "No instalada", "serviceActive": "Activa · funciona independientemente de la ventana de ajustes", "serviceInactive": "No instalada o inactiva", "gnomeInstalledDisabled": "Instalada, pero no activada", "gnomeEnable": "Activar extensión GNOME"},
	}
	if t, ok := all[language]; ok {
		return t
	}
	return all["de"]
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

func commandOutput(name string, args ...string) string {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out)
	}
	return string(out)
}

// runGitHubInstaller downloads the repository's current default branch and runs
// the selected, version-controlled installer script. No local source checkout is
// required; the running wake-agent remains independent of this UI process.
func runGitHubInstaller(w fyne.Window, script string, status *widget.Label, button *widget.Button, refresh func()) {
	status.SetText("Aktueller ProxiLens-Code wird von GitHub geladen …")
	button.Disable()

	go func() {
		defer func() {
			fyne.Do(func() { button.Enable() })
		}()

		root, cleanup, err := downloadGitHubSource()
		if err != nil {
			fyne.Do(func() {
				status.SetText("GitHub-Download fehlgeschlagen")
				dialog.ShowError(err, w)
			})
			return
		}
		defer cleanup()

		scriptPath := filepath.Join(root, "deploy", script)
		if _, err := os.Stat(scriptPath); err != nil {
			fyne.Do(func() {
				status.SetText("Installationsdatei fehlt im GitHub-Projekt")
				dialog.ShowError(fmt.Errorf("GitHub-Quellarchiv enthält deploy/%s nicht: %w", script, err), w)
			})
			return
		}

		fyne.Do(func() { status.SetText("Installation läuft …") })
		cmd := exec.Command("bash", scriptPath)
		cmd.Dir = root
		out, runErr := cmd.CombinedOutput()

		fyne.Do(func() {
			if runErr != nil {
				status.SetText("Installation fehlgeschlagen")
				dialog.ShowError(fmt.Errorf("%s\n%s", runErr, strings.TrimSpace(string(out))), w)
				return
			}
			status.SetText("Installation erfolgreich · Quelle: GitHub")
			refresh()
			if strings.TrimSpace(string(out)) != "" {
				dialog.ShowInformation("ProxiLens-Installation", strings.TrimSpace(string(out)), w)
			}
		})
	}()
}

// downloadGitHubSource asks GitHub for the repository's default branch, downloads
// its ZIP archive, and extracts it into a private temporary directory.
func downloadGitHubSource() (string, func(), error) {
	const repoAPI = "https://api.github.com/repos/anonymous-writer/ProxiLens"

	tempDir, err := os.MkdirTemp("", "proxilens-github-install-*")
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(tempDir) }

	client := &http.Client{Timeout: 2 * time.Minute}
	request := func(endpoint string) (*http.Response, error) {
		req, err := http.NewRequest(http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		req.Header.Set("User-Agent", "ProxiLens-Desktop-Installer")
		return client.Do(req)
	}

	metaResp, err := request(repoAPI)
	if err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("GitHub-Repositoryinformationen konnten nicht geladen werden: %w", err)
	}
	defer metaResp.Body.Close()
	if metaResp.StatusCode != http.StatusOK {
		cleanup()
		return "", func() {}, fmt.Errorf("GitHub-Repositoryabfrage fehlgeschlagen: HTTP %s", metaResp.Status)
	}
	var metadata struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.NewDecoder(metaResp.Body).Decode(&metadata); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("GitHub-Antwort konnte nicht gelesen werden: %w", err)
	}
	if metadata.DefaultBranch == "" || strings.Contains(metadata.DefaultBranch, "..") {
		cleanup()
		return "", func() {}, fmt.Errorf("GitHub hat keinen gültigen Standard-Branch geliefert")
	}

	archiveURL := repoAPI + "/zipball/" + url.PathEscape(metadata.DefaultBranch)
	archiveResp, err := request(archiveURL)
	if err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("ProxiLens-Quellarchiv konnte nicht geladen werden: %w", err)
	}
	defer archiveResp.Body.Close()
	if archiveResp.StatusCode != http.StatusOK {
		cleanup()
		return "", func() {}, fmt.Errorf("Download des GitHub-Quellarchivs fehlgeschlagen: HTTP %s", archiveResp.Status)
	}

	archivePath := filepath.Join(tempDir, "source.zip")
	archiveFile, err := os.OpenFile(archivePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		cleanup()
		return "", func() {}, err
	}
	_, copyErr := io.Copy(archiveFile, io.LimitReader(archiveResp.Body, 100<<20))
	closeErr := archiveFile.Close()
	if copyErr != nil || closeErr != nil {
		cleanup()
		if copyErr != nil {
			return "", func() {}, fmt.Errorf("GitHub-Archiv konnte nicht gespeichert werden: %w", copyErr)
		}
		return "", func() {}, closeErr
	}

	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("GitHub-Archiv ist ungültig: %w", err)
	}
	defer zr.Close()

	extractRoot := filepath.Join(tempDir, "repo")
	if err := os.MkdirAll(extractRoot, 0700); err != nil {
		cleanup()
		return "", func() {}, err
	}

	var total int64
	for _, file := range zr.File {
		cleanName := filepath.Clean(filepath.FromSlash(file.Name))
		parts := strings.Split(cleanName, string(os.PathSeparator))
		if filepath.IsAbs(cleanName) || cleanName == "." || cleanName == ".." ||
			(len(parts) > 0 && parts[0] == "..") {
			cleanup()
			return "", func() {}, fmt.Errorf("Unsicherer Pfad im GitHub-Archiv: %s", file.Name)
		}
		// GitHub ZIP archives contain one top-level directory.
		if len(parts) < 2 {
			continue
		}
		relative := filepath.Join(parts[1:]...)
		if relative == "." || relative == "" {
			continue
		}
		target := filepath.Join(extractRoot, relative)
		if !strings.HasPrefix(target, extractRoot+string(os.PathSeparator)) {
			cleanup()
			return "", func() {}, fmt.Errorf("Ungültiger Archivpfad: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0700); err != nil {
				cleanup()
				return "", func() {}, err
			}
			continue
		}
		total += int64(file.UncompressedSize64)
		if total > 200<<20 {
			cleanup()
			return "", func() {}, fmt.Errorf("GitHub-Quellarchiv überschreitet die erlaubte Entpackgröße")
		}
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			cleanup()
			return "", func() {}, err
		}
		in, err := file.Open()
		if err != nil {
			cleanup()
			return "", func() {}, err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			in.Close()
			cleanup()
			return "", func() {}, err
		}
		_, copyErr := io.Copy(out, io.LimitReader(in, int64(file.UncompressedSize64)+1))
		in.Close()
		closeErr := out.Close()
		if copyErr != nil || closeErr != nil {
			cleanup()
			if copyErr != nil {
				return "", func() {}, copyErr
			}
			return "", func() {}, closeErr
		}
		if file.Mode()&0111 != 0 {
			_ = os.Chmod(target, 0700)
		}
	}

	if _, err := os.Stat(filepath.Join(extractRoot, "deploy", "install-gnome-extension.sh")); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("Im GitHub-Quellarchiv fehlt das GNOME-Installationsskript")
	}
	if _, err := os.Stat(filepath.Join(extractRoot, "deploy", "install-wake-agent.sh")); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("Im GitHub-Quellarchiv fehlt das Wake-Agent-Installationsskript")
	}
	return extractRoot, cleanup, nil
}
