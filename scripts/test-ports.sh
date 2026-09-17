#!/bin/bash
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/.." && pwd)
STATE=$(mktemp -d)
trap 'rm -rf "$STATE"' EXIT
PORT=22222
source "$ROOT/lib/ports.sh"
running() { return 1; }
ports_command add tcp 33033-33035 8000-8002
cp "$STATE/ports.conf" "$STATE/before"
reject() {
 if ports_command "$@"; then echo "Unexpected success: $*" >&2; exit 1; fi
 cmp "$STATE/ports.conf" "$STATE/before"
}
reject add tcp 33033 9 0.0.0.0
reject add tcp 22222 22
reject add udp 0 9
reject add tcp 1-65535 1-65535
reject add tcp 5000-5002 8000-8001
reject add tcp 5000 5000 '127.0.0.1,hostfwd=tcp::1-:1'
reject add tcp 5000 5000 224.0.0.1
reject remove tcp 33033-33036
ports_command add udp 33033 9000
ports_command add tcp 33033 9000 100.85.206.59
rules=$(ports_read)
ports_netdev "$rules"
[[ "$NETDEV" == *hostfwd=tcp:127.0.0.1:33035-:8002* ]]
[[ "$NETDEV" == *hostfwd=udp:127.0.0.1:33033-:9000* ]]
ports_command remove tcp 33033-33035
[[ $(wc -l < "$STATE/ports.conf" | tr -d ' ') = 2 ]]
printf 'tcp 0.0.0.0 99999 80\n' > "$STATE/ports.conf"
if ports_read; then echo 'Accepted corrupt config'; exit 1; fi
ports_command clear
ports_command add tcp 40000-40255 40000-40255
cp "$STATE/ports.conf" "$STATE/before"
reject add udp 50000 50000
PORT=40000
if ports_read; then echo 'Accepted SSH conflict'; exit 1; fi
printf 'PASS port rules: ranges, persistence, overlap, invalid input, removal, limits, SSH conflict\n'
