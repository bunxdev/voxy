#!/usr/bin/env bash
# Cross-build on Linux. Wine is used separately for smoke/integration tests.
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/.." && pwd)
VERSION=$(sed -n 's/^const version = "\([^"]*\)"/\1/p' "$ROOT/windows/main_windows.go")
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo 'Invalid Windows launcher version'; exit 1; }
ZIP_NAME="Voxy-$VERSION-windows-x64.zip"
source "$ROOT/packaging/windows/qemu.lock"
source "$ROOT/images/amd64.lock"
CACHE="$ROOT/.cache/windows-builder"
BUILD="$ROOT/dist/windows-build"
APP="$BUILD/Voxy"
[[ ! -e "$BUILD" ]] || { echo "Move the previous build first: $BUILD"; exit 1; }
[[ ! -e "$ROOT/dist/$ZIP_NAME" ]] || { echo 'Move the previous Windows ZIP first'; exit 1; }
mkdir -p "$CACHE" "$APP/qemu/share" "$APP/image" "$APP/licenses"
fetch() { [[ -f "$2" ]] || curl -fL --retry 3 "$1" -o "$2"; }
fetch "$QEMU_URL" "$CACHE/qemu-setup.exe"
printf '%s  %s\n' "$QEMU_SHA512" "$CACHE/qemu-setup.exe" | sha512sum -c -
fetch "$IMAGE_URL" "$CACHE/base.tar.xz"
printf '%s  %s\n' "$IMAGE_SHA256" "$CACHE/base.tar.xz" | sha256sum -c -
EXTRACT=$(mktemp -d "$CACHE/extract.XXXXXX")
trap 'rm -rf "$EXTRACT"' EXIT
7z x -y -o"$EXTRACT/qemu" "$CACHE/qemu-setup.exe" > "$CACHE/extract.log"
tar -xJf "$CACHE/base.tar.xz" -C "$EXTRACT"
bun "$ROOT/scripts/bundle-windows.ts" "$EXTRACT/qemu" "$APP/qemu"
for firmware in bios-256k.bin bios.bin kvmvapic.bin linuxboot_dma.bin vgabios-stdvga.bin; do
 cp "$EXTRACT/qemu/share/$firmware" "$APP/qemu/share/"
done
for file in kernel initramfs disk.qcow2; do cp "$EXTRACT/$IMAGE_DIRECTORY/$file" "$APP/image/"; done
bun "$ROOT/scripts/image-manifest.ts" "$APP/image"
(cd "$ROOT/windows" && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o "$APP/Voxy.exe" .)
cp "$ROOT/packaging/windows/"*.cmd "$APP/"
cp "$ROOT/packaging/windows/LEEME.txt" "$APP/"
cp "$ROOT/packaging/windows/qemu.lock" "$ROOT/images/amd64.lock" "$APP/licenses/"
cp "$EXTRACT/qemu/COPYING" "$EXTRACT/qemu/COPYING.LIB" "$EXTRACT/qemu/VERSION" "$APP/licenses/"
cp "$(go env GOROOT)/LICENSE" "$APP/licenses/Go-LICENSE"
for module in golang.org/x/crypto golang.org/x/sys golang.org/x/term; do
 module_dir=$(cd "$ROOT/windows" && go list -m -f '{{.Dir}}' "$module")
 cp "$module_dir/LICENSE" "$APP/licenses/$(basename "$module")-LICENSE"
done
(cd "$ROOT/windows" && go list -m all) > "$APP/licenses/go-modules.txt"
cat > "$APP/licenses/SOURCES.txt" <<SOURCES
QEMU Windows build $QEMU_VERSION, unmodified binaries and DLLs from:
$QEMU_URL
Publisher and build/source instructions: https://qemu.weilnetz.de/w64/
https://github.com/stweil/qemu
QEMU dependency package sources/recipes: https://github.com/msys2/MINGW-packages
Exact bundled PE files, hashes and imports: ../qemu/dependencies.json
Debian image: $IMAGE_URL
Debian build recipes: https://github.com/bunxdev/qemu-debian-amd
Package copyright notices: /usr/share/doc/*/copyright inside Debian.
Voxy launcher source: https://github.com/bunxdev/voxy/tree/main/windows
Go dependencies and versions: go-modules.txt
Windows system DLLs and Wine DLLs are NOT redistributed.
SOURCES
(cd "$APP" && find . -type f ! -name SHA256SUMS -print0 | sort -z | xargs -0 sha256sum) > "$APP/SHA256SUMS"
(cd "$BUILD" && zip -q -r "$ROOT/dist/$ZIP_NAME" Voxy)
sha256sum "$ROOT/dist/$ZIP_NAME"
