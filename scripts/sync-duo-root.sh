#!/usr/bin/env bash
set -euo pipefail
if [[ -x /usr/bin/proxilens-duo-sync ]]; then
  exec /usr/bin/proxilens-duo-sync "$@"
fi
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
if [[ -x "$ROOT/build/proxilens-duo-sync" ]]; then
  exec "$ROOT/build/proxilens-duo-sync" "$@"
fi
echo 'ProxiLens Duo-Synchronisierungsprogramm fehlt. Bitte ProxiLens über das DEB-Paket installieren oder den Projekt-Build erstellen.' >&2
exit 2
