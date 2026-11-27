#!/usr/bin/env bash
set -Eeuo pipefail
AGENT_NAME="proxilens-wake-agent"
SERVICE_NAME="${AGENT_NAME}.service"
BIN="${HOME}/.local/bin/${AGENT_NAME}"
SERVICE="${HOME}/.config/systemd/user/${SERVICE_NAME}"

systemctl --user disable --now "$SERVICE_NAME" >/dev/null 2>&1 || true
rm -f -- "$SERVICE" "$BIN"
systemctl --user daemon-reload
echo "ProxiLens Wake-Agent wurde entfernt."
