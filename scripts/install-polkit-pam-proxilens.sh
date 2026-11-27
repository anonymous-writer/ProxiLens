#!/usr/bin/env bash
set -Eeuo pipefail

[[ $EUID -eq 0 ]] || { echo "Bitte mit sudo ausführen." >&2; exit 1; }
ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
if [[ ! -f "${ROOT}/go.mod" && -f "${ROOT}/source/go.mod" ]]; then ROOT="${ROOT}/source"; fi

SOURCE=/usr/lib/pam.d/polkit-1
TARGET=/etc/pam.d/polkit-1
MODULE=/usr/lib/x86_64-linux-gnu/security/pam_proxilens.so
DAEMON=/usr/libexec/proxilens/proxilens-policy
CONFIG=/etc/proxilens/duo.conf

# Ensure the prebuilt PAM module is installed. It is built by
# install-desktop-app.sh as the desktop user, never compiled as root here.
MODULE_SOURCE="${ROOT}/pam/pam_proxilens.so"
if [[ ! -e "$MODULE" && -f "$MODULE_SOURCE" && ! -L "$MODULE_SOURCE" ]]; then
    install -d -o root -g root -m 0755 "$(dirname "$MODULE")"
    install -o root -g root -m 0644 "$MODULE_SOURCE" "$MODULE"
fi

for f in "$SOURCE" "$MODULE" "$DAEMON" "$CONFIG"; do
  [[ -f "$f" && ! -L "$f" ]] || {
    echo "Erforderliche reguläre Datei fehlt/ist Symlink: $f" >&2
    exit 2
  }
done

[[ $(stat -c '%u:%a' "$CONFIG") == "0:600" ]] || {
  echo "Duo-Konfiguration muss root gehören und Modus 0600 haben." >&2
  exit 3
}
getent group polkitd >/dev/null || { echo "Systemgruppe polkitd fehlt." >&2; exit 4; }
systemctl daemon-reload
systemctl enable --now proxilens-policy.service || {
  echo "Policy-Dienst konnte nicht gestartet werden." >&2
  exit 5
}
systemctl is-active --quiet proxilens-policy.service || {
  echo "Policy-Dienst ist nicht aktiv." >&2
  exit 5
}
SOCKET=/run/proxilens-policy/auth.sock
[[ -S "$SOCKET" ]] || { echo "Policy-Socket fehlt." >&2; exit 5; }
[[ $(stat -c '%U:%G:%a' "$SOCKET") == "root:polkitd:660" ]] || {
  echo "Policy-Socket muss root:polkitd mit Modus 660 gehören. Erst den aktualisierten Policy-Dienst installieren und starten." >&2
  exit 5
}
[[ ! -e "$TARGET" ]] || {
  echo "$TARGET existiert bereits; keine Datei wird überschrieben." >&2
  exit 6
}
grep -qE '^@include[[:space:]]+common-auth([[:space:]]|$)' "$SOURCE" || {
  echo "Unbekannte Polkit-PAM-Struktur; Abbruch." >&2
  exit 7
}

backup="/var/backups/proxilens-polkit-$(date +%Y%m%d-%H%M%S)"
install -d -o root -g root -m 0700 "$backup"
cp -a "$SOURCE" "$backup/polkit-1.vendor"

tmp=$(mktemp /etc/pam.d/polkit-1.proxilens.XXXXXX)
cleanup() { rm -f "$tmp"; }
trap cleanup EXIT

awk -v line='auth [success=done auth_err=die ignore=ignore default=die] pam_proxilens.so' '
  BEGIN { inserted=0 }
  /^@include[[:space:]]+common-auth([[:space:]]|$)/ && !inserted {
    print line
    inserted=1
  }
  { print }
  END { if (!inserted) exit 20 }
' "$SOURCE" > "$tmp" || {
  echo "Polkit-PAM-Vorbereitung fehlgeschlagen." >&2
  exit 8
}

chown root:root "$tmp"
chmod 0644 "$tmp"
grep -q 'pam_proxilens\.so' "$tmp" || {
  echo "PAM-Vorbereitung fehlgeschlagen." >&2
  exit 9
}

# Install only an /etc override; the vendor file remains untouched.
install -o root -g root -m 0644 "$tmp" "$TARGET"
echo "Polkit-PAM-Datei installiert: $TARGET"
echo "Paketvorlage gesichert unter: $backup"
echo "sudo-PAM und sudoers wurden nicht verändert."
echo "Rücknahme: sudo ./scripts/remove-polkit-pam-proxilens.sh"
