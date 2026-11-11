#!/usr/bin/env bash
set -Eeuo pipefail

APP_NAME="ProxiLens"
APP_ID="proxilens"
BIN_NAME="proxilens-ui"
ICON_NAME="proxilens"

BIN_DIR="${HOME}/.local/bin"
APP_DIR="${HOME}/.local/share/applications"
ICON_DIR="${HOME}/.local/share/icons/hicolor/512x512/apps"

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd -- "${SCRIPT_DIR}/.." && pwd)"

log() { printf '[%s] %s\n' "$APP_NAME" "$*"; }
die() { printf '[%s] FEHLER: %s\n' "$APP_NAME" "$*" >&2; exit 1; }

# Locate the UI executable in common project/build/download locations.
BIN_SOURCE=""
for candidate in \
    "${PROJECT_ROOT}/${BIN_NAME}" \
    "${PROJECT_ROOT}/dist/${BIN_NAME}" \
    "${PROJECT_ROOT}/build/${BIN_NAME}" \
    "${HOME}/Downloads/${BIN_NAME}"
do
    if [[ -f "$candidate" && -x "$candidate" ]]; then
        BIN_SOURCE="$candidate"
        break
    fi
done

if [[ -z "$BIN_SOURCE" ]]; then
    die "Programm '${BIN_NAME}' nicht gefunden. Lege die ausführbare Datei in das Projekt-Hauptverzeichnis, nach dist/ oder build/, oder nach ~/Downloads/."
fi

# Locate the custom ProxiLens app icon. The personal WLM logo is intentionally not used.
ICON_SOURCE=""
for candidate in \
    "${PROJECT_ROOT}/static/icons/proxilens-512px.png" \
    "${PROJECT_ROOT}/static/icons/icon-512.png"
do
    if [[ -f "$candidate" ]]; then
        ICON_SOURCE="$candidate"
        break
    fi
done

[[ -n "$ICON_SOURCE" ]] || die "ProxiLens-App-Icon nicht gefunden. Erwartet wird proxilens-512px.png im Icon-Verzeichnis."

log "Installiere ${BIN_NAME} nach ${BIN_DIR} ..."
install -d -m 755 "$BIN_DIR"
install -m 755 "$BIN_SOURCE" "${BIN_DIR}/${BIN_NAME}"

log "Installiere App-Icon ..."
install -d -m 755 "$ICON_DIR"
install -m 644 "$ICON_SOURCE" "${ICON_DIR}/${ICON_NAME}.png"

log "Erstelle Ubuntu-Anwendungseintrag ..."
install -d -m 755 "$APP_DIR"
cat > "${APP_DIR}/${APP_ID}.desktop" <<EOF
[Desktop Entry]
Version=1.0
Type=Application
Name=ProxiLens
GenericName=Session Monitor
Comment=Session Monitor for Authelia and Duo
Exec=${BIN_DIR}/${BIN_NAME}
Icon=${ICON_NAME}
Terminal=false
Categories=Utility;Security;
StartupNotify=true
EOF
chmod 644 "${APP_DIR}/${APP_ID}.desktop"

if command -v update-desktop-database >/dev/null 2>&1; then
    update-desktop-database "$APP_DIR" >/dev/null 2>&1 || true
fi
if command -v gtk-update-icon-cache >/dev/null 2>&1; then
    gtk-update-icon-cache -f -t "${HOME}/.local/share/icons/hicolor" >/dev/null 2>&1 || true
fi

log "Installation abgeschlossen."
log "ProxiLens ist in der Ubuntu-Anwendungsübersicht verfügbar."
log "Der Wake-Agent und die GNOME-Erweiterung wurden von diesem Skript nicht verändert."
log "Starten: ${BIN_DIR}/${BIN_NAME}"
