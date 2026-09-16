#!/usr/bin/env bash
# Use the actual Windows ZIP, a dedicated Wine prefix and a fresh test VM.
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/.." && pwd)
ZIP=${1:?Usage: test-wine.sh /absolute/path/Voxy-windows-x64.zip}
ZIP=$(realpath "$ZIP")
if [[ -z "${DISPLAY:-}" ]]; then exec xvfb-run -a "$0" "$ZIP"; fi
mkdir -p "$ROOT/.runtime"
WORK=$(mktemp -d "$ROOT/.runtime/wine-test.XXXXXX")
export WINEPREFIX="$WORK/prefix" WINEARCH=win64 WINEDEBUG=-all
export WINEDLLOVERRIDES='mscoree,mshtml=d' VOXY_ACCEL=tcg
unset VOXY_DATA_DIR VOXY_SSH_PORT
7z x -y -o"$WORK/Paquete ñ con espacios" "$ZIP" > "$WORK/extract.log"
APP="$WORK/Paquete ñ con espacios/Voxy"
(cd "$APP" && sha256sum -c SHA256SUMS) > "$WORK/checksums.log"
wineboot -u > "$WORK/wineboot.log" 2>&1
export VOXY_DATA_DIR="$(winepath -w "$WORK/Datos ñ/amd64")"
wine "$APP/Voxy.exe" doctor 2>&1 | tee "$WORK/doctor.log"
wine "$APP/Voxy.exe" test 2>&1 | tee "$WORK/test.log"
printf '5\n' | wine "$APP/Voxy.exe" > "$WORK/menu.log"
printf 'PASS Wine + TCG. Evidence: %s\n' "$WORK"
