#!/usr/bin/env bash
set -euo pipefail

say() { printf '%s\n' "$*"; }
fail() { say "FEHLER: $*" >&2; exit 1; }

say "ProxiLens Beta 10.8 – Duo-PAM Vorprüfung (nur lesend)"
say "====================================================="
if [[ -r /etc/os-release ]]; then
  . /etc/os-release
  say "System: ${PRETTY_NAME:-unbekannt}"
fi
say "Kernel: $(uname -sr)"
say "Architektur: $(dpkg --print-architecture 2>/dev/null || uname -m)"
say

module=""
for p in /usr/lib/*/security/pam_duo.so /lib/*/security/pam_duo.so /usr/lib/security/pam_duo.so /lib/security/pam_duo.so; do
  if [[ -f "$p" ]]; then module="$p"; break; fi
done
if [[ -n "$module" ]]; then
  say "pam_duo-Modul: gefunden ($module)"
else
  say "pam_duo-Modul: NICHT installiert"
fi

say "Duo-Unix-Paket:"
if dpkg-query -W -f='${Status} ${Version}\n' duo-unix 2>/dev/null | grep -q '^install ok installed'; then
  dpkg-query -W -f='  ${Package} ${Version}\n' duo-unix
else
  say "  duo-unix ist nicht als installiertes Paket registriert"
fi

config=""
for p in /etc/duo/pam_duo.conf /etc/security/pam_duo.conf; do
  if [[ -f "$p" ]]; then config="$p"; break; fi
done
if [[ -n "$config" ]]; then
  say "Duo-Konfiguration: gefunden ($config)"
  owner=$(stat -c '%U' "$config")
  mode=$(stat -c '%a' "$config")
  say "  Eigentümer: $owner; Modus: $mode"
  [[ "$owner" == root ]] || say "  WARNUNG: Datei gehört nicht root"
  # Reject any group/other permissions without exposing file contents.
  perm=$((8#$mode))
  (( (perm & 077) == 0 )) || say "  WARNUNG: Gruppen-/Andere-Rechte gesetzt; erwartet 600 oder restriktiver"
  awk '
    BEGIN { in_duo=0; FS="=" }
    /^[[:space:]]*\[/ { in_duo=($0 ~ /^[[:space:]]*\[duo\][[:space:]]*$/); next }
    in_duo {
      key=$1; val=$2
      gsub(/[[:space:]]/, "", key); gsub(/[[:space:]\r]/, "", val)
      sub(/[;#].*$/, "", val)
      if (key=="ikey" && val!="") ikey=1
      if (key=="skey" && val!="") skey=1
      if (key=="host" && val!="") host=1
      if (key=="failmode" && val=="secure") secure=1
      if (key=="autopush" && val=="yes") push=1
      if (key=="prompts" && val=="1") prompts=1
    }
    END {
      printf "  Integration-Key vorhanden: %s\n", ikey?"ja":"NEIN"
      printf "  Secret-Key vorhanden: %s\n", skey?"ja":"NEIN"
      printf "  API-Hostname vorhanden: %s\n", host?"ja":"NEIN"
      printf "  failmode=secure: %s\n", secure?"ja":"NEIN"
      printf "  autopush=yes: %s\n", push?"ja":"NEIN"
      printf "  prompts=1: %s\n", prompts?"ja":"NEIN"
      if (!(ikey && skey && host && secure && push && prompts)) exit 3
    }
  ' "$config" || say "  WARNUNG: Konfiguration unvollständig oder gewünschte sichere Duo-Optionen fehlen"
else
  say "Duo-Konfiguration: NICHT gefunden"
fi

if command -v pamtester >/dev/null 2>&1; then
  say "pamtester: vorhanden ($(command -v pamtester))"
else
  say "pamtester: nicht installiert (für isolierten PAM-Test erforderlich)"
fi

say
say "Produktive PAM-Dateien wurden NICHT gelesen oder verändert."
say "sudo, common-auth, sudoers und systemd wurden NICHT verändert."
say "Hinweis: Dieser Testpfad ist Duo-only; der manuelle lokale Passwort-Fallback ist darin absichtlich NICHT enthalten."
