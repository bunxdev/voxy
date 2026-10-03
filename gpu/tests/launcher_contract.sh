#!/usr/bin/env bash
# Real helper and real host curl, no VM required. Also runs with macOS Bash 3.2.
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
VOXY_GPU_BINARY=$1
workspace=$(mktemp -d "${TMPDIR:-/tmp}/voxy gpu launcher.XXXXXX")
STATE="$workspace/state with spaces"
PORT=22222
mkdir -p "$STATE"
source "$ROOT/lib/gpu.sh"
sleep 300 &
watch_pid=$!
helper_pid=''
cleanup() {
  if [[ -n "$helper_pid" ]]; then kill -TERM "$helper_pid" 2>/dev/null || true; wait "$helper_pid" 2>/dev/null || true; fi
  kill "$watch_pid" 2>/dev/null || true
  wait "$watch_pid" 2>/dev/null || true
  rm -rf "$workspace"
}
trap cleanup EXIT
"$GPU_BINARY" serve --state-dir "$GPU_STATE" --watch-pid "$watch_pid" > "$workspace/output" 2>&1 &
helper_pid=$!
for ((i=0;i<300;i++)); do [[ -f "$GPU_STATE/server.ready" ]] && break; sleep .05; done
[[ -f "$GPU_STATE/server.ready" ]]
# All reads must succeed with errexit: missing terminal newlines used to break this.
read -r actual_pid < "$GPU_STATE/server.pid"
read -r actual_port < "$GPU_STATE/server.port"
read -r token < "$GPU_STATE/token"
[[ "$actual_pid" = "$helper_pid" && ${#token} = 64 ]]
(umask 077; printf 'header = "Authorization: Bearer %s"\n' "$token" > "$GPU_STATE/curl.conf")
printf 'preserve\n' > "$GPU_STATE/unrelated"
gpu_server_alive
gpu_stop_service
wait "$helper_pid"
helper_pid=''
[[ -f "$GPU_STATE/unrelated" && ! -e "$GPU_STATE/server.lock" && ! -e "$GPU_STATE/token" ]]
kill -0 "$watch_pid"
# Stale state pointing at another live process must never terminate that process.
printf '%s\n' "$watch_pid" > "$GPU_STATE/server.pid"
printf 'stale\n' > "$GPU_STATE/server.lock"
if gpu_server_alive; then echo 'Wrong process was accepted as GPU server' >&2; exit 1; fi
gpu_stop_service
kill -0 "$watch_pid"
[[ ! -e "$GPU_STATE/server.lock" ]]
printf '{"passed":true,"checks":["Bash newline reads","paths with spaces","actual PID identity","authenticated shutdown","stale-lock cleanup","unrelated process preserved"]}\n'
