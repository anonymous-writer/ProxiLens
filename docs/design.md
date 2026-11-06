# ProxiLens – Design und technische Leitplanken

## Visuelle Sprache

- Hintergrund: `#0d1929`
- Karten: `#16263b`
- Rahmen: `#29405c`
- Akzent: `#58a9ff`
- Primärtext: `#edf3ff`
- Sekundärtext: `#a8bad2`
- OK: `#35d6a0`
- Warnung: `#f5bd4f`
- Fehler/Offline: `#ff647c`

## Architektur Phase 1

- `internal/bluez`: BlueZ D-Bus-Anbindung und Geräteereignisse
- `internal/ui`: Fyne-Diagnoseoberfläche
- `cmd/proxilens-scan`: Konsolen-Diagnose
- `cmd/proxilens-ui`: grafische Oberfläche

Das Dashboard zeigt aktuell gefundene BLE-Geräte, Adresse und RSSI. Es verändert den Sperrzustand des Rechners nicht.

## Sicherheitsgrenze

BLE-Werbung und RSSI können höchstens eine Näherung liefern. Vor einer automatischen Entsperrung ist ein kryptografischer Besitznachweis erforderlich. Wenn iPhone/Apple Watch ohne zusätzliche App keinen geeigneten Nachweis bereitstellen, muss ein anderes vertrauenswürdiges Verfahren gewählt werden. Passwort-Login bleibt immer verfügbar.
