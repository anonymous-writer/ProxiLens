# ProxiLens 0.0.2 Alpha.6 – Sleep/Wake mit logind LockedHint

## Ziel dieser Testversion

Diese Version beschränkt den automatischen Duo-Auslöser ausdrücklich auf das
Aufwachen aus Sleep/Suspend. Eine normale Bildschirmsperre mit `Super+L` löst
keinen Duo-Push aus und wird nicht automatisch aufgehoben.

## Änderungen gegenüber Alpha.5

- `org.gnome.ScreenSaver.WakeUpScreen` löst keine Authentifizierung mehr aus.
- Die bisherige GNOME-Eingabeerweiterung wird nicht benötigt; der Installer
  deaktiviert sie, löscht ihre Dateien aber nicht.
- Der Agent beobachtet `LockedHint` der aktiven logind-Wayland-Sitzung
  (einmal pro Sekunde) und protokolliert Zustandswechsel.
- Nach Resume wird der Sperrzustand direkt mit `loginctl` geprüft.
- Vor dem Entsperren wird erneut geprüft, dass dieselbe aktive Sitzung noch
  gesperrt ist.
- PAM und GDM werden nicht verändert.

## Installation

1. Alpha.5 als Rückfallversion behalten.
2. ZIP in ein separates Verzeichnis entpacken.
3. Im Projektverzeichnis ausführen:

   ```bash
   bash deploy/install-wake-agent.sh
   ```

4. Status prüfen:

   ```bash
   systemctl --user status proxilens-wake-agent
   journalctl --user -u proxilens-wake-agent -f
   ```

## Testplan

1. Agent starten und Live-Log öffnen.
2. Mit `Super+L` sperren und 15 Sekunden warten: Es darf kein Duo-Push kommen.
   Im Log sollte `LockedHint: ... ist gesperrt` erscheinen.
3. Normal entsperren: Es sollte `LockedHint: ... ist entsperrt` erscheinen.
4. Erneut sperren und dann Sleep/Suspend auslösen.
5. Gerät aufwecken: Nur wenn die aktive Wayland-Sitzung noch gesperrt ist,
   startet ProxiLens den Duo-Ablauf.
6. Duo ablehnen oder nicht bestätigen: Die Sitzung muss gesperrt bleiben.
7. Duo bestätigen: ProxiLens prüft die Sitzungsidentität und den Sperrzustand
   erneut, bevor `loginctl unlock-session` ausgeführt wird.

## Sicherheitsgrenzen

- Normales `Super+L` löst keine Authentifizierung aus.
- Sleep/Wake allein entsperrt nicht. Eine erfolgreiche Duo-Prüfung und eine
  erneute Prüfung derselben aktiven, gesperrten Wayland-Sitzung sind nötig.
- Bei Netzwerk-, Schlüsselbund-, Konfigurations- oder Authentifizierungsfehlern
  bleibt die Sitzung gesperrt.
- Keine Änderungen an PAM, GDM oder systemweiten Authentifizierungsregeln.
- `LockedHint` wird aktuell einmal pro Sekunde gelesen. Es dient der
  Zustandsbeobachtung; die sicherheitsrelevanten Prüfungen erfolgen zusätzlich
  unmittelbar vor dem Authentifizierungsstart und vor dem Entsperren.

## GNOME-Erweiterung

Die Erweiterung wird nicht mehr benötigt. Sie wird durch den Installer
deaktiviert, aber nicht gelöscht. Zum späteren endgültigen Entfernen:

```bash
bash deploy/uninstall-gnome-extension.sh
```

## Deaktivieren des Wake-Agenten

```bash
systemctl --user disable --now proxilens-wake-agent.service
rm -f ~/.config/systemd/user/proxilens-wake-agent.service ~/.local/bin/proxilens-wake-agent
systemctl --user daemon-reload
```
