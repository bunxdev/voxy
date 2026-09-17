#!/bin/bash
# Test an actual mounted DMG using a new VM, without package-manager PATH entries.
set -euo pipefail
export LC_ALL=C
DMG=${1:?Usage: test-dmg.sh path/to/Voxy.dmg}
WORK=$(mktemp -d "$HOME/voxy-dmg-test.XXXXXX")
MOUNT="$WORK/mounted"
mkdir -p "$MOUNT" "$WORK/Installed Apps"
trap 'hdiutil detach "$MOUNT" >/dev/null 2>&1 || true' EXIT
hdiutil verify "$DMG"
hdiutil attach "$DMG" -nobrowse -readonly -mountpoint "$MOUNT"
ditto "$MOUNT/Voxy.app" "$WORK/Installed Apps/Voxy.app"
hdiutil detach "$MOUNT"
APP="$WORK/Installed Apps/Voxy.app"
RES="$APP/Contents/Resources"
codesign --verify --deep --strict "$APP"
export PATH=/usr/bin:/bin:/usr/sbin:/sbin
export VOXY_DATA_DIR="$WORK/Fresh Debian"
export VOXY_SSH_PORT=22333
for file in "$RES/bin/"* "$RES/lib/"*; do
  case "$file" in *.sh|*.awk) continue;; esac
  if otool -L "$file" | tail -n +2 | grep -E '/opt/|/usr/local/'; then
    echo "External dependency in $file"; exit 1
  fi
done
# An invalid proxy verifies that init uses the embedded verified image.
https_proxy=http://127.0.0.1:1 "$RES/voxy" init
DYLD_PRINT_LIBRARIES=1 "$RES/bin/qemu-img" info "$VOXY_DATA_DIR/disk.qcow2" 2> "$WORK/dyld-img.log"
if grep -E '/opt/|/usr/local/' "$WORK/dyld-img.log"; then exit 1; fi
"$RES/voxy" test
"$RES/voxy" status
printf '4\n' | "$RES/Voxy.command" > "$WORK/panel.log"
grep 'Debian mínimo' "$WORK/panel.log"
printf 'PASS DMG: %s\nInstalled app: %s\nEvidence: %s\n' "$DMG" "$APP" "$WORK"
