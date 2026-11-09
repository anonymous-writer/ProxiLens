# ProxiLens Wake-Agent (Alpha.6, experimentell)

Der Benutzer-Dienst hat zwei getrennte Aufgaben:

- Er beobachtet `org.freedesktop.login1.Manager.PrepareForSleep(false)` und
  startet nach dem Resume den Duo-Ablauf.
- Er liest den `LockedHint`-Status der aktiven logind-Wayland-Sitzung einmal
  pro Sekunde und protokolliert Änderungen.

Eine normale GNOME-Bildschirmsperre (`Super+L`) startet **keinen** Duo-Ablauf.
`WakeUpScreen` von GNOME wird nicht mehr als Authentifizierungsauslöser genutzt.
Der Installer deaktiviert die alte ProxiLens-GNOME-Erweiterung, löscht sie aber
nicht.

Vor dem Duo-Ablauf muss `loginctl` die aktive Wayland-Sitzung als gesperrt
melden. Direkt vor dem Entsperren wird erneut geprüft, dass es dieselbe aktive,
gesperrte Sitzung ist. PAM und GDM werden nicht verändert.

## Installation

```bash
bash deploy/install-wake-agent.sh
systemctl --user status proxilens-wake-agent
journalctl --user -u proxilens-wake-agent -f
```

Voraussetzungen: Go, `dbus-monitor`, aktive GNOME-Benutzersitzung und die
bereits in ProxiLens gespeicherten Authelia-Einstellungen.

## Deaktivieren

```bash
systemctl --user disable --now proxilens-wake-agent.service
rm -f ~/.config/systemd/user/proxilens-wake-agent.service ~/.local/bin/proxilens-wake-agent
systemctl --user daemon-reload
```

Die normale GNOME-Bildschirmsperre bleibt unabhängig davon erhalten.
