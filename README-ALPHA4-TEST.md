# ProxiLens 0.0.2 Alpha.4 – GNOME-Eingabeauslöser

Experimentelle Testversion für GNOME Shell 50.x / Ubuntu Wayland.

## Änderungen

- `WakeUpScreen` löst bei einer normalen Bildschirmsperre keinen Push mehr aus.
- Neue GNOME-Shell-Erweiterung erkennt die erste Tastatur-, Maus- oder Touch-Aktivität am gesperrten Bildschirm.
- Die Erweiterung sendet nur ein D-Bus-Ereignis; sie protokolliert keine Tasteninhalte und keine Mauskoordinaten.
- Der Go-Agent prüft den tatsächlichen Sperrstatus mit `loginctl` und startet höchstens einen Authentifizierungsversuch pro Sperrzyklus.
- Sleep/Wake bleibt als separater Auslöser erhalten.

## Installation

1. Bestehende Alpha.3 als Rückfallversion behalten.
2. Alpha.4 in ein separates Verzeichnis entpacken.
3. Im Projektverzeichnis ausführen:

   ```bash
   bash deploy/install-wake-agent.sh
   ```

4. Falls GNOME die Erweiterung nicht sofort lädt, unter Wayland ab- und wieder anmelden.
5. Prüfen:

   ```bash
   gnome-extensions info proxilens-input@local
   systemctl --user status proxilens-wake-agent
   journalctl --user -u proxilens-wake-agent -f
   ```

## Testfälle

1. Bildschirm mit `Super+L` sperren und zunächst nichts berühren: Es darf kein Push kommen.
2. Maus bewegen oder eine Taste drücken: genau ein Duo Push sollte ausgelöst werden.
3. Während der Anfrage weitere Eingaben machen: kein zweiter Push.
4. Duo ablehnen oder nicht bestätigen: Sitzung bleibt gesperrt.
5. Duo bestätigen: Agent prüft die aktive, gesperrte Wayland-Sitzung erneut und fordert dann das Entsperren an.
6. Sleep/Wake separat testen: der bestehende automatische Resume-Auslöser bleibt aktiv.

## Einschränkungen

- Experimentell; nicht auf GNOME Shell 50.1 in dieser Build-Umgebung live getestet.
- Der GNOME-Shell-Event-Hook ist eine globale Stage-Capture-Verbindung. GNOME-Shell-Updates können Änderungen an der API erfordern.
- Ein erfolgreicher `go test`/Build konnte hier nicht ausgeführt werden, da diese Umgebung Go 1.26.0 nicht herunterladen konnte.
- Die Funktion ersetzt keine PAM/GDM-Anmeldung nach einem Neustart.
