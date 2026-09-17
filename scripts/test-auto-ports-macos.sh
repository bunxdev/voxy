#!/bin/bash
# Isolated VM; run against an extracted 0.7.0 package.
set -euo pipefail
V=${1:?Path to Voxy.app/Contents/Resources/voxy}
PROBE_URL=${2:?URL of architecture-matched Linux ports-probe}
TEST=$(mktemp -d "$HOME/voxy-auto-test.XXXXXX")
export VOXY_DATA_DIR="$TEST/Debian" VOXY_SSH_PORT=22470
trap '"$V" stop || true' EXIT
"$V" init
"$V" ports clear
"$V" ports add tcp 33470 8080
"$V" ports auto on
"$V" start
"$V" wait
"$V" ssh 'command -v ss; ss -H -lntu4'
"$V" ssh 'command -v curl >/dev/null || (apt-get update -qq && apt-get install -y --no-install-recommends curl >/tmp/apt.log)'
"$V" ssh "curl -fsS $PROBE_URL -o /root/ports-probe && chmod +x /root/ports-probe && systemd-run --unit=ports-probe /root/ports-probe"
for i in {1..20}; do curl -fsS --max-time 2 http://127.0.0.1:8080/ > "$TEST/http.txt" && break; sleep 2; done
grep voxy-ports-ok "$TEST/http.txt"
curl -fsS http://127.0.0.1:8081/ | grep voxy-ports-ok
printf voxy-udp-ok | nc -u -w 2 127.0.0.1 8082 > "$TEST/udp.txt"
grep voxy-udp-ok "$TEST/udp.txt"
"$V" ports list
"$V" ports auto off
sleep 5
if curl -fsS --max-time 2 http://127.0.0.1:8080/; then echo 'off failed'; exit 1; fi
curl -fsS http://127.0.0.1:33470/ | grep voxy-ports-ok
"$V" ports auto on
sleep 5
curl -fsS http://127.0.0.1:8080/ | grep voxy-ports-ok
"$V" ssh 'systemctl stop ports-probe'
sleep 5
if curl -fsS --max-time 2 http://127.0.0.1:8080/; then exit 1; fi
"$V" stop
"$V" start
"$V" wait
"$V" ssh 'systemd-run --unit=ports-probe /root/ports-probe'
sleep 5
curl -fsS http://127.0.0.1:8080/ | grep voxy-ports-ok
"$V" ports list
printf 'PASS auto mac: TCP UDP live on/off removal restart manual coexistence\n'
