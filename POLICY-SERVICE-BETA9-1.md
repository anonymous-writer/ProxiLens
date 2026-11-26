# ProxiLens Beta 9.1 – gehärteter Root Policy Probe

## Sicherheitsgrenze

Dies ist weiterhin ausschließlich ein Diagnose-/Beobachtungsdienst. Jede Antwort setzt `authorized: false`; es gibt keine Root-Freigabe, keine Ausführung privilegierter Aktionen und keine Integration in PAM, Polkit, sudoers oder Duo.

## Härtungen gegenüber Beta 9

- Unix-Socket `0660`, Eigentümer `root`, Gruppe `sudo`; nur root und lokale Mitglieder der Gruppe `sudo` können verbinden.
- Laufzeitverzeichnis `0750`, Eigentümer `root:sudo`.
- Höchstens acht gleichzeitig bearbeitete Verbindungen; weitere Verbindungen werden abgewiesen.
- Drei Sekunden Deadline pro Verbindung.
- Anfragezeile auf 1024 Bytes begrenzt; unvollständige/zu große Zeilen und zusätzliches JSON werden abgelehnt.
- Peer PID/UID/GID stammen aus `SO_PEERCRED`.
- Sitzungszuordnung bleibt diagnostisch und basiert auf logind plus Prozess-Cgroup.
- Tests decken Größenlimit und Zeilenabschluss ab.

## Build und Tests auf Ubuntu

```bash
gofmt -w internal/policyservice/*.go
go test ./internal/policyservice -count=1
go test -race ./internal/policyservice -count=1
go test ./...
go test -race ./...
go build -trimpath -o proxilens-ui .
```

## Installationshinweis

Noch nicht installieren, bis die Tests auf dem Zielsystem erfolgreich sind und die systemd-Unit geprüft wurde. Die Unit ist ein neuer separater Dienst; sie ändert keine PAM-, sudoers- oder Polkit-Dateien. Der Dienst darf nicht als Autorisierungsquelle verwendet werden.
