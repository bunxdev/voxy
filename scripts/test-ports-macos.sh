#!/bin/bash
# Fresh VM from the mounted package; never uses the installed VM's disk.
set -euo pipefail
DMG=${1:?DMG path}
PROBE_URL=${2:?URL to architecture-matched Linux test server}
REMOTE_IP=${3:?Host Tailscale IPv4}
WORK=$(mktemp -d "$HOME/voxy-ports-test.XXXXXX")
MOUNT="$WORK/mounted"
mkdir "$MOUNT"
BLOCKER=''
cleanup() {
 [[ -z "$BLOCKER" ]] || kill "$BLOCKER" 2>/dev/null || true
 [[ -z "${V:-}" ]] || "$V" stop >/dev/null 2>&1 || true
 hdiutil detach "$MOUNT" >/dev/null 2>&1 || true
}
trap cleanup EXIT
hdiutil verify "$DMG" >/dev/null
hdiutil attach "$DMG" -nobrowse -readonly -mountpoint "$MOUNT" >/dev/null
ditto "$MOUNT/Voxy.app" "$WORK/Voxy.app"
hdiutil detach "$MOUNT" >/dev/null
codesign --verify --deep --strict "$WORK/Voxy.app"
V="$WORK/Voxy.app/Contents/Resources/voxy"
export VOXY_DATA_DIR="$WORK/Debian"
export VOXY_SSH_PORT=22460
export PATH=/usr/bin:/bin:/usr/sbin:/sbin
"$V" init
"$V" ports add tcp 33460-33461 8080-8081
"$V" ports add udp 33462 8082
"$V" ports add tcp 33463 8080 "$REMOTE_IP"
if "$V" ports add tcp 33460 8080 0.0.0.0; then exit 1; fi
"$V" start
"$V" wait
probe() {
 "$V" ssh 'command -v curl >/dev/null || (apt-get update -qq && apt-get install -y --no-install-recommends curl >/tmp/ports-apt.log)'
 "$V" ssh "curl -fsS --max-time 60 '$PROBE_URL' -o /tmp/voxy-port-probe && chmod +x /tmp/voxy-port-probe && systemd-run --unit=voxy-port-probe /tmp/voxy-port-probe"
 for n in {1..15}; do
  if curl -fsS --max-time 2 http://127.0.0.1:33460/ > "$WORK/http.txt"; then break; fi
  sleep 1
 done
 grep -q voxy-ports-ok "$WORK/http.txt"
 curl -fsS --max-time 5 http://127.0.0.1:33461/ | grep voxy-ports-ok
 curl -fsS --max-time 5 "http://$REMOTE_IP:33463/" | grep voxy-ports-ok
 printf 'voxy-udp-ok' | nc -u -w 2 127.0.0.1 33462 > "$WORK/udp.txt"
 grep -q voxy-udp-ok "$WORK/udp.txt"
}
probe
"$V" ports list
"$V" stop
"$V" start
"$V" wait
probe
"$V" ports clear
# Saving new config doesn't remove active rules until reboot.
curl -fsS --max-time 5 http://127.0.0.1:33460/ | grep voxy-ports-ok
"$V" ports list
"$V" stop
"$V" start
"$V" wait
if curl -fsS --max-time 2 http://127.0.0.1:33460/; then echo 'Removed mapping still open'; exit 1; fi
"$V" stop
"$V" ports add tcp 33460 8080
nc -l 127.0.0.1 33460 >/dev/null & BLOCKER=$!
sleep 1
if "$V" start; then echo 'Occupied port accepted'; exit 1; fi
kill "$BLOCKER"; wait "$BLOCKER" 2>/dev/null || true; BLOCKER=''
"$V" start
"$V" wait
"$V" stop
printf 'PASS macOS: TCP range, UDP echo, Tailscale bind, restart, removal, busy port, clean recovery\nEvidence: %s\n' "$WORK"
