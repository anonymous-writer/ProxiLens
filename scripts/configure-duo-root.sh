#!/usr/bin/env bash
set -euo pipefail
cat <<'MSG'
Diese manuelle Doppeleingabe ist deaktiviert.
Bitte Duo zuerst in ProxiLens unter Einstellungen speichern und dann ausführen:
  ./scripts/sync-duo-root.sh
So bleibt ProxiLens die zentrale Verwaltungsoberfläche. Der Root-Dienst erhält
nur eine geschützte, automatisch übertragene Kopie für die PAM-Authentifizierung.
MSG
exit 2
