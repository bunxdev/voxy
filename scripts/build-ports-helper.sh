#!/usr/bin/env bash
# Source installs: package builds already include the helper.
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/.." && pwd)
mkdir -p "$ROOT/bin"
(cd "$ROOT/windows" && CGO_ENABLED=0 go build -trimpath -o "$ROOT/bin/voxy-ports" ./cmd/voxy-ports)
