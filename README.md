# ProxiLens

**An experimental GNOME lock-screen integration with direct Duo authentication,
with an optional Sleep/Wake user agent.**

ProxiLens is a prototype for Ubuntu systems running GNOME Shell 50.x in
a Wayland user session. It adds an explicit **Unlock with Duo** action
to the GNOME lock screen and uses an independent user-level agent to
handle the authentication request and session-unlock workflow.

> \[!WARNING\] **Alpha / experimental software.** The workflow has been
> tested on the project's target system (Ubuntu, GNOME Shell 50.1,
> Wayland). It is not a general security guarantee and has not been
> validated across all GNOME, Ubuntu, or hardware versions. Review the
> security limitations below before using it.

<p align="center">
  <img
    src="https://raw.githubusercontent.com/anonymous-writer/ProxiLens/main/docs/ProxiLens-Projekt.png"
    alt="ProxiLens project overview"
    width="72%">
</p>

## Screenshots

<p align="center">
  <img
    src="https://raw.githubusercontent.com/anonymous-writer/ProxiLens/main/docs/screenshot1.png"
    alt="ProxiLens desktop application"
    width="100%">
</p>

<p align="center">
  <img
    src="https://raw.githubusercontent.com/anonymous-writer/ProxiLens/main/docs/AppleWatch.png"
    alt="ProxiLens Apple Watch screenshot"
    width="30%">
  &nbsp;
  <img
    src="https://raw.githubusercontent.com/anonymous-writer/ProxiLens/main/docs/iPhone.png"
    alt="ProxiLens iPhone screenshot"
    width="30%">
</p>

<p align="center">
  <em>ProxiLens desktop application, Apple Watch and iPhone views</em>
</p>

## Overview

### GNOME lock screen and Duo authentication

-   The GNOME Shell extension adds an **Unlock with Duo** button to the
    `unlock-dialog`.
-   The user must explicitly click the button to start the Duo workflow.
-   The extension sends a `DuoUnlockRequested` signal to the ProxiLens
    user agent over the session D-Bus.
-   The agent loads the Duo API settings and encrypted credentials from the
    ProxiLens configuration store.
-   The agent requests authentication directly through the Duo Auth API.
-   After successful Duo confirmation, the agent verifies that the same
    active Wayland session is still locked.
-   Only then does it request `loginctl unlock-session` for that
    session.
-   On the tested system, the desktop unlocked after successful Duo
    confirmation without requiring the Ubuntu password to be entered
    again.

The agent also observes GNOME session-mode transitions (`unlock-dialog`
and `user`) and the logind `LockedHint` property.

### Duo status reporting

The agent reports the workflow state to the GNOME extension:

1.  `LOCKED`
2.  `DUO_PENDING`
3.  `DUO_SUCCESS`
4.  `UNLOCK_REQUESTED`
5.  `UNLOCKED` --- after GNOME reports that the session has left the
    unlock dialog
6.  `FAILED` --- if authentication or the unlock request fails

In Alpha.7.3.1, state notifications are centrally deduplicated. If a
GNOME mode transition and the authentication workflow report the same
state at nearly the same time, the state is logged and sent to the
extension only once.

### Authentication latency

When the user clicks the lock-screen button, the agent starts the HTTPS
authentication request immediately. It does not run a separate DNS/TCP
preflight first; the HTTPS request itself establishes the connection.

The Sleep/Wake path retains its network-readiness retry loop because DNS
and network services may still be starting after resume. Password
validation, Duo enrollment verification, Duo Push confirmation, and the
final locked-session checks are unchanged.

## Sleep/Wake handling

-   The user agent monitors logind's `PrepareForSleep` signal.
-   After resume, network availability is checked again because DNS and
    network services may need time to become available.
-   The Duo flow after resume proceeds only if the session is still
    locked.
-   Sleep/Wake handling is independent of manually clicking the Duo
    button.

## Desktop settings application

ProxiLens includes a Fyne-based desktop application for configuring the
Duo API connection and viewing GNOME session, Duo,
installation, and GitHub status.

The settings window is launched as a normal desktop application.

### Direct Duo authentication

The settings UI configures the Duo Auth API directly. The Integration Key,
Secret Key, API Hostname, and Duo username are stored in the local SQLite
credential store; secrets are encrypted with a key held in the GNOME keyring.
The application has a single authentication path: direct Duo Push.

 It is
not required to remain open for the background agent to operate. The
`proxilens-wake-agent.service` systemd user service runs independently
of the settings window.

The application supports three interface languages:

-   German
-   English
-   Spanish

The selected language is saved in the application preferences.

## Project structure

The repository follows the shared organizational layout used across the
project family, with ProxiLens-specific components retained where needed:

- `main.go` starts the desktop UI; `cmd/proxilens-wake-agent/` contains the independent background agent.
- `internal/app/` contains the desktop application.
- `internal/duo/`, `internal/configstore/`, and `internal/session/` contain the domain logic.
- `internal/i18n/` loads all interface translations from one embedded JSON catalog.
- `static/` contains the application logo and icon artwork.
- `gnome-extension/` contains the GNOME Shell extension.
- `scripts/` contains installation scripts and the systemd user service.
- `docs/` contains project documentation and screenshots.
- `.github/` contains issue templates and CI/release workflows.

## Adding or changing interface languages

All desktop UI strings are maintained in `internal/i18n/translations.json`.
Each top-level property is a language code (`de`, `en`, `es`, etc.). To add
a language, add a new object with the same translation keys, including
`languageLabel` (the name shown in the language selector). The selector is
populated automatically from this catalog; no UI code change is required.
The JSON catalog is embedded into the executable at build time.

## Linux architectures

The project supports these Linux architectures when the corresponding
CI/release workflows are enabled:

-   `linux/amd64` --- Intel and AMD 64-bit systems
-   `linux/arm64` --- 64-bit ARM systems, including compatible Raspberry
    Pi models

Published GitHub releases may provide a separate archive and SHA-256
checksum for each architecture. Check the **Releases** page for the
assets available for a particular release.

## Requirements

-   Ubuntu with GNOME Shell 50.x. The GNOME extension installer checks
    for GNOME 50.x.
-   A Wayland user session.
-   `systemd-logind`, `loginctl`, `dbus-monitor`, and `gdbus`.
-   Go **1.27 or newer**, as specified by `go.mod`, when building from
    source or installing the agent from source.
-   A Duo Auth API integration with its IKEY, SKEY, API Hostname, and username.
-   A reachable Duo Auth API integration and an enrolled Duo device.
-   For the desktop UI build, the Linux development libraries required
    by Fyne (see the CI workflow for the Ubuntu package list).

## Installation and updates

### Option A: Install from the desktop application

Open ProxiLens and use the two setup actions:

1.  **Install GNOME extension**
2.  **Install background service**

The application downloads the current source archive from this
repository's default branch and runs the corresponding installer script.
A local Git checkout is not required for these setup actions.

The Wake Agent installer requires the Go version specified in `go.mod`
and the required system tools, including `dbus-monitor`.

### Option B: Run the installer scripts manually

If you have cloned or extracted the project, open a terminal in the
project directory and run:

``` bash
bash scripts/install-wake-agent.sh
bash scripts/install-gnome-extension.sh
systemctl --user restart proxilens-wake-agent.service
```

Keep a copy of the previously working version before updating so that
you can roll back if needed.

### Option C: Debian package (.deb)

The repository now contains Debian packaging metadata in `debian/`. Build on
Debian/Ubuntu with Go 1.27 or newer (the version required by `go.mod`) and the
packaging and Fyne build dependencies installed:

```bash
sudo apt-get install debhelper dpkg-dev golang-go build-essential pkg-config \\
  libgl1-mesa-dev libegl1-mesa-dev xorg-dev libwayland-dev \\
  libxkbcommon-dev wayland-protocols
./scripts/build-deb.sh
```

The resulting architecture-specific package is written to the parent
directory. Install it with:

```bash
sudo apt install ./proxilens_1.0.0-1_$(dpkg --print-architecture).deb
```

The package installs both executables in `/usr/bin`, the desktop entry and
icon system-wide, the GNOME extension in
`/usr/share/gnome-shell/extensions/proxilens-input@local`, and the systemd
**user** unit in `/usr/lib/systemd/user`.

The GNOME extension is installed system-wide but must be enabled separately
for each desktop user (for example from ProxiLens or with
`gnome-extensions enable proxilens-input@local`). The Wake Agent is not
enabled or started automatically. Each user who wants it can run:

```bash
systemctl --user daemon-reload
systemctl --user enable --now proxilens-wake-agent.service
```

The package does not create a root service, does not enable the agent for all
users, and does not remove `~/.config/proxilens` on package removal. Existing
user settings and the GNOME keyring encryption key remain user-owned.

### Verify the GNOME extension

``` bash
gnome-extensions info proxilens-input@local
```

### Follow the agent log

``` bash
journalctl --user -u proxilens-wake-agent.service -f
```

GNOME extensions may need to be reinitialized after updates or
session-mode changes. If the Duo button does not appear, try:

``` bash
gnome-extensions disable proxilens-input@local
gnome-extensions enable proxilens-input@local
```

## Build from source

Install the Linux development dependencies listed in
[`.github/workflows/ci.yml`](.github/workflows/ci.yml), then build both
binaries from the repository root:

``` bash
mkdir -p dist
go build -trimpath -o dist/proxilens-ui .
go build -trimpath -o dist/proxilens-wake-agent ./cmd/proxilens-wake-agent
```

Run the test suite with:

``` bash
go test ./...
```

## Test procedure

1.  Press `Super+L` and confirm that **Unlock with Duo** appears on the
    GNOME lock screen.
2.  Click **Unlock with Duo**.
3.  Approve the Duo Push request on your enrolled device.
4.  Confirm that the desktop unlocks after successful Duo authentication
    without asking for the Ubuntu password again.
5.  Test Sleep/Wake separately.
6.  Confirm that the existing Duo workflow is preserved after resume
    when the session is still locked.

Expected status sequence for the manual button workflow:

``` text
GNOME sessionMode: unlock-dialog active
Duo state: LOCKED
Duo state: DUO_PENDING
Duo state: DUO_SUCCESS
Duo state: UNLOCK_REQUESTED
GNOME sessionMode: user mode active
Duo state: UNLOCKED
```

Each state change should appear only once in the agent log. Depending on
timing, `GNOME sessionMode: user mode active` may appear immediately
before or after the final status notification.

## What ProxiLens does not change

-   It does not modify `/etc/pam.d/*`, PAM, or GDM.
-   It does not replace the normal GNOME lock-screen configuration.
-   Pressing `Super+L` does not automatically start a Duo request; the
    user must explicitly click the Duo button.
-   A GNOME status signal alone does not authorize an unlock. The agent
    checks the Duo result and the current logind lock status
    before requesting the unlock.

## Security and operational limitations

> \[!CAUTION\] ProxiLens is an experimental unlock helper for an already
> logged-in, locked user session. It is **not** a PAM module and is
> **not** a replacement for authentication at system startup or at the
> GDM login screen.

-   The agent runs as a user process in the logged-in user's context.
    The session D-Bus is not a system-wide privileged authentication
    broker.
-   The agent requests `loginctl unlock-session` after successful
    Duo authentication and a locked-session check. This design
    requires further security review before production use.
-   The GNOME extension interacts with private GNOME Shell interfaces,
    including `Main.screenShield` and its dialog/clock widgets. GNOME
    updates may change these internal interfaces and break
    compatibility.
-   Network or DNS outages may delay or prevent Duo authentication. A
    failed Duo workflow must not unlock the session; the regular GNOME
    unlock method remains available.
-   Do not put passwords, tokens, Duo secrets, private server values, or
    other credentials in source code, this README, logs, or public
    project archives.
-   Use this prototype only if you understand its limitations and have a
    working alternative way to access your system.

## Uninstallation and rollback

Disable the GNOME extension:

``` bash
gnome-extensions disable proxilens-input@local
```

Stop and disable the user agent:

``` bash
systemctl --user disable --now proxilens-wake-agent.service
```

This alpha version does not modify PAM or GDM. If you kept a backup of
the previous installation, restore it according to your local setup
procedure.

## GitHub status in the desktop application

The desktop dashboard checks this repository for the latest published
release and the most recent successful GitHub Actions run. The status is
refreshed every 10 minutes. Links to the repository, releases, and
Actions are available in the application footer.

If GitHub cannot be reached, the desktop application continues to work;
only the online repository status is unavailable.

## Further documentation

-   [Alpha.7.3.1 --- Duo status and test
    guide](README-ALPHA7.3-DUO-STATE.md)
-   [Alpha.7 --- GNOME Duo button](README-ALPHA7-DUO-BUTTON-TEST.md)
-   [Sleep/Wake test](README-ALPHA6-TEST.md)
-   [Configuration guide](README-KONFIGURATION.md)

## License

ProxiLens is licensed under the [MIT License](LICENSE).

Copyright (c) 2026 Michael Wlotkowski.

## Manuelles Test-DEB (AMD64)

Für Tests auf einem separaten Branch gibt es den GitHub-Actions-Workflow
`.github/workflows/test-deb-amd64.yml`. Er verändert den Release-Workflow nicht
und wird ausschließlich manuell gestartet:

1. Änderungen auf den gewünschten Test-Branch pushen.
2. Im GitHub-Repository **Actions → Test DEB (AMD64)** öffnen.
3. **Run workflow** wählen und den Test-Branch auswählen.
4. Nach erfolgreichem Lauf das Artefakt `proxilens-test-deb-amd64-*`
   herunterladen. Es enthält das `.deb` und eine SHA-256-Prüfsummendatei.

Der Workflow baut nur für AMD64, veröffentlicht nichts als GitHub-Release und
behält das Build-Artefakt 14 Tage. Die Go-Tests laufen vor dem Paketbau.
