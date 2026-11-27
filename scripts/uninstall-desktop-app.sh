#!/usr/bin/env bash
set -Eeuo pipefail

APP_NAME="ProxiLens"
BIN_NAME="proxilens-ui"
APP_ID="proxilens"
UNIT_NAME="proxilens-grant-broker.service"

BIN_DIR="${HOME}/.local/bin"
APP_DIR="${HOME}/.local/share/applications"
ICON_DIR="${HOME}/.local/share/icons/hicolor/512x512/apps"
CONFIG_HOME="${XDG_CONFIG_HOME:-${HOME}/.config}"
APP_CONFIG_DIR="${CONFIG_HOME}/proxilens"

PURGE_DATA=0
DRY_RUN=0

usage() {
  cat <<'EOF'
ProxiLens-Deinstallation (manuelle Benutzerinstallation)

Aufruf:
  ./scripts/uninstall-desktop-app.sh
  ./scripts/uninstall-desktop-app.sh --purge-data
  ./scripts/uninstall-desktop-app.sh --dry-run [--purge-data]

Optionen:
  --purge-data  Entfernt zusätzlich ~/.config/proxilens (bzw. XDG_CONFIG_HOME/proxilens).
                Eine außerhalb dieses Ordners gespeicherte Datenbank bleibt erhalten.
                Der GNOME-Schlüsselbund-Eintrag wird absichtlich nicht automatisch gelöscht.
  --dry-run     Zeigt geplante Aktionen, ohne Dateien zu verändern.
  -h, --help    Diese Hilfe anzeigen.

Für eine Installation über ein Debian/Ubuntu-Paket stattdessen:
  sudo apt remove proxilens
  sudo apt purge proxilens

Dieses Skript entfernt keine PAM-, sudoers-, Polkit- oder systemweiten
Policy-Konfigurationen. Solche Integrationen sind sicherheitskritisch und
müssen separat mit den passenden ProxiLens-Rücknahme-Skripten entfernt werden.
EOF
}

log() { printf '[%s] %s\n' "$APP_NAME" "$*"; }
run() {
  if (( DRY_RUN )); then
    printf '[DRY-RUN]'; printf ' %q' "$@"; printf '\n'
  else
    "$@"
  fi
}

while (($#)); do
  case "$1" in
    --purge-data) PURGE_DATA=1 ;;
    --dry-run) DRY_RUN=1 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unbekannte Option: $1" >&2; usage >&2; exit 2 ;;
  esac
  shift
done

[[ -n "${HOME:-}" && "$HOME" != "/" ]] || {
  echo "Ungültiges HOME-Verzeichnis; Abbruch." >&2
  exit 2
}

log "Beende den ProxiLens-Benutzerdienst, falls er aktiviert ist."
if command -v systemctl >/dev/null 2>&1; then
  if systemctl --user is-enabled --quiet "$UNIT_NAME" 2>/dev/null; then
    run systemctl --user disable --now "$UNIT_NAME"
  elif systemctl --user is-active --quiet "$UNIT_NAME" 2>/dev/null; then
    run systemctl --user stop "$UNIT_NAME"
  fi
  if (( DRY_RUN )); then
    run systemctl --user daemon-reload
  else
    systemctl --user daemon-reload >/dev/null 2>&1 || true
  fi
fi

log "Entferne ausschließlich die von der manuellen Desktop-Installation angelegten Dateien."
for file in \
  "${BIN_DIR}/${BIN_NAME}" \
  "${APP_DIR}/${APP_ID}.desktop" \
  "${ICON_DIR}/${APP_ID}.png"
do
  if [[ -e "$file" || -L "$file" ]]; then
    run rm -f -- "$file"
    log "Entfernt: $file"
  fi
done

if (( PURGE_DATA )); then
  if [[ -d "$APP_CONFIG_DIR" && ! -L "$APP_CONFIG_DIR" ]]; then
    log "Entferne ProxiLens-Konfigurations- und Datenbankordner: $APP_CONFIG_DIR"
    run rm -rf -- "$APP_CONFIG_DIR"
  else
    log "Kein regulärer ProxiLens-Konfigurationsordner gefunden."
  fi
else
  log "Benutzerdaten bleiben erhalten: $APP_CONFIG_DIR"
fi

if command -v update-desktop-database >/dev/null 2>&1 && [[ -d "$APP_DIR" ]]; then
  run update-desktop-database "$APP_DIR"
fi
if command -v gtk-update-icon-cache >/dev/null 2>&1; then
  icon_theme_dir="${HOME}/.local/share/icons/hicolor"
  if [[ -d "$icon_theme_dir" ]]; then
    run gtk-update-icon-cache -f -t "$icon_theme_dir"
  fi
fi

# Remove only empty directories created by the desktop installer.
for dir in "$ICON_DIR" "$APP_DIR" "$BIN_DIR"; do
  if [[ -d "$dir" && ! -L "$dir" ]]; then
    run rmdir --ignore-fail-on-non-empty -- "$dir" 2>/dev/null || true
  fi
done

log "Desktop-Deinstallation abgeschlossen."
if (( ! PURGE_DATA )); then
  log "Für das zusätzliche Löschen der Standarddatenbank erneut mit --purge-data ausführen."
fi
log "Der GNOME-Schlüsselbund-Eintrag und benutzerdefinierte Datenbankpfade wurden nicht verändert."
log "Systemweite PAM/sudoers/Polkit-Integrationen wurden nicht verändert."
