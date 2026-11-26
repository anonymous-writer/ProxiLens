#!/usr/bin/env bash
set -euo pipefail
[[ $EUID -eq 0 ]] || { echo 'Bitte mit sudo ausführen.' >&2; exit 1; }
ROOT=$(cd "$(dirname "$0")/.." && pwd)
MODULE=/usr/lib/x86_64-linux-gnu/security/pam_proxilens.so
DAEMON=/usr/libexec/proxilens/proxilens-policy
PAM=/etc/pam.d/sudo
DROP=/etc/sudoers.d/90-proxilens
[[ -f $MODULE && -f $DAEMON ]] || { echo 'PAM-Modul oder Policy-Dienst fehlt; Abbruch.' >&2; exit 2; }
[[ -f /etc/proxilens/duo.conf ]] || { echo 'Duo-Konfiguration fehlt; zuerst configure-duo-root.sh.' >&2; exit 3; }
[[ ! -e $DROP ]] || { echo "$DROP existiert bereits; Abbruch." >&2; exit 4; }
grep -q 'pam_proxilens.so' "$PAM" && { echo 'PAM ist bereits integriert; Abbruch.' >&2; exit 5; }
grep -qE '^@include[[:space:]]+common-auth' "$PAM" || { echo 'Unbekannte sudo-PAM-Struktur; keine automatische Änderung.' >&2; exit 6; }
systemctl daemon-reload
if ! systemctl enable --now proxilens-policy.service; then echo 'Policy-Dienst konnte nicht starten; PAM/sudoers bleiben unverändert.' >&2; exit 7; fi
backup="/var/backups/proxilens-sudo-pam-$(date +%Y%m%d-%H%M%S)"
install -d -o root -g root -m 0700 "$backup"
cp -a "$PAM" "$backup/sudo.pam"
if [[ -e /etc/sudoers ]]; then cp -a /etc/sudoers "$backup/sudoers"; fi
if [[ -d /etc/sudoers.d ]]; then cp -a /etc/sudoers.d "$backup/sudoers.d"; fi
TMP=$(mktemp /etc/pam.d/sudo.XXXXXX)
awk -v line='auth [success=done auth_err=die ignore=ignore default=die] pam_proxilens.so' '{ if ($0 ~ /^@include[[:space:]]+common-auth/) print line; print }' "$PAM" > "$TMP"
chown root:root "$TMP"; chmod 0644 "$TMP"; mv "$TMP" "$PAM"
install -o root -g root -m 0440 "$ROOT/packaging/sudoers/proxilens" "$DROP"
if ! visudo -c; then cp -a "$backup/sudo.pam" "$PAM"; rm -f "$DROP"; echo "Rollback ausgeführt. Sicherung: $backup" >&2; exit 8; fi
if ! systemctl is-active --quiet proxilens-policy.service; then cp -a "$backup/sudo.pam" "$PAM"; rm -f "$DROP"; echo "Dienst nicht aktiv; PAM/sudoers zurückgesetzt. Sicherung: $backup" >&2; exit 9; fi
echo "PAM und sudoers integriert. Sicherung: $backup"
echo 'WICHTIG: Aktuelles Root-Terminal offen halten und in einem zweiten Terminal sudo testen.'
