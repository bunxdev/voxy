#!/bin/bash
# Run natively on each Mac. No credentials or existing VM data are packaged.
set -euo pipefail
export LC_ALL=C
ROOT=$(cd "$(dirname "$0")/.." && pwd)
[[ $(uname -s) = Darwin ]] || { echo 'Build on macOS'; exit 1; }
VERSION=${VOXY_VERSION:-$(cat "$ROOT/VERSION")}
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo 'Invalid package version'; exit 1; }
case $(uname -m) in
  arm64) ARCH=arm64; TARGET=aarch64; PREFIX=/opt/homebrew; MIN_OS=15.0;;
  x86_64) ARCH=amd64; TARGET=x86_64; PREFIX=/opt/local; MIN_OS=13.0;;
  *) exit 1;;
esac
export PATH="$PREFIX/bin:$PATH"
OUT="$ROOT/dist/macos-$ARCH-$VERSION"
[[ ! -e "$OUT" ]] || { echo "Remove previous build directory explicitly: $OUT"; exit 1; }
APP="$OUT/stage/Voxy.app"
mkdir -p "$OUT/stage"
osacompile -o "$APP" "$ROOT/packaging/macos/launcher.applescript"
RES="$APP/Contents/Resources"
mkdir -p "$RES/bin" "$RES/lib" "$RES/images" "$RES/share/qemu" "$RES/licenses"
iconutil -c icns "$ROOT/assets/icons/Voxy.iconset" -o "$RES/Voxy.icns"
/usr/libexec/PlistBuddy -c 'Set :CFBundleIconFile Voxy.icns' "$APP/Contents/Info.plist"
cp "$ROOT/voxy" "$RES/voxy"
cp "$ROOT/lib/ports.sh" "$ROOT/lib/ports.awk" "$RES/lib/"
cp "$ROOT/packaging/macos/Voxy.command" "$RES/"
chmod +x "$RES/voxy" "$RES/Voxy.command"
cp "$ROOT/images/$ARCH.lock" "$RES/images/"
source "$ROOT/images/$ARCH.lock"
curl -fL --retry 3 "$IMAGE_URL" -o "$RES/images/base.tar.xz"
(cd "$RES/images" && printf '%s  base.tar.xz\n' "$IMAGE_SHA256" | shasum -a 256 -c -)
# ARM boots the kernel directly. x86 needs only the default PC firmware.
if [[ "$ARCH" = amd64 ]]; then
  for firmware in bios-256k.bin bios.bin kvmvapic.bin linuxboot_dma.bin vgabios-stdvga.bin; do
    cp "$PREFIX/share/qemu/$firmware" "$RES/share/qemu/"
  done
fi
cp "$(command -v qemu-system-$TARGET)" "$RES/bin/"
cp "$(command -v qemu-img)" "$RES/bin/"
printf '%s\n' "$RES/bin/qemu-system-$TARGET" "$RES/bin/qemu-img" > "$OUT/queue"
: > "$RES/dependencies.tsv"
index=1
while file=$(sed -n "${index}p" "$OUT/queue") && [[ -n "$file" ]]; do
  chmod u+w "$file"
  otool -L "$file" | tail -n +2 | awk '{print $1}' > "$OUT/deps"
  while IFS= read -r dep; do
    case "$dep" in
      /usr/lib/*|/System/Library/*) continue;;
      /opt/*|/usr/local/*)
        base=$(basename "$dep")
        dest="$RES/lib/$base"
        if [[ ! -e "$dest" ]]; then
          cp -L "$dep" "$dest"
          printf '%s\t%s\n' "$base" "$dep" >> "$RES/dependencies.tsv"
          printf '%s\n' "$dest" >> "$OUT/queue"
        fi
        install_name_tool -change "$dep" "@loader_path/../lib/$base" "$file";;
      *) echo "Unresolved dependency: $dep in $file"; exit 1;;
    esac
  done < "$OUT/deps"
  if [[ "$file" = "$RES/lib/"* ]]; then install_name_tool -id "@loader_path/$(basename "$file")" "$file"; fi
  index=$((index + 1))
done
# Keep upstream license notices available in the application bundle.
if [[ "$ARCH" = arm64 ]]; then
  for formula in qemu $(cut -f2 "$RES/dependencies.tsv" | sed -n 's|/opt/homebrew/opt/\([^/]*\)/.*|\1|p' | sort -u); do
    cellar=$(brew --prefix "$formula")
    mkdir -p "$RES/licenses/$formula"
    find -L "$cellar" -maxdepth 2 -type f \( -iname '*license*' -o -iname '*copying*' -o -name AUTHORS \) -exec cp {} "$RES/licenses/$formula/" \;
  done
else
  find "$PREFIX/share/doc" -type f \( -iname '*license*' -o -iname '*copying*' -o -iname '*copyright*' \) | while IFS= read -r notice; do
    relative=${notice#"$PREFIX/share/doc/"}
    mkdir -p "$RES/licenses/$(dirname "$relative")"
    cp "$notice" "$RES/licenses/$relative"
  done
  port installed > "$RES/licenses/macports-installed.txt"
fi
curl -fL --retry 3 https://raw.githubusercontent.com/qemu/qemu/v11.1.0/COPYING -o "$RES/licenses/QEMU-COPYING"
cat > "$RES/licenses/SOURCES.txt" <<SOURCES
QEMU 11.1.1: https://download.qemu.org/qemu-11.1.1.tar.xz
Build recipes and patches: https://github.com/Homebrew/homebrew-core (arm64)
https://github.com/macports/macports-ports/tree/master/emulators/qemu (amd64)
Bundled dynamic library origins: ../dependencies.tsv
Debian guest source/build recipes: $IMAGE_URL
https://github.com/bunxdev/qemu-debian-arm
https://github.com/bunxdev/qemu-debian-amd
Debian package copyright notices: /usr/share/doc/*/copyright inside guest.
SOURCES
/usr/libexec/PlistBuddy -c "Add :CFBundleIdentifier string com.bunxdev.voxy" "$APP/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Add :CFBundleShortVersionString string $VERSION" "$APP/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Add :CFBundleVersion string $VERSION" "$APP/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Add :LSMinimumSystemVersion string $MIN_OS" "$APP/Contents/Info.plist"
while IFS= read -r file; do codesign --force --sign - "$file"; done < "$OUT/queue"
codesign --force --sign - --entitlements "$ROOT/packaging/macos/hypervisor.plist" "$RES/bin/qemu-system-$TARGET"
codesign --force --sign - "$APP"
codesign --verify --deep --strict "$APP"
ln -s /Applications "$OUT/stage/Applications"
cat > "$OUT/stage/LEEME.txt" <<NOTICE
Voxy $VERSION ($ARCH). Arrastra Voxy.app a Aplicaciones y ábrela.
El panel se abre en Terminal: iniciar, entrar por SSH y apagar Debian.
Incluye Debian 12, QEMU y bibliotecas. No requiere Homebrew/MacPorts.
Datos fuera de la app: ~/Library/Application Support/Voxy/$ARCH.
Build experimental con firma ad-hoc; sin Developer ID ni notarización Apple.
macOS puede pedir autorización manual en Ajustes > Privacidad y seguridad.
Requisitos: macOS $MIN_OS o posterior; CPU $ARCH.
NOTICE
hdiutil create -volname "Voxy $VERSION $ARCH" -srcfolder "$OUT/stage" -format UDZO "$ROOT/dist/Voxy-$VERSION-macos-$ARCH.dmg"
shasum -a 256 "$ROOT/dist/Voxy-$VERSION-macos-$ARCH.dmg"
