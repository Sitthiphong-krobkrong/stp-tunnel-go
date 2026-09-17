#!/usr/bin/env bash
# Build STP Tunnel for release, with the version stamped into the binary.
#
#   ./build.sh            → build for this machine only
#   ./build.sh all        → macOS .app + Windows .exe (Windows needs Docker running)
#   VERSION=1.2.0 ./build.sh all   → override the version
#
# Bump VERSION for each release you hand out.
set -euo pipefail

VERSION="${VERSION:-1.0.0}"
BUILD_DATE="$(date +%F)"
LDFLAGS="-X main.version=${VERSION} -X main.buildDate=${BUILD_DATE}"
APP_ID="com.stp.tunnel"
APP_NAME="STPTunnel"

echo "Building ${APP_NAME} ${VERSION}"

echo "→ local binary"
# -trimpath strips this machine's absolute file paths (/Users/<you>/...) from
# the binary — without it they sit in plaintext inside the compiled output,
# visible to anyone who runs `strings` on it or ever sees a panic trace.
# fyne package/fyne-cross already strip this for the real dist/ builds below;
# this keeps the plain dev binary consistent even though it never ships.
go build -trimpath -ldflags "${LDFLAGS}" -o stp-tunnel .

if [ "${1:-local}" != "all" ]; then
  echo "Done: ./stp-tunnel"
  exit 0
fi

# `fyne package` and `fyne-cross` have no working way to pass "-X pkg.var=val"
# through to the linker — fyne-cross splits -ldflags on whitespace and
# re-emits each piece as its own GOFLAGS entry (internal/command/context.go),
# which current Go no longer reassembles, so `-X main.version=...` arrives at
# the linker as a bare, argument-less "-X" and the build fails outright.
# Cheaper than patching upstream: stamp the version by editing release.go's
# defaults for the duration of this build, then always put it back.
RELEASE_GO_BAK="$(mktemp)"
cp release.go "${RELEASE_GO_BAK}"
trap 'cp "${RELEASE_GO_BAK}" release.go; rm -f "${RELEASE_GO_BAK}"' EXIT
sed -i '' \
  -e "s/version   = \"[^\"]*\"/version   = \"${VERSION}\"/" \
  -e "s/buildDate = \"[^\"]*\"/buildDate = \"${BUILD_DATE}\"/" \
  release.go

# Everything actually handed out to teammates lands here, one subfolder per
# platform, instead of scattered between the project root and fyne-cross's
# own bin/dist/tmp tree.
DIST_DIR="dist"
MAC_DIR="${DIST_DIR}/mac"
WIN_DIR="${DIST_DIR}/win"
mkdir -p "${MAC_DIR}" "${WIN_DIR}"

echo "→ macOS app bundle"
rm -rf "${APP_NAME}.app" "${MAC_DIR}/${APP_NAME}.app"
# --release wants a codesigning identity, which this app doesn't have yet
# (see README), so it can fail — fall back to an unsigned package. Both
# attempts must carry -app-version: an earlier version of this script
# dropped it from the fallback, so the .app would silently package with
# fyne's own default "1.0.0" no matter what VERSION was set to.
if ! fyne package -os darwin -icon icon.png -name "${APP_NAME}" -app-id "${APP_ID}" \
  -app-version "${VERSION}" --release; then
  echo "   --release packaging failed (no codesigning identity?) — falling back to unsigned." >&2
  fyne package -os darwin -icon icon.png -name "${APP_NAME}" -app-id "${APP_ID}" \
    -app-version "${VERSION}"
fi
mv "${APP_NAME}.app" "${MAC_DIR}/"
cp GUIDE-macOS.md "${MAC_DIR}/GUIDE.md"

echo "→ Windows executable (via Docker)"
if ! docker ps >/dev/null 2>&1; then
  echo "   Docker isn't running — skipping the Windows build." >&2
  exit 1
fi
fyne-cross windows -arch=amd64 -icon icon.png -app-id "${APP_ID}" -name "${APP_NAME}" \
  -app-version "${VERSION}"
cp "fyne-cross/dist/windows-amd64/${APP_NAME}.zip" "${WIN_DIR}/${APP_NAME}-${VERSION}.zip"
cp GUIDE-Windows.md "${WIN_DIR}/GUIDE.md"

echo
echo "Ready to hand out — zip/share the folder as-is, GUIDE.md is already inside:"
echo "  ${MAC_DIR}/  (${APP_NAME}.app + GUIDE.md)"
echo "  ${WIN_DIR}/  (${APP_NAME}-${VERSION}.zip + GUIDE.md)"
