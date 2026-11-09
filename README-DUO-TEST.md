# ProxiLens – direkter Authelia Duo-Push-Test

## Änderungen

- Der bisherige OIDC Device Authorization Flow wurde aus der Oberfläche entfernt.
- ProxiLens fragt das Authelia-Passwort für jeden Vorgang erneut ab.
- Das Passwort wird nicht in Konfigurationen oder Logs geschrieben.
- Authentifizierung erfolgt über Authelias HTTPS-API: `POST /api/firstfactor`, `GET /api/user/info`, `POST /api/secondfactor/duo`.
- Das Authelia-Sitzungscookie wird ausschließlich im Arbeitsspeicher des laufenden Vorgangs gehalten.
- Vor dem Duo-Aufruf wird geprüft, ob `has_duo` für den angemeldeten Benutzer wahr ist.
- Der Benutzername ist weiterhin fest auf `<Authelia-Benutzername>` gesetzt.
- Die einmalige Entsperrfreigabe bleibt standardmäßig deaktiviert.
- Es werden keine PAM-, GDM-, GNOME- oder Authelia-Servereinstellungen geändert.

## Bauen

Auf dem Ubuntu-Laptop im entpackten Projektverzeichnis:

```bash
go version
# Erforderlich: Go 1.26 oder neuer entsprechend go.mod

go test ./...
go build -o proxilens-ui ./cmd/proxilens-ui
./proxilens-ui
```

## Sicherer erster Test

1. Die Checkbox für den einmaligen Entsperrtest ausgeschaltet lassen.
2. Auf „Mit Authelia + Duo Push anmelden“ klicken.
3. Authelia-Passwort eingeben.
4. Prüfen, ob eine Duo-Push-Anfrage auf dem iPhone erscheint.
5. Push bestätigen und auf die Erfolgsmeldung in ProxiLens achten.
6. Prüfen, dass der Desktop dabei nicht entsperrt wird.

Erst nach erfolgreichem Authentifizierungstest separat den einmaligen Entsperrtest durchführen. Die Checkbox ist eine ausdrückliche Freigabe und bleibt nach jedem Versuch ausgeschaltet.

## Fehlerhinweise

- `HTTP 401` bei First Factor: Benutzername/Passwort falsch oder Authelia lehnt die Anmeldung ab.
- Kein Sitzungscookie: Authelia hat keine authentifizierte Sitzung ausgegeben; der Ablauf wird abgebrochen.
- `has_duo=false`: Für den Benutzer ist in Authelia kein Duo-Faktor registriert.
- Fehler bei `/api/secondfactor/duo`: Duo-Anfrage wurde nicht erfolgreich abgeschlossen; es erfolgt keine Entsperrung.

Die Implementierung orientiert sich an der offiziellen Authelia PAM-Integration und verwendet deren dokumentierte API-Sequenz.
