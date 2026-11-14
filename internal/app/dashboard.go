package app

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image/color"
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

	"github.com/local/proxilens/internal/buildinfo"
	"github.com/local/proxilens/internal/configstore"
	"github.com/local/proxilens/internal/duo"
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
	gnomeInstallButton := widget.NewButtonWithIcon("GNOME-Erweiterung installieren", theme.DownloadIcon(), nil)
	serviceInstallButton := widget.NewButtonWithIcon("Hintergrunddienst installieren", theme.SettingsIcon(), nil)
	var refreshInstallStatus func()
	gnomeInstallButton.OnTapped = func() {
		if _, err := os.Stat("/usr/share/gnome-shell/extensions/proxilens-input@local/metadata.json"); err == nil {
			gnomeInstallButton.Disable()
			go func() {
				cmd := exec.Command("gnome-extensions", "enable", "proxilens-input@local")
				out, enableErr := cmd.CombinedOutput()
				fyne.Do(func() {
					if enableErr != nil {
						dialog.ShowError(fmt.Errorf("GNOME-Erweiterung konnte nicht aktiviert werden: %v\\n%s", enableErr, strings.TrimSpace(string(out))), w)
					}
					refreshInstallStatus()
				})
			}()
			return
		}
		runGitHubInstaller(w, "install-gnome-extension.sh", gnomeInstallStatus, gnomeInstallButton, refreshInstallStatus, func() string { return lang })
	}
	serviceInstallButton.OnTapped = func() {
		if _, err := os.Stat("/usr/lib/systemd/user/proxilens-wake-agent.service"); err == nil {
			serviceInstallButton.Disable()
			go func() {
				cmd := exec.Command("systemctl", "--user", "enable", "--now", "proxilens-wake-agent.service")
				out, startErr := cmd.CombinedOutput()
				fyne.Do(func() {
					if startErr != nil {
						serviceInstallStatus.SetText(i18n.Translations(lang)["serviceInactive"])
						dialog.ShowError(fmt.Errorf("Wake-Agent konnte nicht aktiviert werden: %v\\n%s", startErr, strings.TrimSpace(string(out))), w)
					} else {
						serviceInstallStatus.SetText(i18n.Translations(lang)["serviceActive"])
					}
					refreshInstallStatus()
				})
			}()
			return
		}
		runGitHubInstaller(w, "install-wake-agent.sh", serviceInstallStatus, serviceInstallButton, refreshInstallStatus, func() string { return lang })
	}
	refreshInstallStatus = func() {
		go func() {
			const extensionUUID = "proxilens-input@local"
			extensionDirs := []string{
				filepath.Join("/usr/share/gnome-shell/extensions", extensionUUID),
				filepath.Join(os.Getenv("HOME"), ".local", "share", "gnome-shell", "extensions", extensionUUID),
			}
			gnomeInstalled := false
			for _, extensionDir := range extensionDirs {
				if _, metadataErr := os.Stat(filepath.Join(extensionDir, "metadata.json")); metadataErr == nil {
					gnomeInstalled = true
					break
				}
			}
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
					gnomeInstallStatus.SetText(i18n.Translations(lang)["gnomeEnabled"])
					gnomeInstallButton.Disable()
				} else if gnomeInstalled {
					gnomeInstallStatus.SetText(i18n.Translations(lang)["gnomeInstalledDisabled"])
					gnomeInstallButton.SetText(i18n.Translations(lang)["gnomeEnable"])
					gnomeInstallButton.Enable()
				} else {
					gnomeInstallStatus.SetText(i18n.Translations(lang)["notInstalled"])
					gnomeInstallButton.SetText(i18n.Translations(lang)["gnomeInstall"])
					gnomeInstallButton.Enable()
				}
				if serviceInstalled {
					serviceInstallStatus.SetText(i18n.Translations(lang)["serviceActive"])
					serviceInstallButton.Disable()
				} else {
					serviceInstallStatus.SetText(i18n.Translations(lang)["serviceInactive"])
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
			statusCard, settingsCard, setupCard, headerRow, footer,
			subtitle, languageSelect, topRight, sessionValue, authValue, duoValue,
			sessionName, authName, duoName,
			databaseName, saveButton, saveStatus, checkButton, pushTextTitle, pushTextLabel, pushTypeEntry,
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
		container.NewVScroll(container.NewVBox(statusCard, setupCard, settingsCard, checkButton)),
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

// runGitHubInstaller downloads the repository's current default branch and runs
// the selected, version-controlled installer script. No local source checkout is
// required; the running wake-agent remains independent of this UI process.
func runGitHubInstaller(w fyne.Window, script string, status *widget.Label, button *widget.Button, refresh func(), language func() string) {
	status.SetText(i18n.Translations(language())["installDownloading"])
	button.Disable()

	go func() {
		defer func() {
			fyne.Do(func() { button.Enable() })
		}()

		root, cleanup, err := downloadGitHubSource()
		if err != nil {
			fyne.Do(func() {
				status.SetText(i18n.Translations(language())["installDownloadFailed"])
				dialog.ShowError(err, w)
			})
			return
		}
		defer cleanup()

		scriptPath, pathErr := findInstallerScript(root, script)
		if pathErr != nil {
			fyne.Do(func() {
				status.SetText(i18n.Translations(language())["installScriptMissing"])
				dialog.ShowError(pathErr, w)
			})
			return
		}

		fyne.Do(func() { status.SetText(i18n.Translations(language())["installRunning"]) })
		cmd := exec.Command("bash", scriptPath)
		cmd.Dir = root
		out, runErr := cmd.CombinedOutput()

		fyne.Do(func() {
			if runErr != nil {
				status.SetText(i18n.Translations(language())["installFailed"])
				dialog.ShowError(fmt.Errorf("%s\n%s", runErr, strings.TrimSpace(string(out))), w)
				return
			}
			status.SetText(i18n.Translations(language())["installSuccess"])
			refresh()
			if strings.TrimSpace(string(out)) != "" {
				dialog.ShowInformation(i18n.Translations(language())["installDialog"], strings.TrimSpace(string(out)), w)
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

	projectRoot, err := findInstallerProjectRoot(extractRoot)
	if err != nil {
		cleanup()
		return "", func() {}, err
	}
	return projectRoot, cleanup, nil
}

// findInstallerProjectRoot locates the project regardless of whether GitHub's
// archive has one wrapper directory, an additional nested project directory,
// or the older deploy/ layout.
func findInstallerProjectRoot(extractRoot string) (string, error) {
	var candidates []string
	err := filepath.WalkDir(extractRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(extractRoot, path)
		if err != nil {
			return err
		}
		if strings.Count(rel, string(os.PathSeparator)) > 5 {
			return filepath.SkipDir
		}
		scriptsDir := filepath.Join(path, "scripts")
		deployDir := filepath.Join(path, "deploy")
		hasScripts := fileExists(filepath.Join(scriptsDir, "install-gnome-extension.sh")) &&
			fileExists(filepath.Join(scriptsDir, "install-wake-agent.sh"))
		hasDeploy := fileExists(filepath.Join(deployDir, "install-gnome-extension.sh")) &&
			fileExists(filepath.Join(deployDir, "install-wake-agent.sh"))
		if hasScripts || hasDeploy {
			candidates = append(candidates, path)
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("GitHub-Archiv konnte nicht nach Installationsskripten durchsucht werden: %w", err)
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("Im GitHub-Quellarchiv wurden die beiden Installationsskripte nicht gefunden. Erwartet werden scripts/install-gnome-extension.sh und scripts/install-wake-agent.sh (alternativ deploy/...). Bitte prüfe, ob die aktuelle Dateistruktur bereits in den GitHub-Standard-Branch hochgeladen wurde.")
	}
	if len(candidates) > 1 {
		return "", fmt.Errorf("Im GitHub-Quellarchiv wurden mehrere mögliche ProxiLens-Projektverzeichnisse gefunden; die Installation wurde aus Sicherheitsgründen abgebrochen.")
	}
	return candidates[0], nil
}

func findInstallerScript(projectRoot, script string) (string, error) {
	if filepath.Base(script) != script || (script != "install-gnome-extension.sh" && script != "install-wake-agent.sh") {
		return "", fmt.Errorf("Unzulässiger Name des Installationsskripts: %s", script)
	}
	for _, dir := range []string{"scripts", "deploy"} {
		candidate := filepath.Join(projectRoot, dir, script)
		if fileExists(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("Installationsskript %s wurde weder unter scripts/ noch unter deploy/ gefunden.", script)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
