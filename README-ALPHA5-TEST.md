> **Hinweis:** Diese Alpha.5-Testanleitung ist durch `README-ALPHA6-TEST.md` ersetzt. Alpha.5 reagierte auf `WakeUpScreen`, was auf dem Zielgerät zu spät kam. Alpha.6 löst Duo ausschließlich nach Sleep/Wake aus und nutzt logind `LockedHint` zur Sperrstatusbeobachtung.

# ProxiLens 0.0.2 Alpha.5 – direkte GNOME-Eingabeerkennung

Experimentelle Testversion für Ubuntu Wayland / GNOME Shell 50.x.

## Änderung gegenüber Alpha.4

- Keine GNOME-Shell-Erweiterung mehr erforderlich.
- Der Go-Agent beobachtet `org.gnome.ScreenSaver` direkt.
- `ActiveChanged(true)` markiert den Sperrzyklus.
- Das erste `WakeUpScreen` während dieses Sperrzyklus löst Duo aus.
- Doppelte Signale von mehreren GNOME-D-Bus-Sendern werden durch den Sperrzyklus-Guard zusammengefasst.
- Sleep/Wake bleibt als separater Auslöser über logind erhalten.
- Die Eingabeinhalte und Mauskoordinaten werden nicht erfasst.

## Installation

1. Alpha.3/Alpha.4 als Rückfallversion behalten.
2. Alpha.5 in ein separates Verzeichnis entpacken.
3. Im Projektverzeichnis ausführen:

   ```bash
   bash deploy/install-wake-agent.sh
   ```

4. Prüfen:

   ```bash
   systemctl --user status proxilens-wake-agent
   journalctl --user -u proxilens-wake-agent -f
   ```

Der Installer deaktiviert die bisherige ProxiLens-GNOME-Erweiterung, falls sie vorhanden ist. Ihre Dateien werden nicht gelöscht.

## Testfälle

1. Mit `Super+L` sperren und zunächst nichts berühren: kein Duo-Push.
2. Maus bewegen oder Taste drücken: GNOME sollte `WakeUpScreen` senden und genau einen Duo-Push auslösen.
3. Weitere Eingaben während desselben Sperrzyklus: kein weiterer Push.
4. Duo ablehnen/nicht bestätigen: Sitzung bleibt gesperrt.
5. Duo bestätigen: Agent prüft vor dem Entsperren erneut die aktive, gesperrte Sitzung.
6. Sleep/Wake separat testen: automatischer Resume-Auslöser bleibt erhalten.

## Technischer Hinweis

GNOME ScreenSaver kann dasselbe Signal von mehreren D-Bus-Sendern veröffentlichen. Der Agent ignoriert Duplikate durch den `triggered`-Guard. Der tatsächliche GNOME-Eingabezeitpunkt muss auf dem Zielgerät bestätigt werden; diese Umgebung kann GNOME 50.1 nicht live ausführen. Ein Build mit Go 1.26.0 wurde hier nicht durchgeführt.
