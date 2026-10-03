#!/usr/bin/env bash
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/.." && pwd)
case $(uname -s) in Linux|Darwin) ;; *) echo 'GPU compartida disponible en Linux y macOS' >&2; exit 1;; esac
mkdir -p "$ROOT/bin"
[[ $# = 0 ]] || { echo 'Native build only; use cargo directly for cross compilation.' >&2; exit 1; }
cargo build --release --locked --manifest-path "$ROOT/gpu/Cargo.toml"
target_dir=${CARGO_TARGET_DIR:-$ROOT/gpu/target}
cp "$target_dir/release/voxy-gpu" "$ROOT/bin/voxy-gpu"
chmod 755 "$ROOT/bin/voxy-gpu"
if [[ $(uname -s) = Darwin ]]; then codesign --force --sign - "$ROOT/bin/voxy-gpu"; fi
echo "GPU helper: $ROOT/bin/voxy-gpu"
