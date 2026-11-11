#!/usr/bin/env bash
set -Eeuo pipefail
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
command -v dpkg-buildpackage >/dev/null 2>&1 || {
  echo "Fehlt: dpkg-buildpackage (Paket: dpkg-dev)" >&2; exit 1;
}
command -v dh >/dev/null 2>&1 || {
  echo "Fehlt: debhelper (und debhelper-compat)" >&2; exit 1;
}
dpkg-buildpackage -us -uc -b
echo "Debian-Paket wurde im übergeordneten Verzeichnis erstellt."
