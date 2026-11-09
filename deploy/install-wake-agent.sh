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
# The agent now uses logind LockedHint for lock-state observation.
# The old experimental shell extension is no longer needed; disable it if present.
if command -v gnome-extensions >/dev/null 2>&1; then
  gnome-extensions disable proxilens-input@local >/dev/null 2>&1 || true
fi
echo "Wake-Agent mit direkter GNOME-Sperrbildschirm-Erkennung installiert."
echo "Status: systemctl --user status proxilens-wake-agent"
echo "Log: journalctl --user -u proxilens-wake-agent -f"
