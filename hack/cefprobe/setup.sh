#!/usr/bin/env bash
# Lay out the CEF payload this probe needs (#60 phase 0, belluikit/bellui#102).
#
# Fetches the pinned CEF binary distribution, verifies it, strips libcef.so,
# and arranges the result as ./cefroot — headers where the cgo build expects
# them, libcef.so and Resources where CefSettings expects them, plus the
# subprocess helper.
#
# The strip is not a nicety: libcef.so ships at 1.36 GB and is 256 MiB stripped.
# That 81% is what makes the payload shippable at all, and it is the same step
# belluikit/gocef-libs' mirror workflow will run (belluikit/bellui#104).
#
# Idempotent: re-running skips whatever is already in place.

set -euo pipefail

CEF_VERSION="154.0.34+g14c5a08+chromium-154.0.8037.98"
CEF_PLATFORM="${CEF_PLATFORM:-linux64}"
CEF_SHA1_linux64="15c91ad767097217b2e82ba8b933fe30b134be69"

cd "$(dirname "$0")"
ROOT="$PWD/cefroot"
ARCHIVE="$PWD/.cefcache/cef_${CEF_PLATFORM}.tar.bz2"
DIST="$PWD/.cefcache/cef_binary_${CEF_VERSION}_${CEF_PLATFORM}_minimal"

mkdir -p "$(dirname "$ARCHIVE")"

if [ ! -f "$ARCHIVE" ]; then
  # The version contains '+', which has to be percent-encoded in the URL.
  encoded="${CEF_VERSION//+/%2B}"
  url="https://cef-builds.spotifycdn.com/cef_binary_${encoded}_${CEF_PLATFORM}_minimal.tar.bz2"
  echo "fetching $url"
  curl -fSL --progress-bar -o "$ARCHIVE" "$url"
fi

# Verify against the pin. This is the same check belluikit/gocef-libs will make,
# and the reason the archive's checksum is recorded rather than trusted.
expected_var="CEF_SHA1_${CEF_PLATFORM}"
expected="${!expected_var:-}"
if [ -n "$expected" ]; then
  actual="$(sha1sum "$ARCHIVE" | cut -d' ' -f1)"
  if [ "$actual" != "$expected" ]; then
    echo "checksum mismatch for $ARCHIVE" >&2
    echo "  expected $expected" >&2
    echo "  actual   $actual" >&2
    exit 1
  fi
  echo "checksum ok: $actual"
fi

if [ ! -d "$DIST" ]; then
  echo "extracting (this takes a minute: 1.4 GB uncompressed)"
  tar -xjf "$ARCHIVE" -C "$(dirname "$DIST")"
fi

mkdir -p "$ROOT"

# Headers. CEF's own headers include each other as "include/capi/...", so the
# directory *containing* include/ is what goes on the compiler's search path —
# which is why the cgo CFLAGS point at cefroot and not at cefroot/include.
if [ ! -d "$ROOT/include" ]; then
  echo "staging headers"
  cp -r "$DIST/include" "$ROOT/include"
fi

if [ ! -f "$ROOT/libcef.so" ]; then
  echo "stripping libcef.so ($(du -h "$DIST/Release/libcef.so" | cut -f1) unstripped)"
  cp "$DIST/Release/libcef.so" "$ROOT/libcef.so.tmp"
  strip --strip-all "$ROOT/libcef.so.tmp"
  mv "$ROOT/libcef.so.tmp" "$ROOT/libcef.so"
  echo "  -> $(du -h "$ROOT/libcef.so" | cut -f1) stripped"
fi

# The rest of Release/: the Vulkan loader and SwiftShader, which is the software
# fallback a machine with no usable GPU lands on, and the V8 snapshot blobs.
#
# The blobs are easy to miss, because they are the one part of the runtime that
# is NOT under Resources/ — and omitting them fails as
# "Error loading V8 startup snapshot file", twice, followed by a core dump.
for so in libvk_swiftshader.so libvulkan.so.1 libEGL.so libGLESv2.so; do
  if [ -f "$DIST/Release/$so" ] && [ ! -f "$ROOT/$so" ]; then
    cp "$DIST/Release/$so" "$ROOT/$so"
  fi
done
for extra in "$DIST/Release/"*.bin "$DIST/Release/"*.json; do
  [ -f "$extra" ] && cp -n "$extra" "$ROOT/" 2>/dev/null || true
done

# Resources go **flat**, beside libcef.so, not in a Resources/ subdirectory.
# CEF's own distribution ships them under Resources/ and its CMake copies them
# next to the binary, because that is where Chromium looks: icudtl.dat in
# particular is found relative to the module, and a nested copy fails with
# "Invalid file descriptor to ICU data received" and a core dump rather than
# anything that names the real problem.
if [ ! -f "$ROOT/icudtl.dat" ]; then
  echo "staging Resources (flat, beside libcef.so)"
  cp -r "$DIST/Resources/." "$ROOT/"
fi

# The subprocess helper, built here rather than shipped, because this is a probe.
# belluikit/gocef-libs publishes a prebuilt one (belluikit/bellui#104).
if [ ! -f "$ROOT/cefprobe_helper" ]; then
  echo "building the subprocess helper"
  cc -O2 -I"$ROOT" -DCEF_API_VERSION=15101 -o "$ROOT/cefprobe_helper" helper/helper.c -ldl
fi

# The webview backend (and so hack/cefhostprobe) launches cef_helper, as laid
# out in a gocef-libs payload, at the binding's API version rather than this
# probe's.
if [ ! -f "$ROOT/cef_helper" ]; then
  echo "building cef_helper for the webview backend"
  cc -O2 -I"$ROOT" -DCEF_API_VERSION=13300 -o "$ROOT/cef_helper" helper/helper.c -ldl
fi

echo
echo "payload ready: $ROOT"
du -sh "$ROOT"
echo
echo "run:  go run -tags cef ./hack/cefprobe -dump out"
