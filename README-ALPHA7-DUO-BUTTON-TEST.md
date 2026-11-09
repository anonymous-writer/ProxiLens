# ProxiLens Alpha.7 – experimenteller Duo-Button auf dem GNOME-Sperrbildschirm

## Ziel

Diese Testversion ergänzt den GNOME-Entsperrdialog um „Mit Duo entsperren“. Der Button sendet ausschließlich ein D-Bus-Signal an den ProxiLens-Benutzer-Agenten. Die Erweiterung meldet zusätzlich den Wechsel des GNOME-Session-Modus (`unlock-dialog`/`user`) über das Session-D-Bus an den Agenten. Der Agent protokolliert diesen Zustand und verlangt ihn als zusätzliche Vorbedingung für einen Button-Klick. Vor der Authentifizierung und unmittelbar vor dem Entsperrversuch prüft er weiterhin `LockedHint`; die GNOME-Meldung ersetzt keine Authentifizierung und ist allein keine Freigabe.

Die normale Ubuntu-Passwortentsperrung bleibt verfügbar. PAM/GDM werden nicht verändert. Sleep/Wake bleibt als separater Auslöser erhalten.

## GNOME-Sperrstatus-Indikator

Beim Wechsel in `unlock-dialog` sendet die Erweiterung `LockStateChanged(true)`, beim Verlassen `LockStateChanged(false)`. Der Agent protokolliert diese Zustandswechsel. Das ist ein zusätzlicher Statusindikator; `loginctl LockedHint` bleibt die maßgebliche Sicherheitsprüfung.

## Installation

1. Alpha.6 als Rückfallversion behalten.
2. Alpha.7 in ein neues Verzeichnis entpacken.
3. Im Projektverzeichnis ausführen:

```bash
bash deploy/install-wake-agent.sh
bash deploy/install-gnome-extension.sh
systemctl --user restart proxilens-wake-agent.service
```

4. Prüfen:

```bash
systemctl --user status proxilens-wake-agent.service
journalctl --user -u proxilens-wake-agent.service -f
```

5. `Super+L` drücken. Im Sperrbildschirm sollte „Mit Duo entsperren“ erscheinen. Im Agent-Log sollte außerdem `GNOME sessionMode: unlock-dialog aktiv` stehen. Nach dem Entsperren sollte `GNOME sessionMode: user-Modus aktiv` erscheinen. Erst nach Klick darf die Duo-Anfrage starten.

## Erwartetes Verhalten

- Normales Sperren/Entsperren löst keinen Duo-Push aus.
- Klick auf „Mit Duo entsperren“ löst eine Authentifizierung aus.
- Sleep/Wake löst weiterhin den bisherigen Ablauf aus, sofern die Sitzung gesperrt bleibt.
- Bei Fehlern bleibt die Sitzung gesperrt; die normale Ubuntu-Entsperrung bleibt als Rückfall verfügbar.

## Sicherheit und Grenzen

Dies ist ein experimenteller Prototyp für GNOME Shell 50 und verwendet private GNOME-Shell-Oberflächen (`Main.screenShield._dialog._clock`). GNOME-Updates können die Darstellung verändern. Der Agent akzeptiert nur den expliziten Button-Klick und prüft vor dem Authentifizieren sowie unmittelbar vor dem Entsperrversuch die aktive gesperrte Wayland-Sitzung.

Die Freigabe verwendet dieselbe `loginctl unlock-session`-Methode wie die bereits getestete Alpha.6-Sleep-Funktion. Das ist kein PAM-Authentifizierungsmodul. Vor produktivem Einsatz muss der gesamte Ablauf auf dem Zielgerät getestet werden.

## Rückbau

```bash
gnome-extensions disable proxilens-input@local
```

Die Sleep/Wake-Funktion bleibt davon unberührt.
