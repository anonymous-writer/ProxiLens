
#!/usr/bin/env bash
set -Eeuo pipefail

UUID="proxilens-input@local"
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE="${ROOT}/gnome-extension/${UUID}"
WORK="$(mktemp -d)"
PACKAGE="${WORK}/${UUID}.shell-extension.zip"
TARGET="${HOME}/.local/share/gnome-shell/extensions/${UUID}"

log() {
    printf '[ProxiLens] %s\n' "$*"
}

die() {
    printf '[ProxiLens] ERROR: %s\n' "$*" >&2
    exit 1
}

cleanup() {
    rm -rf -- "$WORK"
}
trap cleanup EXIT

command -v gnome-shell >/dev/null 2>&1 ||
    die "GNOME Shell wurde nicht gefunden."

command -v gnome-extensions >/dev/null 2>&1 ||
    die "gnome-extensions wurde nicht gefunden."

command -v python3 >/dev/null 2>&1 ||
    die "Python 3 wird benötigt."

VERSION="$(gnome-shell --version | awk '{print $3}')"

[[ "$VERSION" == 50.* || "$VERSION" == 50 ]] ||
    die "Diese Erweiterung unterstützt GNOME 50.x. Gefunden: $VERSION"

[[ -d "$SOURCE" ]] ||
    die "Quellordner fehlt: $SOURCE"

[[ -s "$SOURCE/metadata.json" ]] ||
    die "metadata.json fehlt oder ist leer."

[[ -s "$SOURCE/extension.js" ]] ||
    die "extension.js fehlt oder ist leer."

python3 - "$SOURCE/metadata.json" "$UUID" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as f:
    metadata = json.load(f)

if metadata.get("uuid") != sys.argv[2]:
    raise SystemExit("UUID in metadata.json stimmt nicht.")

if "50" not in metadata.get("shell-version", []):
    raise SystemExit("GNOME 50 ist nicht als unterstützte Version eingetragen.")
PY

log "Erstelle GNOME-Erweiterungspaket ..."

gnome-extensions pack "$SOURCE" \
    --force \
    --out-dir="$WORK" ||
    die "Das GNOME-Erweiterungspaket konnte nicht erstellt werden."

[[ -s "$PACKAGE" ]] ||
    die "Das erzeugte Erweiterungspaket fehlt."

log "Installiere Erweiterungspaket ..."

mkdir -p "$(dirname "$TARGET")"

if ! gnome-extensions install --force "$PACKAGE"; then
    die "GNOME konnte das Erweiterungspaket nicht installieren."
fi

[[ -s "$TARGET/metadata.json" ]] ||
    die "GNOME hat die Erweiterung nicht am erwarteten Ziel installiert."

log "Erweiterung wurde installiert."
log "Bitte jetzt abmelden und wieder anmelden."
log "Anschließend im Terminal ausführen:"
printf '  gnome-extensions enable %s\n' "$UUID"
