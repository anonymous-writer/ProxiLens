#!/usr/bin/env bash
set -Eeuo pipefail

[[ $EUID -eq 0 ]] || { echo "Bitte mit sudo ausführen." >&2; exit 1; }

TARGET=/etc/pam.d/polkit-1
if [[ ! -f "$TARGET" ]] || ! grep -q 'pam_proxilens\.so' "$TARGET"; then
  echo "Keine ProxiLens-Polkit-PAM-Überschreibung gefunden."
  exit 0
fi

backup="/var/backups/proxilens-polkit-remove-$(date +%Y%m%d-%H%M%S)"
install -d -o root -g root -m 0700 "$backup"
cp -a "$TARGET" "$backup/polkit-1.installed"
rm -f "$TARGET"

echo "ProxiLens-Polkit-PAM-Überschreibung entfernt."
echo "Die Paketdatei /usr/lib/pam.d/polkit-1 ist wieder wirksam."
echo "Sicherung: $backup"
echo "Der sudo-PAM-Stack und sudoers wurden nicht verändert."
