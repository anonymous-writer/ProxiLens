#!/usr/bin/env bash
set -Eeuo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
AGENT_NAME="proxilens-wake-agent"
SERVICE_NAME="${AGENT_NAME}.service"
BIN_DIR="${HOME}/.local/bin"
SERVICE_DIR="${HOME}/.config/systemd/user"
TARGET_BIN="${BIN_DIR}/${AGENT_NAME}"
SERVICE_SOURCE="${ROOT}/scripts/systemd/${SERVICE_NAME}"
SERVICE_TARGET="${SERVICE_DIR}/${SERVICE_NAME}"
BUILD_TMP=""
SERVICE_TMP=""
BACKUP_BIN=""

log() { printf '[ProxiLens] %s\n' "$*"; }
die() { printf '[ProxiLens] ERROR: %s\n' "$*" >&2; exit 1; }

command -v go >/dev/null 2>&1 || die "Go is not installed. Install Go 1.27 or newer."
command -v dbus-monitor >/dev/null 2>&1 || die "dbus-monitor is missing. Install the dbus-bin package."
command -v systemctl >/dev/null 2>&1 || die "systemctl was not found."
[[ -f "$ROOT/go.mod" ]] || die "go.mod was not found in the project root."
[[ -f "$ROOT/cmd/proxilens-wake-agent/main.go" ]] || die "Wake Agent source is missing."
[[ -s "$SERVICE_SOURCE" ]] || die "systemd service file is missing or empty."

REQUIRED_GO="$(awk '$1 == "go" { print $2; exit }' "$ROOT/go.mod")"
[[ -n "$REQUIRED_GO" ]] || die "Could not determine the required Go version from go.mod."
INSTALLED_GO="$(go env GOVERSION 2>/dev/null | sed 's/^go//')"
[[ -n "$INSTALLED_GO" ]] || die "Could not determine the installed Go version."

version_ge() {
  [[ "$(printf '%s\n%s\n' "$1" "$2" | sort -V | head -n1)" == "$2" ]]
}
version_ge "$INSTALLED_GO" "$REQUIRED_GO" || \
  die "Go ${REQUIRED_GO} or newer is required; detected Go ${INSTALLED_GO}."

# Build to a temporary file first. A failed build leaves the running agent untouched.
install -d -m 0755 "$BIN_DIR" "$SERVICE_DIR"
BUILD_TMP="$(mktemp "${BIN_DIR}/.${AGENT_NAME}.new.XXXXXX")"
BACKUP_BIN="${BIN_DIR}/.${AGENT_NAME}.backup.$$"
cleanup() {
  [[ -z "$BUILD_TMP" ]] || rm -f -- "$BUILD_TMP"
  [[ -z "$SERVICE_TMP" ]] || rm -f -- "$SERVICE_TMP"
  if [[ -n "$BACKUP_BIN" && -e "$BACKUP_BIN" && ! -e "$TARGET_BIN" ]]; then
    mv -- "$BACKUP_BIN" "$TARGET_BIN"
  else
    [[ -z "$BACKUP_BIN" ]] || rm -f -- "$BACKUP_BIN"
  fi
}
trap cleanup EXIT

cd "$ROOT"
log "Building Wake Agent with Go ${INSTALLED_GO} ..."
go build -trimpath -o "$BUILD_TMP" ./cmd/proxilens-wake-agent
[[ -x "$BUILD_TMP" ]] || die "The Wake Agent build did not produce an executable."

SERVICE_TMP="$(mktemp "${SERVICE_DIR}/.${SERVICE_NAME}.new.XXXXXX")"
install -m 0644 "$SERVICE_SOURCE" "$SERVICE_TMP"
grep -Fq "ExecStart=%h/.local/bin/${AGENT_NAME}" "$SERVICE_TMP" || {
  rm -f -- "$SERVICE_TMP"
  die "The service file ExecStart does not match the installation path."
}

# Stop before replacing the executable; keep a backup for rollback.
systemctl --user disable --now "$SERVICE_NAME" >/dev/null 2>&1 || true
if [[ -e "$TARGET_BIN" ]]; then
  mv -- "$TARGET_BIN" "$BACKUP_BIN"
fi
mv -- "$BUILD_TMP" "$TARGET_BIN"
BUILD_TMP=""
chmod 0755 "$TARGET_BIN"

SERVICE_BACKUP=""
if [[ -e "$SERVICE_TARGET" ]]; then
  SERVICE_BACKUP="${SERVICE_DIR}/.${SERVICE_NAME}.backup.$$"
  mv -- "$SERVICE_TARGET" "$SERVICE_BACKUP"
fi
mv -- "$SERVICE_TMP" "$SERVICE_TARGET"
SERVICE_TMP=""

if ! systemctl --user daemon-reload || ! systemctl --user enable --now "$SERVICE_NAME"; then
  systemctl --user disable --now "$SERVICE_NAME" >/dev/null 2>&1 || true
  rm -f -- "$TARGET_BIN" "$SERVICE_TARGET"
  [[ ! -e "$BACKUP_BIN" ]] || mv -- "$BACKUP_BIN" "$TARGET_BIN"
  [[ -z "$SERVICE_BACKUP" || ! -e "$SERVICE_BACKUP" ]] || mv -- "$SERVICE_BACKUP" "$SERVICE_TARGET"
  systemctl --user daemon-reload >/dev/null 2>&1 || true
  systemctl --user start "$SERVICE_NAME" >/dev/null 2>&1 || true
  die "Could not activate the service. The previous files were restored when available."
fi

[[ -z "$BACKUP_BIN" ]] || rm -f -- "$BACKUP_BIN"
[[ -z "${SERVICE_BACKUP:-}" ]] || rm -f -- "$SERVICE_BACKUP"
BACKUP_BIN=""
trap - EXIT
log "Wake Agent installed and enabled."
log "Status: systemctl --user status ${SERVICE_NAME}"
log "Log: journalctl --user -u ${SERVICE_NAME} -f"
