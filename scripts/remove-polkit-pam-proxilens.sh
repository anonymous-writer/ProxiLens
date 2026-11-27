#!/usr/bin/env bash
set -Eeuo pipefail

[[ $EUID -eq 0 ]] || { echo "Bitte mit sudo ausführen." >&2; exit 1; }

TARGET=/etc/pam.d/polkit-1
STATE_DIR=/var/lib/proxilens/polkit-pam
ORIGINAL="$STATE_DIR/polkit-1.original"
MARKER="$STATE_DIR/managed"

if [[ ! -f "$TARGET" ]] || ! grep -q 'pam_proxilens\.so' "$TARGET"; then
  echo "Keine aktive ProxiLens-Polkit-PAM-Regel gefunden."
  exit 0
fi

# Never delete the active PAM file unless its exact pre-installation copy exists.
if [[ ! -f "$ORIGINAL" || ! -f "$MARKER" ]]; then
  echo "Sicherung der ursprünglichen Polkit-PAM-Datei fehlt. Aus Sicherheitsgründen wird nichts verändert." >&2
  exit 2
fi

restore_mode=$(cat "$MARKER")
[[ "$restore_mode" == "0" || "$restore_mode" == "1" ]] || {
  echo "Sicherungsmarker ist ungültig; Abbruch." >&2
  exit 3
}

backup="/var/backups/proxilens-polkit-remove-$(date +%Y%m%d-%H%M%S)"
install -d -o root -g root -m 0700 "$backup"
cp -a "$TARGET" "$backup/polkit-1.installed"
cp -a "$ORIGINAL" "$backup/polkit-1.original"

if [[ "$restore_mode" == "1" ]]; then
  tmp=$(mktemp /etc/pam.d/polkit-1.restore.XXXXXX)
  trap 'rm -f "$tmp"' EXIT
  cp -a "$ORIGINAL" "$tmp"
  chown --reference="$ORIGINAL" "$tmp"
  chmod --reference="$ORIGINAL" "$tmp"
  mv -f "$tmp" "$TARGET"
  echo "Ursprüngliche lokale Polkit-PAM-Datei wiederhergestellt."
else
  rm -f "$TARGET"
  echo "Lokale Überschreibung entfernt; die Vendor-Datei ist wieder wirksam."
fi

rm -rf -- "$STATE_DIR"
echo "Sicherung der installierten und ursprünglichen Dateien: $backup"
echo "Der sudo-PAM-Stack und sudoers wurden nicht verändert."
