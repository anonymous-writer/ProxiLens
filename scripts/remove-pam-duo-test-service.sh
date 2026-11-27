#!/usr/bin/env bash
set -euo pipefail
SERVICE=/etc/pam.d/proxilens-duo-test
[[ $EUID -eq 0 ]] || { echo "Bitte mit sudo starten." >&2; exit 1; }
[[ -f "$SERVICE" ]] || { echo "Kein ProxiLens-Testdienst vorhanden."; exit 0; }
grep -q 'ProxiLens isolated Duo test service' "$SERVICE" || { echo "Datei hat keinen ProxiLens-Marker; wird nicht entfernt." >&2; exit 1; }
rm -- "$SERVICE"
echo "Isolierter ProxiLens-Duo-PAM-Testdienst entfernt."
