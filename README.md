# ProxiLens – Bluetooth-Proximity für Ubuntu 24.04

Eigenständige Go-Neuentwicklung für Ubuntu 24.04 LTS / GNOME. Das Dashboard orientiert sich an der visuellen Sprache von LeaseLens (Navy `#0d1929`, Karten `#16263b`, Akzentblau `#58a9ff`, feine blaue Rahmen).

## Entwicklungsstand: Phase 1 – BLE-Diagnose

- BlueZ über D-Bus anbinden
- Bluetooth-LE-Discovery starten und sichtbare Geräte samt RSSI erfassen
- Fyne-Dashboard zur Live-Anzeige der gefundenen Geräte
- **Noch keine automatische Sperre oder Entsperrung**

Die Erkennung von iPhone und Apple Watch muss am Zielrechner praktisch geprüft werden. Apple-Geräte können private/wechselnde BLE-Adressen verwenden und senden nicht zwingend dauerhaft identifizierbare Werbung. Die macOS-CoreBluetooth- und Bluetooth-Cache-Logik des ursprünglichen BLEUnlock lässt sich nicht unverändert auf Linux übertragen.

## Voraussetzungen

```bash
sudo apt update
sudo apt install golang-go bluez libgl1-mesa-dev xorg-dev
sudo systemctl enable --now bluetooth
bluetoothctl show
```

Für den Build wird Go 1.22 oder neuer benötigt. Im Projektverzeichnis:

```bash
go mod tidy
```

## Start

Live-Dashboard:

```bash
go run ./cmd/proxilens-ui
```

Konsolen-Diagnose:

```bash
go run ./cmd/proxilens-scan
```

Falls BlueZ den Scan nicht starten lässt, zunächst interaktiv prüfen:

```bash
bluetoothctl
# scan on
```

## Nächste Entwicklungsschritte

1. Scan-Ergebnisse auf dem Zielsystem mit iPhone und Apple Watch erfassen.
2. Sichtbare Namen, Service-UUIDs, Manufacturer-Data und Adressstabilität auswerten.
3. Gerätebindung belastbar gestalten; nicht allein auf Name, MAC oder RSSI vertrauen.
4. RSSI-Filter, Hysterese, Verzögerungen und Signal-Timeout ergänzen.
5. Sitzungssperre über `loginctl lock-session` integrieren.
6. Einen systemd-User-Dienst und eine Installationsroutine ergänzen.
7. Entsperrung separat mit einem überprüfbaren kryptografischen Authentifizierungsverfahren entwerfen.

## Sicherheit

RSSI ist eine Näherungsheuristik, kein Authentifizierungsnachweis. Ein BLE-Gerät in Reichweite oder eine kopierte Kennung darf nicht allein die Sitzung entsperren. Ein PAM-Modul nach dem Muster „BLE-Gerät gesehen = Anmeldung erfolgreich“ wird ausdrücklich nicht implementiert. Die GNOME-/GDM-Anmeldung muss jederzeit per regulärem Passwort bzw. bestehendem Anmeldeverfahren möglich bleiben.
