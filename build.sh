#!/usr/bin/env bash
# Build STP Tunnel for release, with the version stamped into the binary.
#
#   ./build.sh            → build for this machine only
#   ./build.sh all        → macOS .app + Windows .exe (Windows needs Docker running)
#   VERSION=1.2.0 ./build.sh all   → override the version (skips the auto-bump prompt below)
#
# The released version lives in VERSION (repo root) — that's the source of
# truth `./build.sh` (no args) falls back to. `./build.sh all` additionally
# looks at commits since the last v<version> tag: if any use the
# Conventional Commits feat:/fix:/BREAKING CHANGE prefix, it offers to bump
# VERSION for you before building. Setting VERSION explicitly always skips
# that prompt.
set -euo pipefail

VERSION_FILE="VERSION"
CURRENT_VERSION="$(cat "${VERSION_FILE}" 2>/dev/null || echo "1.0.0")"

if [ "${1:-local}" == "all" ] && [ -z "${VERSION:-}" ]; then
  LAST_TAG="v${CURRENT_VERSION}"
  if git rev-parse -q --verify "refs/tags/${LAST_TAG}" >/dev/null; then
    COMMIT_RANGE="${LAST_TAG}..HEAD"
  else
    COMMIT_RANGE="HEAD"
  fi
  COMMITS="$(git log --pretty=format:'%s' "${COMMIT_RANGE}" -- 2>/dev/null || true)"

  BUMP=""
  if echo "${COMMITS}" | grep -qE '^[a-z]+(\([^)]+\))?!:|^BREAKING CHANGE'; then
    BUMP="major"
  elif echo "${COMMITS}" | grep -qE '^feat(\([^)]+\))?:'; then
    BUMP="minor"
  elif echo "${COMMITS}" | grep -qE '^fix(\([^)]+\))?:'; then
    BUMP="patch"
  fi

  # -t 0 guards against hanging a non-interactive run (CI, etc.) — it just
  # builds the current VERSION unchanged in that case.
  if [ -n "${BUMP}" ] && [ -t 0 ]; then
    IFS='.' read -r MAJOR MINOR PATCH <<< "${CURRENT_VERSION}"
    case "${BUMP}" in
      major) NEXT_VERSION="$((MAJOR + 1)).0.0" ;;
      minor) NEXT_VERSION="${MAJOR}.$((MINOR + 1)).0" ;;
      patch) NEXT_VERSION="${MAJOR}.${MINOR}.$((PATCH + 1))" ;;
    esac
    echo "พบ commit ประเภท ${BUMP} ตั้งแต่ ${LAST_TAG}:"
    echo "${COMMITS}" | grep -E '^(feat|fix)(\([^)]+\))?!?:|^BREAKING CHANGE' | sed 's/^/  - /'
    read -r -p "Bump version ${CURRENT_VERSION} → ${NEXT_VERSION}? [Y/n] " ANSWER
    if [[ ! "${ANSWER}" =~ ^[Nn] ]]; then
      CURRENT_VERSION="${NEXT_VERSION}"
      echo "${CURRENT_VERSION}" > "${VERSION_FILE}"
      echo "→ VERSION → ${CURRENT_VERSION}. อย่าลืมอัปเดต CHANGELOG.md / README.md แล้ว commit + tag v${CURRENT_VERSION} เอง"
    fi
  fi
fi

VERSION="${VERSION:-${CURRENT_VERSION}}"
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
