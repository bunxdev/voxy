#!/usr/bin/env bash
set -euo pipefail
[[ $(uname -s) = Darwin ]] || { echo 'Este instalador requiere macOS'; exit 1; }
export PATH="/opt/homebrew/bin:/usr/local/bin:/opt/local/bin:$PATH"
case $(uname -m) in arm64) qemu=qemu-system-aarch64;; x86_64) qemu=qemu-system-x86_64;; *) exit 1;; esac
if ! command -v "$qemu" >/dev/null || ! command -v qemu-img >/dev/null; then
  if command -v brew >/dev/null; then
    brew update
    HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_NO_INSTALL_CLEANUP=1 brew install qemu
  elif command -v port >/dev/null; then
    sudo port -N install qemu
  else
    echo 'Instala Homebrew o MacPorts para tu versión de macOS y vuelve a ejecutar este script.' >&2
    echo 'https://brew.sh/  https://www.macports.org/install.php' >&2
    exit 1
  fi
fi
"$qemu" --version
qemu-img --version
[[ $(sysctl -n kern.hv_support) = 1 ]] || { echo 'HVF no está disponible'; exit 1; }
echo 'Dependencias listas. Ejecuta ./voxy init y ./voxy start (sin sudo).'
