#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
if [[ ! -x "$ROOT/build/proxilens-duo-sync" ]]; then
  echo 'Synchronisierungsprogramm fehlt. Bitte zuerst ./scripts/build-install-sudo-pam.sh ausführen.' >&2
  exit 2
fi
exec "$ROOT/build/proxilens-duo-sync" "$@"
