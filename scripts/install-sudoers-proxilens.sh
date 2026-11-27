#!/usr/bin/env bash
set -euo pipefail

# Deliberately refuses to activate sudo policy until the real PAM module and
# privileged authorizer are installed. It never edits /etc/sudoers directly.
TARGET=/etc/sudoers.d/90-proxilens
SOURCE="$(cd "$(dirname "$0")/.." && pwd)/packaging/sudoers/proxilens"
PAM_MODULE=/usr/lib/x86_64-linux-gnu/security/pam_proxilens.so
SERVICE=/usr/lib/systemd/system/proxilens-policy.service

if [[ ${EUID} -ne 0 ]]; then echo "Bitte mit sudo ausführen." >&2; exit 1; fi
if [[ ! -f "$PAM_MODULE" ]]; then
  echo "ABBRUCH: $PAM_MODULE fehlt. Die produktive sudoers-Regel wird nicht aktiviert." >&2
  exit 2
fi
if [[ ! -f "$SERVICE" ]] || ! systemctl is-active --quiet proxilens-policy.service; then
  echo "ABBRUCH: Der echte ProxiLens-Policy-Dienst ist nicht installiert und aktiv." >&2
  exit 3
fi
if [[ -e "$TARGET" ]]; then
  echo "ABBRUCH: $TARGET existiert bereits; keine Datei wird überschrieben." >&2
  exit 4
fi

backup="/var/backups/proxilens-sudoers-$(date +%Y%m%d-%H%M%S)"
install -d -m 0700 "$backup"
cp -a /etc/sudoers "$backup/sudoers"
if [[ -d /etc/sudoers.d ]]; then cp -a /etc/sudoers.d "$backup/sudoers.d"; fi
install -o root -g root -m 0440 "$SOURCE" "$TARGET"
if ! visudo -c; then
  rm -f "$TARGET"
  echo "Syntaxprüfung fehlgeschlagen; ProxiLens-Datei entfernt. Sicherung: $backup" >&2
  exit 5
fi
echo "ProxiLens sudoers-Regel installiert. Sicherung: $backup"
