#!/usr/bin/env bash
set -Eeuo pipefail

UUID="proxilens-input@local"
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE="${ROOT}/gnome-extension/${UUID}"
EXTENSIONS_DIR="${HOME}/.local/share/gnome-shell/extensions"
TARGET="${EXTENSIONS_DIR}/${UUID}"
STAGE="${EXTENSIONS_DIR}/.${UUID}.new.$$"
BACKUP="${EXTENSIONS_DIR}/.${UUID}.backup.$$"

log() { printf '[ProxiLens] %s\n' "$*"; }
die() { printf '[ProxiLens] ERROR: %s\n' "$*" >&2; exit 1; }

command -v gnome-shell >/dev/null 2>&1 || die "GNOME Shell was not found."
command -v gnome-extensions >/dev/null 2>&1 || die "The gnome-extensions command was not found."

VERSION="$(gnome-shell --version | awk '{print $3}')"
[[ "$VERSION" == 50.* || "$VERSION" == 50 ]] || \
  die "This experimental extension targets GNOME Shell 50.x; detected: ${VERSION:-unknown}."

# Validate the complete source before touching the current installation.
[[ -d "$SOURCE" ]] || die "Extension source directory is missing: $SOURCE"
[[ -s "$SOURCE/metadata.json" ]] || die "metadata.json is missing or empty."
[[ -s "$SOURCE/extension.js" ]] || die "extension.js is missing or empty."
command -v python3 >/dev/null 2>&1 || die "python3 is required to validate metadata.json."
python3 - "$SOURCE/metadata.json" "$UUID" <<'PY'
import json, sys
with open(sys.argv[1], encoding="utf-8") as f:
    data = json.load(f)
if data.get("uuid") != sys.argv[2]:
    raise SystemExit("metadata.json UUID does not match the extension directory.")
if "50" not in data.get("shell-version", []):
    raise SystemExit("metadata.json does not declare GNOME Shell 50 support.")
PY

install -d -m 0755 "$EXTENSIONS_DIR"
cleanup() {
  rm -rf -- "$STAGE"
  if [[ -d "$BACKUP" && ! -e "$TARGET" ]]; then
    mv -- "$BACKUP" "$TARGET"
  else
    rm -rf -- "$BACKUP"
  fi
}
trap cleanup EXIT

# Stage and validate a complete copy first; preserve the previous install for rollback.
install -d -m 0755 "$STAGE"
install -m 0644 "$SOURCE/metadata.json" "$STAGE/metadata.json"
install -m 0644 "$SOURCE/extension.js" "$STAGE/extension.js"
python3 -m json.tool "$STAGE/metadata.json" >/dev/null

if [[ -e "$TARGET" ]]; then
  gnome-extensions disable "$UUID" >/dev/null 2>&1 || true
  mv -- "$TARGET" "$BACKUP"
fi
mv -- "$STAGE" "$TARGET"

if ! gnome-extensions enable "$UUID"; then
  rm -rf -- "$TARGET"
  [[ ! -d "$BACKUP" ]] || mv -- "$BACKUP" "$TARGET"
  gnome-extensions enable "$UUID" >/dev/null 2>&1 || true
  die "Could not enable the extension. The previous installation was restored when available."
fi

rm -rf -- "$BACKUP"
trap - EXIT
log "GNOME Duo button extension installed and enabled."
log "On Wayland, logging out and back in may be required for the lock-screen UI to refresh."
