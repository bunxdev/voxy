#!/usr/bin/env bash
# Only needed for a clean Intel Mac running Ventura without a package manager.
set -euo pipefail
[[ $(uname -s) = Darwin && $(uname -m) = x86_64 && $(sw_vers -productVersion) = 13.* ]] || { echo 'Este bootstrap requiere macOS 13 Ventura Intel'; exit 1; }
if [[ -x /opt/local/bin/port ]]; then /opt/local/bin/port version; exit 0; fi
stage=$(mktemp -d)
trap 'rm -rf "$stage"' EXIT
cd "$stage"
pkg=MacPorts-2.12.6-13-Ventura.pkg
curl -fL --retry 3 -o "$pkg" "https://github.com/macports/macports-base/releases/download/v2.12.6/$pkg"
printf '260e3dc742d359cad4e78f03007102acff8bca29d9dcac5ab434fbb2f5e91adf  %s\n' "$pkg" | shasum -a 256 -c -
sudo installer -pkg "$pkg" -target /
echo 'MacPorts instalado. Continúa con ./scripts/install-macos.sh'
