#!/usr/bin/env bash
set -euo pipefail
[[ $EUID -eq 0 ]] || { echo 'Bitte mit sudo ausführen.' >&2; exit 1; }
PAM=/etc/pam.d/sudo
DROP=/etc/sudoers.d/90-proxilens
if [[ -f $PAM ]]; then sed -i '\|pam_proxilens\.so|d' "$PAM"; fi
rm -f "$DROP"
visudo -c
systemctl disable --now proxilens-policy.service || true
echo 'ProxiLens-PAM-Zeile und sudoers-Drop-in entfernt. Duo-Konfiguration bleibt erhalten.'
