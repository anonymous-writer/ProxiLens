# Experimenteller ProxiLens-Wake-Agent (Alpha.2)

## Umfang dieser Testfassung

Diese Fassung ergänzt einen optionalen systemd-Benutzerdienst. Er beobachtet das logind-Signal `PrepareForSleep(false)`. Wenn die vorhandene Wayland-Benutzersitzung nach dem Aufwachen noch gesperrt ist, wartet der Agent zunächst mit begrenzten Wiederholungen auf DNS und eine TCP-Verbindung zum konfigurierten Authelia-Host. Erst danach lädt er die Standarddatenbank `~/.config/proxilens/proxilens.db`, liest die vorhandenen Einstellungen und fordert über den bestehenden Authelia-/Duo-Code einen Push an. Nur nach erfolgreicher Duo-Bestätigung und erneuter Prüfung derselben gesperrten Sitzung wird `loginctl unlock-session` aufgerufen.

Der Dienst ist **nicht automatisch aktiviert**. Installation erfolgt nur durch den Benutzer. Er benötigt eine laufende grafische Benutzersitzung, `dbus-monitor` und Zugriff auf den GNOME-Schlüsselbund. Er verwendet derzeit den Standard-Datenbankpfad; ein in der GUI abweichend gewählter Datenbankpfad wird noch nicht übernommen.

## Installation

1. Zuerst ProxiLens normal öffnen und Authelia-URL, Benutzername und Passwort speichern.
2. Im Projektverzeichnis ausführen: `./deploy/install-wake-agent.sh`
3. Status prüfen: `systemctl --user status proxilens-wake-agent`
4. Logs ansehen: `journalctl --user -u proxilens-wake-agent -f`

Deaktivieren/entfernen:

```bash
systemctl --user disable --now proxilens-wake-agent.service
rm -f ~/.config/systemd/user/proxilens-wake-agent.service ~/.local/bin/proxilens-wake-agent
systemctl --user daemon-reload
```

## Wichtige Einschränkung

**Die Auslösung durch die erste Tastatur- oder Mausaktivität am bereits gesperrten GNOME-Wayland-Bildschirm ist in dieser Alpha.2 noch nicht enthalten.** Wayland erlaubt einer normalen Anwendung nicht, globale Eingaben am Sperrbildschirm mitzulesen. Dafür ist eine separat installierbare und versionskompatible GNOME-Shell-Erweiterung nötig. Diese Testfassung implementiert nur den zusätzlichen Sleep/Wake-Auslöser. Sie verändert weder PAM noch GDM und startet keine Authentifizierung beim normalen Sperren oder bei beliebigen Eingaben.

## Sicherheit / Testhinweise

- Nur eine laufende Duo-Anfrage gleichzeitig.
- Sitzung muss vor dem Authentifizieren gesperrt und danach weiterhin dieselbe gesperrte Sitzung sein.
- Bei Fehlern bleibt die Sitzung gesperrt.
- Der Dienst protokolliert Statusmeldungen, aber keine Passwörter.
- Die GNOME-Schlüsselbund-Entsperrbarkeit nach Resume hängt von deiner Desktop-Konfiguration ab.
- Dies ist ein experimenteller Teststand, nicht als produktionsreif zertifiziert.

## DNS-/Netzwerk-Warteverhalten (DNS-Fix)

Nach `PrepareForSleep(false)` wartet der Agent auf die Erreichbarkeit des in den Einstellungen konfigurierten Authelia-Hosts. Er prüft dazu eine TCP-Verbindung zum Host/Port mit begrenzten Wiederholungen (bis etwa eine Minute). Erst wenn die Verbindung steht, wird genau ein Authentifizierungsversuch gestartet. Die Warteprüfung sendet keine Zugangsdaten. Die systemd-Service-Datei enthält keine ungültige `Documentation=`-Dateiangabe mehr.
