#!/usr/bin/env bash
set -Eeuo pipefail
SCRIPT_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
if [[ ! -f "${SCRIPT_ROOT}/go.mod" && -f "${SCRIPT_ROOT}/source/go.mod" ]]; then SCRIPT_ROOT="${SCRIPT_ROOT}/source"; fi
ROOT="$SCRIPT_ROOT"

[[ $EUID -eq 0 ]] || { echo 'Bitte mit sudo ausführen.' >&2; exit 1; }
PAM=/etc/pam.d/sudo
DROP=/etc/sudoers.d/90-proxilens
MODULE=/usr/lib/x86_64-linux-gnu/security/pam_proxilens.so
DAEMON=/usr/libexec/proxilens/proxilens-policy
UNIT=/usr/lib/systemd/system/proxilens-policy.service
CONFIG=/etc/proxilens/duo.conf

# Ensure the prebuilt PAM module is installed. It is built by
# install-desktop-app.sh as the desktop user, never compiled as root here.
MODULE_SOURCE="${ROOT}/pam/pam_proxilens.so"
if [[ ! -e "$MODULE" && -f "$MODULE_SOURCE" && ! -L "$MODULE_SOURCE" ]]; then
    install -d -o root -g root -m 0755 "$(dirname "$MODULE")"
    install -o root -g root -m 0644 "$MODULE_SOURCE" "$MODULE"
fi

for f in "$PAM" "$MODULE" "$DAEMON" "$UNIT" "$CONFIG"; do
  [[ -f "$f" && ! -L "$f" ]] || { echo "Erforderliche reguläre Datei fehlt/ist Symlink: $f" >&2; exit 2; }
done
[[ $(stat -c '%u:%a' "$CONFIG") == '0:600' ]] || { echo 'Duo-Konfiguration muss root gehören und Modus 0600 haben.' >&2; exit 3; }
[[ ! -e "$DROP" ]] || { echo "$DROP existiert bereits; Abbruch." >&2; exit 4; }
! grep -q 'pam_proxilens\.so' "$PAM" || { echo 'PAM ist bereits integriert; Abbruch.' >&2; exit 5; }
grep -qE '^@include[[:space:]]+common-auth([[:space:]]|$)' "$PAM" || { echo 'Unbekannte sudo-PAM-Struktur; keine automatische Änderung.' >&2; exit 6; }
command -v visudo >/dev/null || { echo 'visudo fehlt.' >&2; exit 7; }
command -v systemctl >/dev/null || { echo 'systemctl fehlt.' >&2; exit 7; }

backup="/var/backups/proxilens-sudo-pam-$(date +%Y%m%d-%H%M%S)"
install -d -o root -g root -m 0700 "$backup"
cp -a "$PAM" "$backup/sudo.pam"
[[ ! -e /etc/sudoers ]] || cp -a /etc/sudoers "$backup/sudoers"
[[ ! -d /etc/sudoers.d ]] || cp -a /etc/sudoers.d "$backup/sudoers.d"
printf '%s\n' "$(systemctl is-enabled proxilens-policy.service 2>/dev/null || true)" > "$backup/service-enabled-before"
printf '%s\n' "$(systemctl is-active proxilens-policy.service 2>/dev/null || true)" > "$backup/service-active-before"

pam_changed=0
drop_created=0
rollback() {
  rc=$?
  trap - EXIT
  if (( rc != 0 )); then
    (( pam_changed == 0 )) || cp -a "$backup/sudo.pam" "$PAM"
    (( drop_created == 0 )) || rm -f "$DROP"
    if [[ $(cat "$backup/service-active-before") != active ]]; then
      systemctl stop proxilens-policy.service >/dev/null 2>&1 || true
    fi
    if [[ $(cat "$backup/service-enabled-before") != enabled ]]; then
      systemctl disable proxilens-policy.service >/dev/null 2>&1 || true
    fi
    echo "Installation fehlgeschlagen; Änderungen zurückgerollt. Sicherung: $backup" >&2
  fi
  exit "$rc"
}
trap rollback EXIT

# Build the prospective PAM file and validate it is syntactically non-empty before installation.
tmp=$(mktemp /etc/pam.d/sudo.proxilens.XXXXXX)
awk -v line='auth [success=done auth_err=die ignore=ignore default=die] pam_proxilens.so' \
  '{ if ($0 ~ /^@include[[:space:]]+common-auth([[:space:]]|$)/) print line; print }' "$PAM" > "$tmp"
chown root:root "$tmp"
chmod 0644 "$tmp"
grep -q 'pam_proxilens\.so' "$tmp" || { rm -f "$tmp"; echo 'PAM-Vorbereitung fehlgeschlagen.' >&2; exit 8; }

systemctl daemon-reload
systemctl enable --now proxilens-policy.service
systemctl is-active --quiet proxilens-policy.service || { rm -f "$tmp"; echo 'Policy-Dienst ist nicht aktiv.' >&2; exit 9; }
install -o root -g root -m 0440 "$(dirname "$0")/../packaging/sudoers/proxilens" "$DROP"
drop_created=1
if ! visudo -c; then rm -f "$tmp"; echo 'sudoers-Prüfung fehlgeschlagen.' >&2; exit 10; fi

# PAM is the final activation step. The explicit P choice returns PAM_IGNORE,
# which continues into the preserved common-auth include. Duo errors die closed.
pam_changed=1
mv -f "$tmp" "$PAM"
chmod 0644 "$PAM"
trap - EXIT
echo "ProxiLens sudo-PAM integriert. Sicherung: $backup"
echo 'Duo Push ist Standard; nur die explizite Wahl P führt zu Ubuntu common-auth.'
echo 'Ein offenes Root-Terminal behalten und in einem zweiten Terminal testen.'
