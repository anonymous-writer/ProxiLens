#!/usr/bin/env bash
set -euo pipefail

# Read-only preflight. This script does not install packages or modify the host.
fail=0
check() {
  local label="$1"; shift
  if "$@" >/dev/null 2>&1; then printf '[OK]   %s\n' "$label"; else printf '[HINWEIS] %s\n' "$label"; fail=1; fi
}

printf '%s\n' 'ProxiLens integration preflight (read-only)' '------------------------------------------'
check 'systemd verfügbar' command -v systemctl
check 'loginctl verfügbar' command -v loginctl
check 'D-Bus system socket vorhanden' test -S /run/dbus/system_bus_socket
check 'PAM development headers vorhanden' test -f /usr/include/security/pam_modules.h
check 'Polkit CLI verfügbar' command -v pkaction
check 'sudoers syntax currently valid' sudo -n visudo -c
check 'logind erreichbar' loginctl list-sessions --no-legend
printf '\nHinweis: [HINWEIS] bedeutet nur, dass die Voraussetzung nicht nachgewiesen wurde.\n'
printf '%s\n' 'Es wurden keine Systemdateien verändert.'
exit 0
