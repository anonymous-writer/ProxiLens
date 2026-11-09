# ProxiLens – Konfigurationsverwaltung

## Einstellungen

Die Einstellungen werden über **Einstellungen** im Hauptfenster bearbeitet.
Der Standardpfad lautet `~/.config/proxilens/proxilens.db`. Ein anderer
SQLite-Dateipfad kann dort gewählt werden. Bei einem Pfadwechsel wird eine
neue Datenbank verwendet; die bisherige Datei wird nicht verschoben oder
überschrieben.

Die Datenbank enthält Authelia-URL und Benutzername. Das Authelia-Passwort
wird mit AES-256-GCM verschlüsselt in SQLite abgelegt. Der zufällig erzeugte
32-Byte-Schlüssel liegt separat im Desktop-Schlüsselbund (Secret Service).
ProxiLens bricht das Speichern ab, wenn der Schlüsselbund nicht verfügbar ist.

## Erstmalige Einrichtung

1. ProxiLens starten und **Einstellungen** öffnen.
2. Authelia-Server und Benutzername prüfen.
3. Authelia-Passwort eingeben.
4. Standarddatenbankpfad übernehmen oder einen anderen Pfad wählen.
5. Speichern. Danach kann die direkte Duo-Push-Anmeldung gestartet werden.

Beim Wechsel auf eine neue Datenbank wird nichts aus der alten Datenbank
kopiert. Das Passwort muss dort erneut eingegeben werden. Die alte Datenbank
bleibt unverändert erhalten.

## Build

```bash
go test ./...
go build -o proxilens-ui ./cmd/proxilens-ui
```

Benötigt die in `go.mod` angegebene Go-Version und Zugriff auf die Go-Module.
Unter GNOME muss ein Secret-Service-kompatibler Schlüsselbund verfügbar und
entsperrt sein, damit das Passwort gespeichert bzw. gelesen werden kann.
