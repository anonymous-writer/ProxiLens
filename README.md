# ProxiLens

**Bluetooth proximity diagnostics for Ubuntu 24.04 LTS / GNOME**, written in Go.

> Current status: Phase 1 (BLE discovery and diagnostics). Automatic locking and unlocking are not implemented yet.

## Features in this phase

- Discover Bluetooth Low Energy devices through BlueZ/D-Bus.
- Display discovered devices and signal strength (RSSI) in a Fyne desktop dashboard.
- Provide a command-line scanner for troubleshooting.
- GitHub Actions CI checks formatting, tests, vet, and builds both applications.

## Requirements

- Ubuntu 24.04 LTS (GNOME)
- Go 1.22 or newer
- BlueZ
- Fyne build dependencies

Install system packages:

```bash
sudo apt update
sudo apt install -y golang-go bluez libgl1-mesa-dev xorg-dev
sudo systemctl enable --now bluetooth
```

## Run from source

```bash
git clone https://github.com/OWNER/REPOSITORY.git
cd proxilens
go mod tidy
go run ./cmd/proxilens-ui
```

CLI scanner:

```bash
go run ./cmd/proxilens-scan
```

## GitHub repository setup

1. Create a new **public or private repository** on GitHub named `proxilens`.
2. Replace `OWNER/REPOSITORY` in this README and `.github/ISSUE_TEMPLATE/config.yml` with your GitHub owner and repository.
3. Set the Go module path to your real repository path. For example, if your GitHub account is `my-account`, change `module github.com/local/proxilens` in `go.mod` to `module github.com/my-account/proxilens`, then replace all imports beginning with `github.com/local/proxilens/` with `github.com/my-account/proxilens/`.
4. Run `go mod tidy`, then commit **both** `go.mod` and `go.sum`.
5. Push the project to the `main` branch.

Example commands after creating the empty GitHub repository (replace the URL):

```bash
git init
git add .
git commit -m "chore: initial ProxiLens project"
git branch -M main
git remote add origin https://github.com/OWNER/REPOSITORY.git
git push -u origin main
```

The workflow in `.github/workflows/ci.yml` runs automatically on pushes to `main` and pull requests.

## Creating a release

Create a version tag such as `v0.1.0`, then create and publish a GitHub Release for that tag. The release workflow builds Linux amd64 binaries for the GUI and CLI, packages them into a `.tar.gz`, generates a SHA-256 checksum, and attaches both files to the GitHub Release.

## Development commands

```bash
gofmt -w .
go test ./...
go vet ./...
go build ./cmd/proxilens-scan
go build ./cmd/proxilens-ui
```

## Current limitations and security

The iPhone and Apple Watch detection behavior must be validated on the target hardware. Apple devices may use private or rotating BLE addresses and do not necessarily advertise a stable, continuously identifiable signal.

RSSI is only a proximity heuristic, not proof of identity. A nearby BLE advertisement must never be sufficient to authenticate or unlock a user session. A future implementation should preserve normal GNOME/GDM authentication and use a cryptographically verifiable mechanism for any unlock-related feature.

## License

No license has been selected yet. Choose and add a license before publishing the repository if you want to grant others explicit reuse rights.


## Apple Watch Lernmodus (experimentell)

1. Bluetooth-Scan im Dashboard starten.
2. **Apple Watch anlernen** öffnen.
3. Für die 30-Sekunden-Referenzmessung die Watch außer Reichweite legen.
4. Anschließend die Watch neben den Rechner legen und die 60-Sekunden-Messung starten.
5. Kandidaten prüfen und den passenden Eintrag speichern.

Die Zuordnung wird lokal unter `~/.config/proxilens/apple-watch.json` gespeichert.
Die Kandidatensuche vergleicht mittlere RSSI-Werte und neu sichtbare BLE-Adressen.
Sie ist eine experimentelle Näherung: BLE-Adressen können rotieren und RSSI ist
weder eine eindeutige Geräteidentität noch ein Authentifizierungsnachweis.
Der Lernmodus entsperrt oder sperrt den Rechner nicht.


### Hinweis zur Kandidatenbewertung

Neu auftauchende BLE-Adressen werden nur als „neu entdeckt“ angezeigt.
Für sie wird keine künstliche RSSI-Verbesserung berechnet und sie können
nicht als Watch gespeichert werden. Eine speicherbare Kandidatenbewertung
setzt voraus, dass dieselbe Adresse in Referenz- und Nahmessung vorkommt
und sich ihr RSSI messbar verbessert. Bei rotierenden Apple-Adressen kann
das weitere Messmethoden bzw. wiederholte Lernläufe erfordern.


## BLE-Diagnosemodus

Im Dashboard **BLE-Diagnose exportieren** anklicken. ProxiLens speichert die aktuell
von BlueZ gelieferten Gerätedaten als JSON unter
`~/.config/proxilens/ble-diagnostics.json` (Dateirechte 0600).
Erfasst werden Name, Adresse, RSSI, TxPower, Service-UUIDs, Manufacturer Data
und Service Data, soweit BlueZ diese Informationen vom Adapter bereitstellt.
Alternativ zeigt `go run ./cmd/proxilens-scan` diese Felder im Terminal.

Hinweis: BlueZ kann Werbedaten je nach Controller, BlueZ-Version und
Betriebszustand unterschiedlich vollständig bereitstellen. Apple-Geräte können
private/rotierende Adressen verwenden. Aus Service-UUIDs oder Herstellerdaten
allein folgt keine sichere Geräteidentität. Der Export enthält lokale
Bluetooth-Umgebungsdaten; vor dem Teilen bitte prüfen und fremde Geräteadressen
ggf. schwärzen.


## Apple-BLE-Monitoring (experimentell)

Die Geräteliste kennzeichnet Geräte mit Manufacturer Company ID `0x004C`
(dezimal 76) als **Apple-Herstellerdaten**. Das ist ein Hinweis auf Apple-formatierte
Werbedaten, aber kein Nachweis für eine Apple Watch: iPhone, AirPods und andere
Apple-Geräte können dieselbe Company ID verwenden.

Der Button **BLE-Diagnose exportieren** enthält jetzt zusätzlich `events`:
zeitgestempelte Änderungen an Adresse, Name, RSSI, UUIDs und Herstellerdaten.
Die Ereignisse werden im laufenden Programm im Speicher gesammelt (maximal 20.000)
und beim Export zusammen mit dem aktuellen Gerätesnapshot gespeichert.
BlueZ liefert nur die Daten, die Adapter und BlueZ tatsächlich bereitstellen;
dies ist kein vollständiger Rohpaket-Mitschnitt und kein kontinuierlicher
Bluetooth-HCI-Sniffer.

Für den Vergleich bitte je zwei Minuten scannen:
A) Watch nahe am Lenovo, iPhone mehrere Meter entfernt.
B) Watch außer Reichweite, iPhone nahe am Lenovo.
Jeweils Diagnose exportieren und die beiden JSON-Dateien getrennt sichern.
Vor dem Teilen bitte fremde Geräteadressen prüfen/schwärzen.
