#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
command -v go >/dev/null || { echo "Go ist nicht installiert." >&2; exit 1; }
command -v dbus-monitor >/dev/null || { echo "dbus-monitor fehlt. Installiere dbus-bin." >&2; exit 1; }
mkdir -p "$HOME/.local/bin" "$HOME/.config/systemd/user"
cd "$ROOT"
go build -trimpath -o "$HOME/.local/bin/proxilens-wake-agent" ./cmd/proxilens-wake-agent
install -m 0644 "$ROOT/deploy/systemd/proxilens-wake-agent.service" "$HOME/.config/systemd/user/"
systemctl --user daemon-reload
systemctl --user enable --now proxilens-wake-agent.service
echo "Wake-Agent mit Sleep/Wake-, GNOME-Sperrstatus- und Duo-Zustandsüberwachung installiert."
echo "Status: systemctl --user status proxilens-wake-agent"
echo "Log: journalctl --user -u proxilens-wake-agent -f"
