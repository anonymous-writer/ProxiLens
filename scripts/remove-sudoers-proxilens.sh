#!/usr/bin/env bash
set -euo pipefail
TARGET=/etc/sudoers.d/90-proxilens
if [[ ${EUID} -ne 0 ]]; then echo "Bitte mit sudo ausführen." >&2; exit 1; fi
if [[ ! -f "$TARGET" ]] || ! grep -q '^# ProxiLens: require fresh authentication' "$TARGET"; then
  echo "ABBRUCH: Datei fehlt oder gehört nicht eindeutig ProxiLens." >&2; exit 2
fi
rm -f "$TARGET"
visudo -c
echo "ProxiLens sudoers-Regel entfernt; vorhandene sudoers-Dateien blieben unangetastet."
