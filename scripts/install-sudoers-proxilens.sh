#!/usr/bin/env bash
set -euo pipefail

# Deliberately refuses to activate sudo policy until the real PAM module and
# privileged authorizer are installed. It never edits /etc/sudoers directly.
TARGET=/etc/sudoers.d/90-proxilens
SOURCE="$(cd "$(dirname "$0")/.." && pwd)/packaging/sudoers/proxilens"
multiarch=""
if command -v dpkg-architecture >/dev/null 2>&1; then
  multiarch="$(dpkg-architecture -qDEB_HOST_MULTIARCH 2>/dev/null || true)"
fi
if [[ -z "$multiarch" ]]; then
  case "$(dpkg --print-architecture 2>/dev/null || true)" in
    amd64) multiarch="x86_64-linux-gnu" ;;
    arm64) multiarch="aarch64-linux-gnu" ;;
    armhf) multiarch="arm-linux-gnueabihf" ;;
    i386) multiarch="i386-linux-gnu" ;;
    riscv64) multiarch="riscv64-linux-gnu" ;;
    *) echo "Nicht unterstützte Debian-Architektur für PAM: $(dpkg --print-architecture 2>/dev/null || echo unbekannt)" >&2; exit 2 ;;
  esac
fi
PAM_MODULE="/usr/lib/${multiarch}/security/pam_proxilens.so"
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
