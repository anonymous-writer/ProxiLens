#!/usr/bin/env bash
set -euo pipefail
UUID="proxilens-input@local"
gnome-extensions disable "$UUID" 2>/dev/null || true
rm -rf "$HOME/.local/share/gnome-shell/extensions/$UUID"
echo "ProxiLens GNOME-Eingabeerweiterung entfernt."
