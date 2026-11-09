# ProxiLens

**GNOME session and authentication dashboard for Ubuntu 24.04**, written in Go/Fyne.

## Current phase

BLE code and Bluetooth dependencies have been removed completely. The current build is a safe session-monitor prototype:

- Detects the current user's logind session using `loginctl`.
- Displays session ID, Wayland/X11 type, class, active state, and `LockedHint`.
- Refreshes the status every three seconds.
- Does **not** unlock sessions, invoke PAM, modify GDM, or contact Authelia yet.

## Requirements

- Ubuntu 24.04 / GNOME
- Go 1.23 or newer
- Fyne Linux build dependencies (`libgl1-mesa-dev`, `xorg-dev`, `pkg-config`)
- `systemd-logind` / `loginctl`

Install dependencies:

```bash
sudo apt update
sudo apt install -y golang-go pkg-config libgl1-mesa-dev xorg-dev
```

Run:

```bash
go mod tidy
go run ./cmd/proxilens-ui
```

## Planned next phases

1. Authelia integration using a documented and supported authentication flow.
2. Bind each approval to a fresh, unique unlock request and the exact logind session.
3. Add a privileged, narrowly scoped unlock broker only after the authentication flow is proven.
4. Integrate the broker with GNOME session unlocking and test failure/timeout/replay behavior.

The current prototype intentionally contains no unlock command. A successful loginctl `unlock-session` command is not itself an authentication mechanism. Do not expose an unauthenticated remote command or treat a push notification alone as proof unless the result is cryptographically tied to the current request.

## Build and test

```bash
gofmt -w .
go test ./...
go vet ./...
go build ./cmd/proxilens-ui
```

## Authelia-Verbindungsprüfung (read-only)

Beim Start prüft ProxiLens automatisch die HTTPS-Verbindung zur konfigurierten
Authelia-Adresse (Standard: `https://auth.example.com`) und lädt
`/.well-known/openid-configuration`. Die Prüfung wird alle 60 Sekunden wiederholt
und kann über die Schaltfläche manuell gestartet werden. Es werden keine
Anmeldedaten gespeichert, keine Duo-Anfragen ausgelöst und keine Entsperrung
vorgenommen. TLS-Zertifikate werden regulär geprüft.

Die aktuelle Version ist weiterhin ein Beobachtungsprototyp. Sie authentifiziert
noch keine Benutzer und entsperrt keine GNOME-Sitzung.

## OIDC Device-Authorization Discovery (read-only)

Version 0.4 zeigt zusätzlich an, ob die OIDC-Discovery einen
`device_authorization_endpoint` und den `urn:ietf:params:oauth:grant-type:device_code`
Grant meldet. Dies ist nur eine Capability-Prüfung: ProxiLens startet keinen
Device-Code-Flow, erzeugt keinen QR-Code, löst keinen Duo-Push aus und erhält
keine Authentifizierung. Die angezeigte Zuordnung `lokaler Linux-Benutzer` → `Authelia-Benutzer` ist
bis zu einer echten, kryptografisch geprüften Identitätsbindung nur informativ.

Authelias dokumentierter Device-Authorization-Ablauf verlangt eine Freigabe
über Verification-URL/Benutzercode und kann bei der Anmeldung den zweiten Faktor
einschließen. Er ist nicht gleichbedeutend mit einem automatisch ausgelösten
Duo-Push ohne QR-Code oder Browser.

## Device Flow Basisstand

Der ProxiLens-Device-Flow verwendet für den Authelia-Client `proxilens` standardmäßig
`openid authelia.pam` als Scope. Das entspricht dem erfolgreichen manuellen
Device-Authorization-Test gegen den hier verwendeten Authelia-Server.

Die Anwendung fordert einen eigenen Gerätecode an, fragt den Token-Endpunkt ab
und prüft das ID-Token (Signatur, Issuer, Audience und Ablaufzeit). Der erwartete
Benutzername ist derzeit in `internal/ui/dashboard.go` als `<Authelia-Benutzername>` hinterlegt.

**Sicherheitsgrenze:** Diese Version löst keine GNOME-Entsperrung aus und ändert
keine PAM-, GDM- oder GNOME-Konfiguration.


## Version 0.5.4

Während eines laufenden Device-Flows verbirgt die Fensterschaltfläche X nur das Fenster. Die Autorisierung läuft weiter; nach ihrem Abschluss wird das Fenster wieder eingeblendet. Die Desktop-Entsperrung bleibt deaktiviert.


## Experimenteller Wake-Agent (Alpha.3)

Ein optionaler systemd-Benutzerdienst für Duo Push nach dem Aufwachen aus dem Energiesparmodus ist enthalten. Installation und Einschränkungen: [README-WAKE-AGENT.md](README-WAKE-AGENT.md). Die Erkennung der ersten Eingabe am GNOME-Wayland-Sperrbildschirm ist noch nicht enthalten.


## Alpha.6: Duo ausschließlich nach Sleep/Wake

Der optionale Wake-Agent kann in Alpha.6 den logind-Status `LockedHint` der
aktiven Wayland-Sitzung beobachten. Eine normale Bildschirmsperre mit
`Super+L` löst keinen Duo-Push aus. Der Duo-Ablauf wird ausschließlich nach
Sleep/Wake gestartet und nur dann fortgesetzt, wenn die Sitzung weiterhin
gesperrt ist. PAM und GDM werden nicht verändert. Details und Testplan:
[README-ALPHA6-TEST.md](README-ALPHA6-TEST.md).

Die Alpha.6 ist experimentell und muss auf dem Zielgerät getestet werden.


## Experimenteller GNOME-Duo-Button

Siehe [Alpha.7-Testanleitung](README-ALPHA7-DUO-BUTTON-TEST.md).
