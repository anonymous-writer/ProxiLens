#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SRC="$ROOT/cmd/proxilens-pam-test/main.c"
OUT="$ROOT/build/proxilens-pam-test"

if [[ "${EUID}" -eq 0 ]]; then
  echo "Bitte als normaler Benutzer bauen, nicht als root." >&2
  exit 2
fi
if ! command -v cc >/dev/null 2>&1; then
  echo "C-Compiler fehlt. Benötigt wird build-essential." >&2
  exit 2
fi
if ! printf '#include <security/pam_appl.h>\n' | cc -x c -fsyntax-only - >/dev/null 2>&1; then
  echo "PAM-Entwicklungsheader fehlen. Auf Ubuntu: sudo apt install libpam0g-dev" >&2
  exit 2
fi
mkdir -p "$ROOT/build"
cc -std=c11 -O2 -Wall -Wextra -Werror -D_FORTIFY_SOURCE=2 \
  -fstack-protector-strong -fPIE -pie -Wl,-z,relro,-z,now \
  "$SRC" -o "$OUT" -lpam
chmod 0755 "$OUT"
echo "Gebaut: $OUT"
echo "Start nur für den isolierten Diagnosetest: sudo -- '$OUT'"
