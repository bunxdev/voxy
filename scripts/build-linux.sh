#!/usr/bin/env bash
# Deterministic Linux launcher/GPU bundle; VM images remain checksum-pinned downloads.
set -euo pipefail
export LC_ALL=C
ROOT=$(cd "$(dirname "$0")/.." && pwd)
[[ $(uname -s) = Linux ]] || { echo 'Build this package on Linux.' >&2; exit 1; }
VERSION=${VOXY_VERSION:-$(cat "$ROOT/VERSION")}
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo 'Invalid version.' >&2; exit 1; }
case $(uname -m) in x86_64) ARCH=amd64; RUST_TARGET=x86_64-unknown-linux-gnu;; aarch64) ARCH=arm64; RUST_TARGET=aarch64-unknown-linux-gnu;; *) echo 'Unsupported architecture.' >&2; exit 1;; esac
OUT=${VOXY_DIST_DIR:-$ROOT/dist}
mkdir -p "$OUT"
OUT=$(cd "$OUT" && pwd)
NAME="Voxy-$VERSION-linux-$ARCH"
ARCHIVE="$OUT/$NAME.tar.gz"
[[ ! -e "$ARCHIVE" && ! -e "$ARCHIVE.sha256" ]] || { echo "Refusing to overwrite $ARCHIVE" >&2; exit 1; }
if [[ -z "${VOXY_GPU_BINARY:-}" ]]; then "$ROOT/scripts/build-gpu.sh"; fi
HELPER=${VOXY_GPU_BINARY:-$ROOT/bin/voxy-gpu}
[[ -x "$HELPER" ]] || { echo 'VOXY_GPU_BINARY must be an executable Linux helper.' >&2; exit 1; }
MACHINE=$(readelf -h "$HELPER" | sed -n 's/^ *Machine: *//p')
case "$ARCH:$MACHINE" in amd64:*X86-64*|arm64:AArch64) ;; *) echo 'GPU helper architecture does not match this package.' >&2; exit 1;; esac
stage=$(mktemp -d "$OUT/.linux-package.XXXXXX")
trap 'rm -rf "$stage"' EXIT
package="$stage/$NAME"
mkdir -p "$package/bin" "$package/images" "$package/scripts" "$package/licenses"
cp "$ROOT/voxy" "$ROOT/VERSION" "$ROOT/README.md" "$ROOT/TESTING.md" "$package/"
cp -R "$ROOT/lib" "$ROOT/docs" "$package/"
cp "$ROOT/images/amd64.lock" "$ROOT/images/arm64.lock" "$package/images/"
cp "$HELPER" "$package/bin/voxy-gpu"
cp "$ROOT/scripts/build-gpu.sh" "$ROOT/scripts/build-linux.sh" "$package/scripts/"
mkdir -p "$package/gpu"
cp "$ROOT/gpu/Cargo.toml" "$ROOT/gpu/Cargo.lock" "$ROOT/gpu/README.md" "$package/gpu/"
cp -R "$ROOT/gpu/src" "$ROOT/gpu/tests" "$package/gpu/"
# Automatic port discovery is optional; manual forwarding does not need this helper.
if [[ -n "${VOXY_PORTS_HELPER:-}" ]]; then
  cp "$VOXY_PORTS_HELPER" "$package/bin/voxy-ports"
  cp "$ROOT/packaging/Go-LICENSE" "$package/licenses/Go-LICENSE"
fi
for notice in "$ROOT"/LICENSE* "$ROOT"/COPYING*; do
  [[ ! -f "$notice" ]] || cp "$notice" "$package/licenses/"
done
# Include the notices of locked Rust dependencies; no credentials/build outputs are copied.
cargo metadata --locked --format-version 1 --filter-platform "$RUST_TARGET" --manifest-path "$ROOT/gpu/Cargo.toml" > "$stage/metadata.json"
python3 - "$stage/metadata.json" "$package/licenses" <<'PY'
import json, pathlib, shutil, sys
metadata=json.loads(pathlib.Path(sys.argv[1]).read_text())
out=pathlib.Path(sys.argv[2])/'rust'; out.mkdir()
index=[]
for item in sorted(metadata['packages'],key=lambda p:(p['name'],p['version'])):
    if item['source'] is None:continue
    root=pathlib.Path(item['manifest_path']).parent
    dest=out/(item['name']+'-'+item['version']); dest.mkdir()
    names=[]
    candidates=list(root.iterdir())
    if item.get('license_file'):candidates.append(root/item['license_file'])
    for source in sorted(set(candidates)):
        if source.is_file() and (source.name.upper().startswith(('LICENSE','COPYING','NOTICE')) or str(source)==str(root/(item.get('license_file') or ''))):
            shutil.copyfile(source,dest/source.name);names.append(source.name)
    index.append({'name':item['name'],'version':item['version'],'license':item.get('license'),'repository':item.get('repository'),'notices':names})
(out/'rust-dependencies.json').write_text(json.dumps(index,indent=2,sort_keys=True)+'\n')
PY
GLIBC_MIN=$(readelf --version-info "$HELPER" | grep -o 'GLIBC_[0-9.]*' | sort -Vu | tail -1 || true)
[[ -n "$GLIBC_MIN" ]] || GLIBC_MIN='none recorded (inspect ELF dependencies)'
{
  printf 'Package: %s\nArchitecture: %s\nGPU helper minimum GLIBC symbol version: %s\n' "$NAME" "$ARCH" "$GLIBC_MIN"
  printf 'GPU helper SHA256: '; sha256sum "$HELPER" | cut -d' ' -f1
  printf '\nGPU ELF needed libraries:\n'
  readelf -d "$HELPER" | sed -n 's/.*Shared library: \[\(.*\)\]/\1/p'
} > "$package/BUILD-INFO.txt"
cat > "$package/README-LINUX.txt" <<NOTICE
Voxy $VERSION for Linux $ARCH

Extract this directory, then run ./voxy init and ./voxy start.
Debian is downloaded from the image lock URL and its SHA256 is verified.
This package does not bundle a disk image, QEMU, NVIDIA drivers or Vulkan loader.
Install the host distribution's QEMU system emulator, qemu-img, OpenSSH client,
curl, xz, Bash and standard Unix tools. GPU compute additionally requires a
supported NVIDIA GPU, its Vulkan driver, and libvulkan.so.1 on the host.
The helper's actual minimum GLIBC requirement is $GLIBC_MIN; see BUILD-INFO.txt.
A package built on a newer host does not imply support for older distributions.

GPU: ./voxy gpu status; ./voxy gpu enable; ./voxy gpu test.
Trusted WGSL uses the host GPU through an authenticated local bridge; there is
no CPU fallback and no transparent CUDA/PyTorch device inside Debian.
See docs/GPU.md and gpu/README.md for limits and the trust boundary.
If bin/voxy-ports is absent, automatic port discovery is unavailable; explicit
forwarding via ./voxy ports add remains available.
NOTICE
chmod 755 "$package/voxy" "$package/bin/voxy-gpu" "$package/scripts/"*.sh
find "$package" -type d -exec chmod 755 {} +
(cd "$package" && find . -type f ! -name SHA256SUMS -print0 | sort -z | xargs -0 sha256sum > SHA256SUMS)
epoch=${SOURCE_DATE_EPOCH:-$(git -C "$ROOT" log -1 --format=%ct 2>/dev/null || printf '0')}
[[ "$epoch" =~ ^[0-9]+$ ]] || { echo 'Invalid SOURCE_DATE_EPOCH.' >&2; exit 1; }
tar --sort=name --mtime="@$epoch" --owner=0 --group=0 --numeric-owner -C "$stage" -cf - "$NAME" | gzip -n -9 > "$ARCHIVE"
(cd "$OUT" && sha256sum "$NAME.tar.gz" > "$NAME.tar.gz.sha256")
printf 'Package: %s\nMinimum recorded GLIBC: %s\n' "$ARCHIVE" "$GLIBC_MIN"
