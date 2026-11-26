# ProxiLens Beta 10.6 – isolierte Integrationsvorbereitung

## Inhalt

- `approval.DuoAdapter` verbindet den Approval-Coordinator mit dem vorhandenen Duo-Push-Client und reicht Statusmeldungen weiter.
- `approval.LogindSessionInspector` prüft diagnostisch, ob genau eine aktive grafische Wayland-Benutzersitzung ermittelt werden kann.
- Tests decken Context-Abbruch, Weitergabe des Duo-Adapters und Ablehnung nicht aktiver/unverfügbarer Sitzungen ab.

## Sicherheitsgrenzen

- Duo-Erfolg ist nur ein Authentifizierungsergebnis, keine Systemautorisierung.
- Die logind-Abfrage ist nur Sitzungsdiagnostik. Sie ist ausdrücklich **kein** Identitätsnachweis für einen privilegierten Daemon. Ein späterer Daemon muss den Client beispielsweise über Unix-Socket-Peer-Credentials (SO_PEERCRED) identifizieren und diese Identität mit einer vertrauenswürdigen Sitzung verknüpfen.
- Der Passwort-Fallback bleibt ein separater manueller Zustandsübergang. In dieser Beta ist kein PAM-Adapter angeschlossen; es wird kein Passwort abgefragt oder gespeichert.
- Es werden keine Grants erstellt und keine Systemaktionen ausgeführt.
- Keine Änderungen an `/etc/pam.d`, sudoers, Polkit, systemd- oder GNOME-Konfiguration.

## Prüfung auf Ubuntu

Im Projektverzeichnis:

```bash
go test -race ./...
go build -trimpath -o proxilens-ui .
```

Die Projekt-Toolchain ist in `go.mod` festgelegt. Bei einem abweichenden Go-Versionsfehler bitte die genaue Ausgabe sichern.
