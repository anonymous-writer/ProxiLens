#!/usr/bin/env bash
set -euo pipefail
SERVICE=/etc/pam.d/proxilens-duo-test
MODULE=""

[[ $EUID -eq 0 ]] || { echo "Bitte mit sudo starten." >&2; exit 1; }
[[ ! -e "$SERVICE" ]] || { echo "$SERVICE existiert bereits; nichts wird überschrieben." >&2; exit 1; }

for p in /usr/lib/*/security/pam_duo.so /lib/*/security/pam_duo.so /usr/lib/security/pam_duo.so /lib/security/pam_duo.so; do
  if [[ -f "$p" ]]; then MODULE="$p"; break; fi
done
[[ -n "$MODULE" ]] || { echo "pam_duo.so fehlt. Installiere zuerst Duo Unix nach offizieller Duo-Anleitung." >&2; exit 1; }

CONFIG=""
for p in /etc/duo/pam_duo.conf /etc/security/pam_duo.conf; do
  if [[ -f "$p" ]]; then CONFIG="$p"; break; fi
done
[[ -n "$CONFIG" ]] || { echo "pam_duo.conf fehlt; kein PAM-Testdienst wird angelegt." >&2; exit 1; }
[[ $(stat -c '%U' "$CONFIG") == root ]] || { echo "pam_duo.conf muss root gehören." >&2; exit 1; }
MODE=$(stat -c '%a' "$CONFIG")
PERM=$((8#$MODE))
(( (PERM & 077) == 0 )) || { echo "pam_duo.conf ist für Gruppe/Andere lesbar; setze restriktive Rechte (z.B. 600)." >&2; exit 1; }

# Check required values without printing secrets.
awk '
  BEGIN { in_duo=0; FS="=" }
  /^[[:space:]]*\[/ { in_duo=($0 ~ /^[[:space:]]*\[duo\][[:space:]]*$/); next }
  in_duo {
    key=$1; val=$2; gsub(/[[:space:]]/, "", key); gsub(/[[:space:]\r]/, "", val); sub(/[;#].*$/, "", val)
    if (key=="ikey" && val!="") ikey=1
    if (key=="skey" && val!="") skey=1
    if (key=="host" && val!="") host=1
    if (key=="failmode" && val=="secure") secure=1
    if (key=="autopush" && val=="yes") push=1
    if (key=="prompts" && val=="1") prompts=1
  }
  END { exit !(ikey && skey && host && secure && push && prompts) }
' "$CONFIG" || { echo "Duo-Konfiguration unvollständig: benötigt [duo], ikey, skey, host, failmode=secure, autopush=yes, prompts=1." >&2; exit 1; }

TMP=$(mktemp /etc/pam.d/.proxilens-duo-test.XXXXXX)
trap 'rm -f "$TMP"' EXIT
cat > "$TMP" <<'PAM'
#%PAM-1.0
# ProxiLens isolated Duo test service. NOT the sudo PAM stack.
# Deliberately Duo-only: no local-password fallback in this test.
auth    requisite    pam_duo.so
account required     pam_unix.so
PAM
chown root:root "$TMP"
chmod 0644 "$TMP"
mv "$TMP" "$SERVICE"
trap - EXIT

echo "Isolierter PAM-Testdienst erstellt: $SERVICE"
echo "Produktive Dateien /etc/pam.d/sudo und common-auth blieben unverändert."
echo "Test (erst nach Installation von pamtester): sudo pamtester proxilens-duo-test \"\$USER\" authenticate"
echo "Entfernen: sudo ./scripts/remove-pam-duo-test-service.sh"
