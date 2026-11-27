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
STATE_DIR=/var/lib/proxilens/polkit-pam
ORIGINAL="$STATE_DIR/polkit-1.original"
MARKER="$STATE_DIR/managed"

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

# An already managed installation is a successful no-op.
if [[ -f "$TARGET" ]] && grep -q 'pam_proxilens\.so' "$TARGET"; then
  echo "ProxiLens-Polkit-PAM ist bereits installiert."
  exit 0
fi

# Use the actual active /etc file as the source, preserving local changes.
if [[ -e "$TARGET" ]]; then
  [[ -f "$TARGET" && ! -L "$TARGET" ]] || {
    echo "$TARGET ist keine reguläre Datei; Abbruch." >&2
    exit 6
  }
  BASE_FILE="$TARGET"
  NEED_RESTORE=1
else
  BASE_FILE="$SOURCE"
  NEED_RESTORE=0
fi

grep -qE '^@include[[:space:]]+common-auth([[:space:]]|$)' "$BASE_FILE" || {
  echo "Unbekannte Polkit-PAM-Struktur; Abbruch." >&2
  exit 7
}

# Do not replace an earlier backup: it is the authoritative pre-ProxiLens file.
if [[ -e "$STATE_DIR" ]]; then
  [[ -d "$STATE_DIR" && -f "$ORIGINAL" && -f "$MARKER" ]] || {
    echo "Vorhandener ProxiLens-Sicherungszustand ist unvollständig; manuelle Prüfung erforderlich." >&2
    exit 8
  }
else
  install -d -o root -g root -m 0700 "$STATE_DIR"
  cp -a "$BASE_FILE" "$ORIGINAL"
  printf '%s\n' "$NEED_RESTORE" > "$MARKER"
  chown root:root "$MARKER"
  chmod 0600 "$MARKER"
fi

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
' "$BASE_FILE" > "$tmp" || {
  echo "Polkit-PAM-Vorbereitung fehlgeschlagen." >&2
  exit 9
}

chown root:root "$tmp"
chmod 0644 "$tmp"
grep -q 'pam_proxilens\.so' "$tmp" || {
  echo "PAM-Vorbereitung fehlgeschlagen." >&2
  exit 10
}

install -o root -g root -m 0644 "$tmp" "$TARGET"
echo "Polkit-PAM-Datei installiert: $TARGET"
echo "Ursprüngliche lokale PAM-Datei gesichert unter: $ORIGINAL"
echo "sudo-PAM und sudoers wurden nicht verändert."
echo "Rücknahme: sudo ./scripts/remove-polkit-pam-proxilens.sh"
