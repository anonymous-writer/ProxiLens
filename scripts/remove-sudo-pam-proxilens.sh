#!/usr/bin/env bash
set -Eeuo pipefail
[[ $EUID -eq 0 ]] || { echo 'Bitte mit sudo ausführen.' >&2; exit 1; }
PAM=/etc/pam.d/sudo
DROP=/etc/sudoers.d/90-proxilens
backup="/var/backups/proxilens-remove-$(date +%Y%m%d-%H%M%S)"
install -d -o root -g root -m 0700 "$backup"
[[ ! -e "$PAM" ]] || cp -a "$PAM" "$backup/sudo.pam"
[[ ! -e "$DROP" ]] || cp -a "$DROP" "$backup/90-proxilens"
if [[ -f "$PAM" ]] && grep -q 'pam_proxilens\.so' "$PAM"; then
  tmp=$(mktemp /etc/pam.d/sudo.XXXXXX)
  sed '\|pam_proxilens\.so|d' "$PAM" > "$tmp"
  chown root:root "$tmp"; chmod 0644 "$tmp"; mv "$tmp" "$PAM"
fi
rm -f "$DROP"
if ! visudo -c; then
  [[ ! -e "$backup/sudo.pam" ]] || cp -a "$backup/sudo.pam" "$PAM"
  [[ ! -e "$backup/90-proxilens" ]] || cp -a "$backup/90-proxilens" "$DROP"
  echo "sudoers-Prüfung fehlgeschlagen; vorheriger Stand wiederhergestellt. Sicherung: $backup" >&2
  exit 2
fi
if [[ -f /etc/pam.d/polkit-1 ]] && grep -q 'pam_proxilens\.so' /etc/pam.d/polkit-1; then
  echo 'Polkit-PAM ist weiterhin aktiv; Policy-Dienst bleibt für Polkit eingeschaltet.'
else
  systemctl disable --now proxilens-policy.service || true
fi
echo "ProxiLens-sudo-Integration entfernt. Sicherung: $backup"
echo 'Die Duo-Konfiguration bleibt erhalten.'
