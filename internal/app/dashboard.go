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
	var refreshIntegrationStatus func()
	syncRootButton := widget.NewButton("Duo-Konfiguration für sudo/Polkit übernehmen", func() {
		if _, err := exec.LookPath("/usr/bin/proxilens-duo-sync"); err != nil {
			dialog.ShowError(fmt.Errorf("Duo-Synchronisierung ist nicht installiert. Bitte ProxiLens-DEB aktualisieren."), w)
			return
		}
		dialog.ShowConfirm("Systemauthentifizierung", "Die gespeicherten Duo-Zugangsdaten werden geschützt an den Root-Policy-Dienst übertragen. Fortfahren?", func(ok bool) {
			if !ok {
				return
			}
			go func() {
				cmd := exec.Command("/usr/bin/proxilens-duo-sync", "--db", dbPath)
				out, err := cmd.CombinedOutput()
				fyne.Do(func() {
					if err != nil {
						msg := strings.TrimSpace(string(out))
						if msg == "" {
							msg = err.Error()
						}
						dialog.ShowError(fmt.Errorf("Duo-Synchronisierung: %s", msg), w)
					} else {
						dialog.ShowInformation("ProxiLens", strings.TrimSpace(string(out)), w)
					}
					if refreshIntegrationStatus != nil {
						refreshIntegrationStatus()
					}
				})
			}()
		}, w)
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
		widget.NewSeparator(), container.NewVBox(databaseName, dbEntry), saveButton, syncRootButton, saveStatus,
	)
	settingsCard := widget.NewCard("Authentifizierung", "Direkte Duo-Push-Authentifizierung", duoFields)

	// System authentication is provided by the installed PAM module and policy broker.
	// This screen reports installation state only; it does not trigger authentication tests.
	sudoIntegrationValue := widget.NewLabel("Status wird geprüft …")
	polkitIntegrationValue := widget.NewLabel("Status wird geprüft …")
	policyServiceValue := widget.NewLabel("Status wird geprüft …")
	integrationCard := widget.NewCard(i18n.Translations(lang)["integrationTitle"], i18n.Translations(lang)["integrationSubtitle"], container.NewVBox(
		statusLine("sudo (PAM)", sudoIntegrationValue),
		statusLine("Polkit (PAM)", polkitIntegrationValue),
		statusLine("ProxiLens Policy-Dienst", policyServiceValue),
		widget.NewLabel(i18n.Translations(lang)["integrationPolicy"]),
	))

	integrationActions := container.NewGridWithColumns(2)
	runIntegrationScript := func(script, actionLabel string, refresh func()) {
		scriptPath, findErr := findIntegrationScript(script)
		if findErr != nil {
			dialog.ShowError(findErr, w)
			return
		}
		dialog.ShowConfirm(
			"Systemintegration",
			actionLabel+"? Dafür werden Administratorrechte benötigt. Die sudo- und Polkit-Integrationen werden getrennt verwaltet.",
			func(confirmed bool) {
				if !confirmed {
					return
				}
				go func() {
					cmd := exec.Command("pkexec", "/bin/bash", scriptPath)
					output, runErr := cmd.CombinedOutput()
					fyne.Do(func() {
						if runErr != nil {
							msg := strings.TrimSpace(string(output))
							if msg == "" {
								msg = runErr.Error()
							}
							dialog.ShowError(fmt.Errorf("%s: %s", actionLabel, msg), w)
						} else {
							msg := strings.TrimSpace(string(output))
							if msg == "" {
								msg = actionLabel + " abgeschlossen."
							}
							dialog.ShowInformation("ProxiLens", msg, w)
						}
						refresh()
					})
				}()
			},
			w,
		)
	}
	sudoInstallButton := widget.NewButton("sudo installieren", func() {
		runIntegrationScript("install-sudo-pam-proxilens.sh", "sudo-PAM-Integration installieren", refreshIntegrationStatus)
	})
	sudoRemoveButton := widget.NewButton("sudo deinstallieren", func() {
		runIntegrationScript("remove-sudo-pam-proxilens.sh", "sudo-PAM-Integration entfernen", refreshIntegrationStatus)
	})
	polkitInstallButton := widget.NewButton("Polkit installieren", func() {
		runIntegrationScript("install-polkit-pam-proxilens.sh", "Polkit-PAM-Integration installieren", refreshIntegrationStatus)
	})
	polkitRemoveButton := widget.NewButton("Polkit deinstallieren", func() {
		runIntegrationScript("remove-polkit-pam-proxilens.sh", "Polkit-PAM-Integration entfernen", refreshIntegrationStatus)
	})
	integrationActions.Add(sudoInstallButton)
	integrationActions.Add(sudoRemoveButton)
	integrationActions.Add(polkitInstallButton)
	integrationActions.Add(polkitRemoveButton)
	integrationCard.SetContent(container.NewVBox(
		statusLine("sudo (PAM)", sudoIntegrationValue),
		statusLine("Polkit (PAM)", polkitIntegrationValue),
		statusLine("ProxiLens Policy-Dienst", policyServiceValue),
		widget.NewLabel(i18n.Translations(lang)["integrationPolicy"]),
		widget.NewSeparator(),
		integrationActions,
	))

	refreshIntegrationStatus = func() {
		readable := func(path string) bool {
			b, err := os.ReadFile(path)
			return err == nil && strings.Contains(string(b), "pam_proxilens.so")
		}
		serviceOut, serviceErr := exec.Command("systemctl", "is-active", "proxilens-policy.service").CombinedOutput()
		serviceActive := serviceErr == nil && strings.TrimSpace(string(serviceOut)) == "active"
		sudoInstalled := readable("/etc/pam.d/sudo")
		polkitInstalled := readable("/etc/pam.d/polkit-1")
		if sudoInstalled {
			sudoIntegrationValue.SetText(i18n.Translations(lang)["integrationEnabled"])
			sudoInstallButton.Disable()
			sudoRemoveButton.Enable()
		} else {
			sudoIntegrationValue.SetText(i18n.Translations(lang)["integrationUnknown"])
			sudoInstallButton.Enable()
			sudoRemoveButton.Disable()
		}
		if polkitInstalled {
			polkitIntegrationValue.SetText(i18n.Translations(lang)["integrationEnabled"])
			polkitInstallButton.Disable()
			polkitRemoveButton.Enable()
		} else {
			polkitIntegrationValue.SetText(i18n.Translations(lang)["integrationUnknown"])
			polkitInstallButton.Enable()
			polkitRemoveButton.Disable()
		}
		if serviceActive {
			policyServiceValue.SetText(i18n.Translations(lang)["policyServiceActive"])
		} else {
			policyServiceValue.SetText(i18n.Translations(lang)["policyServiceInactive"])
		}
	}
	refreshIntegrationStatus()

	integrationCard.SetContent(container.NewVBox(
		statusLine("sudo (PAM)", sudoIntegrationValue),
		statusLine("Polkit (PAM)", polkitIntegrationValue),
		statusLine("ProxiLens Policy-Dienst", policyServiceValue),
		widget.NewLabel(i18n.Translations(lang)["integrationPolicy"]),
		widget.NewSeparator(),
		integrationActions,
	))

	// GNOME unlock integration: user-session extension and independent wake agent.
	gnomeInstallStatus := widget.NewLabel(i18n.Translations(lang)["gnomeChecking"])
	wakeInstallStatus := widget.NewLabel(i18n.Translations(lang)["serviceChecking"])
	gnomeInstallButton := widget.NewButton(i18n.Translations(lang)["gnomeInstall"], nil)
	gnomeRemoveButton := widget.NewButton("GNOME-Erweiterung entfernen", nil)
	wakeInstallButton := widget.NewButton("Wake-Agent installieren", nil)
	wakeRemoveButton := widget.NewButton("Wake-Agent entfernen", nil)

	runUserScript := func(script, title string, refresh func()) {
		scriptPath, err := findIntegrationScript(script)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		dialog.ShowConfirm(title, title+"? Der Vorgang wird für deinen Benutzer ausgeführt.", func(ok bool) {
			if !ok {
				return
			}
			go func() {
				cmd := exec.Command("/bin/bash", scriptPath)
				output, runErr := cmd.CombinedOutput()
				fyne.Do(func() {
					if runErr != nil {
						msg := strings.TrimSpace(string(output))
						if msg == "" {
							msg = runErr.Error()
						}
						dialog.ShowError(fmt.Errorf("%s: %s", title, msg), w)
					} else {
						msg := strings.TrimSpace(string(output))
						if msg == "" {
							msg = title + " abgeschlossen."
						}
						dialog.ShowInformation("ProxiLens", msg, w)
					}
					refresh()
				})
			}()
		}, w)
	}

	var refreshWakeStatus func()
	gnomeInstallButton.OnTapped = func() {
		// If installed but disabled, enable it without reinstalling.
		installed := false
		for _, dir := range []string{
			filepath.Join("/usr/share/gnome-shell/extensions", "proxilens-input@local"),
			filepath.Join(os.Getenv("HOME"), ".local/share/gnome-shell/extensions", "proxilens-input@local"),
		} {
			if _, err := os.Stat(filepath.Join(dir, "metadata.json")); err == nil {
				installed = true
			}
		}
		if installed {
			go func() {
				cmd := exec.Command("gnome-extensions", "enable", "proxilens-input@local")
				out, err := cmd.CombinedOutput()
				fyne.Do(func() {
					if err != nil {
						dialog.ShowError(fmt.Errorf("GNOME-Erweiterung konnte nicht aktiviert werden: %v: %s", err, strings.TrimSpace(string(out))), w)
					}
					refreshWakeStatus()
				})
			}()
			return
		}
		runUserScript("install-gnome-extension.sh", "GNOME-Erweiterung installieren", refreshWakeStatus)
	}
	gnomeRemoveButton.OnTapped = func() {
		runUserScript("uninstall-gnome-extension.sh", "GNOME-Erweiterung entfernen", refreshWakeStatus)
	}
	wakeInstallButton.OnTapped = func() {
		runUserScript("install-wake-agent.sh", "Wake-Agent installieren", refreshWakeStatus)
	}
	wakeRemoveButton.OnTapped = func() {
		runUserScript("uninstall-wake-agent.sh", "Wake-Agent entfernen", refreshWakeStatus)
	}
	refreshWakeStatus = func() {
		go func() {
			const uuid = "proxilens-input@local"
			extensionInstalled := false
			for _, dir := range []string{
				filepath.Join("/usr/share/gnome-shell/extensions", uuid),
				filepath.Join(os.Getenv("HOME"), ".local/share/gnome-shell/extensions", uuid),
			} {
				if _, err := os.Stat(filepath.Join(dir, "metadata.json")); err == nil {
					extensionInstalled = true
				}
			}
			enabled := false
			out, _ := exec.Command("gnome-extensions", "list", "--enabled").CombinedOutput()
			for _, id := range strings.Split(string(out), "\n") {
				if strings.TrimSpace(id) == uuid {
					enabled = true
				}
			}
			serviceOut, serviceErr := exec.Command("systemctl", "--user", "is-active", "proxilens-wake-agent.service").CombinedOutput()
			serviceActive := serviceErr == nil && strings.TrimSpace(string(serviceOut)) == "active"
			fyne.Do(func() {
				t := i18n.Translations(lang)
				if extensionInstalled && enabled {
					gnomeInstallStatus.SetText(t["gnomeEnabled"])
					gnomeInstallButton.SetText(t["gnomeEnabled"])
					gnomeInstallButton.Disable()
					gnomeRemoveButton.Enable()
				} else if extensionInstalled {
					gnomeInstallStatus.SetText(t["gnomeInstalledDisabled"])
					gnomeInstallButton.SetText(t["gnomeEnable"])
					gnomeInstallButton.Enable()
					gnomeRemoveButton.Enable()
				} else {
					gnomeInstallStatus.SetText(t["notInstalled"])
					gnomeInstallButton.SetText(t["gnomeInstall"])
					gnomeInstallButton.Enable()
					gnomeRemoveButton.Disable()
				}
				if serviceActive {
					wakeInstallStatus.SetText(t["serviceActive"])
					wakeInstallButton.Disable()
					wakeRemoveButton.Enable()
				} else {
					wakeInstallStatus.SetText(t["serviceInactive"])
					wakeInstallButton.Enable()
					wakeRemoveButton.Enable() // permits cleanup of a partially installed agent
				}
			})
		}()
	}
	refreshWakeStatus()
	wakeCard := widget.NewCard(
		"GNOME-Sperrbildschirm & Energiesparmodus",
		"GNOME-Duo-Schaltfläche und Hintergrunddienst zum Wiederaufnehmen nach Standby/Ruhezustand",
		container.NewVBox(
			statusLine("GNOME-Erweiterung", gnomeInstallStatus),
			container.NewGridWithColumns(2, gnomeInstallButton, gnomeRemoveButton),
			widget.NewSeparator(),
			statusLine("ProxiLens Wake-Agent", wakeInstallStatus),
			container.NewGridWithColumns(2, wakeInstallButton, wakeRemoveButton),
		),
	)

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
		integrationCard.Title, integrationCard.Subtitle = t["integrationTitle"], t["integrationSubtitle"]
		sudoInstallButton.SetText(t["integrationInstallSudo"])
		sudoRemoveButton.SetText(t["integrationRemoveSudo"])
		polkitInstallButton.SetText(t["integrationInstallPolkit"])
		polkitRemoveButton.SetText(t["integrationRemovePolkit"])
		saveButton.SetText(t["save"])
		checkButton.SetText(t["check"])
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
		refreshIntegrationStatus()
		if refreshWakeStatus != nil {
			refreshWakeStatus()
		}
		gnomeInstallButton.SetText(t["gnomeInstall"])
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
		integrationCard.Refresh()
		languageSelect.PlaceHolder = t["languageSelectPlaceholder"]
		languageSelect.Refresh()
		for _, obj := range []fyne.CanvasObject{
			statusCard, settingsCard, integrationCard, wakeCard, headerRow, footer,
			subtitle, languageSelect, topRight, sessionValue, authValue, duoValue,
			sessionName, authName, duoName,
			databaseName, saveButton, saveStatus, checkButton, sudoIntegrationValue, polkitIntegrationValue, policyServiceValue,
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
		container.NewVScroll(container.NewVBox(statusCard, integrationCard, wakeCard, settingsCard, checkButton)),
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
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			fyne.Do(refreshIntegrationStatus)
			fyne.Do(refreshWakeStatus)
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

// findIntegrationScript locates the administrator script in a source checkout.
// The GUI never constructs privileged shell commands from user input.
func findIntegrationScript(name string) (string, error) {
	if filepath.Base(name) != name || !strings.HasSuffix(name, ".sh") {
		return "", errors.New("Ungültiger Integrationsskriptname")
	}
	home, _ := os.UserHomeDir()
	if home == "" {
		home = os.Getenv("HOME")
	}
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" && home != "" {
		dataHome = filepath.Join(home, ".local", "share")
	}
	candidates := []string{
		filepath.Join(dataHome, "proxilens", "source", "scripts", name),
		filepath.Join(dataHome, "proxilens", "scripts", name),
		filepath.Join(home, ".local", "share", "proxilens", "scripts", name),
		filepath.Join(home, ".local", "share", "proxilens", "source", "scripts", name),
		filepath.Join("/usr", "libexec", "proxilens", "scripts", name),
		filepath.Join("/usr", "local", "share", "proxilens", "scripts", name),
		filepath.Join("/usr", "share", "proxilens", "scripts", name),
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(wd, "scripts", name))
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for i := 0; i < 5; i++ {
			candidates = append(candidates, filepath.Join(dir, "scripts", name))
			dir = filepath.Dir(dir)
		}
	}
	for _, candidate := range candidates {
		info, err := os.Lstat(candidate)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		return candidate, nil
	}
	return "", fmt.Errorf("Integrationsskript %q wurde nicht gefunden. Geprüfte Orte: %s. Bitte zuerst die Integrationsskripte bereitstellen.", name, strings.Join(candidates, ", "))
}
