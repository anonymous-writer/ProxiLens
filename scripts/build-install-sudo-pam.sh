#!/usr/bin/env bash
ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

# Root phase: install artifacts already built by the invoking user. Do not
# rebuild under sudo: Ubuntu's sudo secure_path may hide a user-local Go toolchain.
if [[ $EUID -eq 0 ]]; then
  [[ ${1:-} == "--install" ]] || { echo 'Interner Installationsaufruf fehlt.' >&2; exit 2; }
  for artifact in build/proxilens-policy build/proxilens-duo-sync pam/pam_proxilens.so packaging/systemd/system/proxilens-policy.service packaging/helpers/proxilens-duo-import; do
    [[ -f "$ROOT/$artifact" && ! -L "$ROOT/$artifact" ]] || { echo "Build-Artefakt fehlt oder ist kein reguläres File: $artifact" >&2; exit 2; }
  done
  install -d -o root -g root -m 0755 /usr/libexec/proxilens /usr/lib/x86_64-linux-gnu/security /usr/lib/systemd/system
  install -o root -g root -m 0755 "$ROOT/build/proxilens-policy" /usr/libexec/proxilens/proxilens-policy
  install -o root -g root -m 0755 "$ROOT/build/proxilens-duo-sync" /usr/libexec/proxilens/proxilens-duo-sync
  install -o root -g root -m 0755 "$ROOT/packaging/helpers/proxilens-duo-import" /usr/libexec/proxilens/proxilens-duo-import
  install -o root -g root -m 0644 "$ROOT/pam/pam_proxilens.so" /usr/lib/x86_64-linux-gnu/security/pam_proxilens.so
  install -o root -g root -m 0644 "$ROOT/packaging/systemd/system/proxilens-policy.service" /usr/lib/systemd/system/proxilens-policy.service
  systemctl daemon-reload
  echo 'PAM-Modul, Policy-Dienst und systemd-Unit installiert. PAM/sudoers sind noch NICHT verändert.'
  echo 'Als Nächstes: sudo ./scripts/configure-duo-root.sh und danach sudo ./scripts/install-sudo-pam-proxilens.sh'
  exit 0
fi

command -v go >/dev/null || { echo 'Go fehlt im Benutzer-PATH.' >&2; exit 2; }
command -v make >/dev/null || { echo 'make fehlt.' >&2; exit 2; }
# Ubuntu/Debian libpam0g-dev commonly ships headers and libpam.so but no pam.pc.
PAM_HEADER="${PAM_HEADER:-/usr/include/security/pam_appl.h}"
[[ -r "$PAM_HEADER" ]] || { echo "PAM-Header fehlt: $PAM_HEADER (Paket: libpam0g-dev)." >&2; exit 2; }
command -v cc >/dev/null || { echo 'C-Compiler fehlt (Paket: build-essential).' >&2; exit 2; }
mkdir -p build
CGO_ENABLED=0 go build -trimpath -o build/proxilens-policy ./cmd/proxilens-policy
CGO_ENABLED=0 go build -trimpath -o build/proxilens-duo-sync ./cmd/proxilens-duo-sync
make -C pam clean all
echo 'Build erfolgreich. Installiere jetzt die Dateien mit sudo …'
exec sudo -- "$ROOT/scripts/build-install-sudo-pam.sh" --install
