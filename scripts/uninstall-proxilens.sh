#!/usr/bin/env bash
set -Eeuo pipefail

UUID="proxilens-input@local"
AGENT_NAME="proxilens-wake-agent"
SERVICE_NAME="${AGENT_NAME}.service"
APP_ID="proxilens"
ICON_DIR="${HOME}/.local/share/icons/hicolor/512x512/apps"
APP_DIR="${HOME}/.local/share/applications"
BIN_DIR="${HOME}/.local/bin"
EXTENSION_DIR="${HOME}/.local/share/gnome-shell/extensions/${UUID}"
SERVICE_DIR="${HOME}/.config/systemd/user"
CONFIG_DIR="${HOME}/.config/proxilens"

REMOVE_DESKTOP=false
REMOVE_CONFIG=false
ASSUME_YES=false

usage() {
  cat <<'EOF'
Usage: uninstall-proxilens.sh [options]

Removes the ProxiLens GNOME extension and Wake Agent service/binary.
By default, the desktop UI installation and user configuration/database
are preserved.

Options:
  --with-desktop   Also remove the ProxiLens desktop launcher, UI binary and icon.
  --remove-config  Also permanently delete ~/.config/proxilens (settings/database).
  --yes            Do not ask for confirmation.
  -h, --help       Show this help.
EOF
}

for arg in "$@"; do
  case "$arg" in
    --with-desktop) REMOVE_DESKTOP=true ;;
    --remove-config) REMOVE_CONFIG=true ;;
    --yes) ASSUME_YES=true ;;
    -h|--help) usage; exit 0 ;;
    *) printf 'Unknown option: %s\n' "$arg" >&2; usage >&2; exit 2 ;;
  esac
done

log() { printf '[ProxiLens] %s\n' "$*"; }

if [[ "$ASSUME_YES" != true ]]; then
  echo "This will remove the ProxiLens GNOME extension and Wake Agent."
  [[ "$REMOVE_DESKTOP" != true ]] || echo "The desktop launcher, UI binary and icon will also be removed."
  [[ "$REMOVE_CONFIG" != true ]] || echo "WARNING: ProxiLens settings and database will also be permanently deleted."
  read -r -p "Continue? [y/N] " answer
  [[ "$answer" =~ ^[Yy]$ ]] || { echo "Cancelled."; exit 0; }
fi

if command -v systemctl >/dev/null 2>&1; then
  systemctl --user disable --now "$SERVICE_NAME" >/dev/null 2>&1 || true
fi
rm -f -- "${SERVICE_DIR}/${SERVICE_NAME}" "${BIN_DIR}/${AGENT_NAME}"

if command -v gnome-extensions >/dev/null 2>&1; then
  gnome-extensions disable "$UUID" >/dev/null 2>&1 || true
fi
rm -rf -- "$EXTENSION_DIR"

if [[ "$REMOVE_DESKTOP" == true ]]; then
  rm -f -- "${APP_DIR}/${APP_ID}.desktop"
  rm -f -- "${BIN_DIR}/proxilens-ui"
  rm -f -- "${ICON_DIR}/proxilens.png"
  if command -v update-desktop-database >/dev/null 2>&1; then
    update-desktop-database "$APP_DIR" >/dev/null 2>&1 || true
  fi
  if command -v gtk-update-icon-cache >/dev/null 2>&1; then
    gtk-update-icon-cache -f -t "${HOME}/.local/share/icons/hicolor" >/dev/null 2>&1 || true
  fi
fi

if [[ "$REMOVE_CONFIG" == true ]]; then
  rm -rf -- "$CONFIG_DIR"
fi

if command -v systemctl >/dev/null 2>&1; then
  systemctl --user daemon-reload >/dev/null 2>&1 || true
fi

log "GNOME extension and Wake Agent removed."
[[ "$REMOVE_DESKTOP" != true ]] || log "Desktop launcher, UI binary and icon removed."
[[ "$REMOVE_CONFIG" != true ]] || log "User configuration and database removed."
[[ "$REMOVE_CONFIG" == true ]] || log "User configuration and database were preserved at ${CONFIG_DIR}."
log "A GNOME logout/login may be required to fully refresh the lock-screen UI."
