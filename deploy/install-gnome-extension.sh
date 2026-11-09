#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
UUID="proxilens-input@local"
TARGET="$HOME/.local/share/gnome-shell/extensions/$UUID"

if ! command -v gnome-shell >/dev/null 2>&1; then
  echo "GNOME Shell wurde nicht gefunden." >&2
  exit 1
fi
VERSION="$(gnome-shell --version | awk '{print $3}')"
if [[ "$VERSION" != 50* ]]; then
  echo "Diese experimentelle Erweiterung ist für GNOME Shell 50.x vorgesehen; gefunden: $VERSION" >&2
  exit 1
fi

mkdir -p "$(dirname "$TARGET")"
rm -rf "$TARGET"
install -d -m 0755 "$TARGET"
install -m 0644 "$ROOT/gnome-extension/$UUID/metadata.json" "$TARGET/metadata.json"
install -m 0644 "$ROOT/gnome-extension/$UUID/extension.js" "$TARGET/extension.js"

gnome-extensions enable "$UUID" || {
  echo "Erweiterung wurde installiert, konnte aber nicht sofort aktiviert werden." >&2
  echo "Bitte ab- und wieder anmelden und danach ausführen: gnome-extensions enable $UUID" >&2
  exit 1
}

echo "ProxiLens GNOME-Eingabeerweiterung installiert und aktiviert."
echo "Unter Wayland kann eine Ab- und Anmeldung erforderlich sein."
