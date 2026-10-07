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

# Keep latest.json's version field from silently drifting behind VERSION —
# this is exactly the "forgot to update it" trap README.md warns about
# (teammates press Check for updates and see nothing new). Only the version
# field is mechanical here; "notes" is filled from CHANGELOG.md further down.
echo "→ syncing version into latest.json"
jq --arg v "${VERSION}" '.version = $v' latest.json > latest.json.tmp && mv latest.json.tmp latest.json

# Offer to move CHANGELOG.md's [Unreleased] entries into a dated section for
# this version — a mechanical heading rename only, never generated content:
# what changed and why still needs a person. Skipped entirely if Unreleased
# is empty (nothing pending) or the run isn't interactive.
if [ -t 0 ]; then
  UNRELEASED_BLOCK="$(awk '/^## \[Unreleased\]$/{flag=1; next} /^## \[/{flag=0} flag' CHANGELOG.md | tr -d '[:space:]')"
  if [ -n "${UNRELEASED_BLOCK}" ]; then
    read -r -p "พบเนื้อหาใน CHANGELOG.md [Unreleased] — ย้ายเป็น [${VERSION}] - ${BUILD_DATE} เลยไหม? [Y/n] " ANSWER
    if [[ ! "${ANSWER}" =~ ^[Nn] ]]; then
      awk -v ver="${VERSION}" -v date="${BUILD_DATE}" '
        /^## \[Unreleased\]$/ && !done {
          print
          print ""
          print "## [" ver "] - " date
          done=1
          next
        }
        { print }
      ' CHANGELOG.md > CHANGELOG.md.tmp && mv CHANGELOG.md.tmp CHANGELOG.md
      echo "→ CHANGELOG.md: [Unreleased] → [${VERSION}] - ${BUILD_DATE}"
      echo "   (ยังต้องเพิ่มแถวเองในตาราง \"สรุป release\" ด้านบนของไฟล์)"
    fi
  fi
fi

# Release notes come from CHANGELOG.md's [VERSION] section (user-facing
# subsections only — see tools/relnotes). Generated before the slow build so
# a release with no notes stops here, not after ten minutes of compiling.
# Everything handed out lands in one folder per version, dragged to the
# shared folder as-is.
DIST_DIR="dist"
RELEASE_DIR="${DIST_DIR}/${VERSION}"
rm -rf "${RELEASE_DIR}"
mkdir -p "${RELEASE_DIR}"
echo "→ release notes from CHANGELOG.md [${VERSION}]"
if ! go run ./tools/relnotes -version "${VERSION}" -format txt > "${RELEASE_DIR}/RELEASE-NOTES.txt"; then
  echo "   เขียน release notes ของ ${VERSION} ใน CHANGELOG.md ก่อน (หัวข้อ Added / Changed / Fixed)" >&2
  exit 1
fi
SHORT_NOTES="$(go run ./tools/relnotes -version "${VERSION}" -format notes)"
jq --arg n "${SHORT_NOTES}" '.notes = $n' latest.json > latest.json.tmp && mv latest.json.tmp latest.json

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
MAC_DIR="${DIST_DIR}/mac"
mkdir -p "${MAC_DIR}"

echo "→ macOS app bundle (universal: arm64 + amd64)"
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

# Reuse whatever identity fyne just signed with (a real Developer ID from
# --release, or none) so the re-sign below doesn't downgrade a real
# signature to ad-hoc.
SIGN_IDENTITY="$(codesign -dvvv "${APP_NAME}.app" 2>&1 | awk -F'=' '/^Authority=/{print $2; exit}')"
SIGN_IDENTITY="${SIGN_IDENTITY:--}"

# `fyne package` only builds the executable for THIS machine's own CPU. On
# an Apple Silicon build host that's an arm64-only binary — a teammate on
# an Intel Mac then gets "The application ... can't be opened" with no
# further explanation (unlike Intel-on-arm64, there is no Rosetta path
# for arm64-on-Intel; it just can't execute). Cross-build the other arch —
# Xcode's clang can already target it without a separate cross toolchain —
# and lipo the two into one universal binary so the .app runs on either.
MAC_EXE="$(plutil -extract CFBundleExecutable raw "${APP_NAME}.app/Contents/Info.plist")"
NATIVE_ARCH="$(go env GOARCH)"
if [ "${NATIVE_ARCH}" = "arm64" ]; then
  OTHER_ARCH="amd64"; OTHER_TARGET="x86_64-apple-macos10.13"
else
  OTHER_ARCH="arm64"; OTHER_TARGET="arm64-apple-macos11"
fi
OTHER_BIN="$(mktemp)"
GOOS=darwin GOARCH="${OTHER_ARCH}" CGO_ENABLED=1 CC="clang -target ${OTHER_TARGET}" \
  go build -trimpath -ldflags "${LDFLAGS}" -o "${OTHER_BIN}" .
lipo -create -output "${APP_NAME}.app/Contents/MacOS/${MAC_EXE}" \
  "${APP_NAME}.app/Contents/MacOS/${MAC_EXE}" "${OTHER_BIN}"
rm -f "${OTHER_BIN}"

# Replacing the executable invalidates fyne's seal — re-sign so Gatekeeper
# sees a valid signature covering both architectures.
codesign --force --deep -s "${SIGN_IDENTITY}" "${APP_NAME}.app"

mv "${APP_NAME}.app" "${MAC_DIR}/"

# Hand out a real zip, not the loose .app folder. A .app *is* a folder, so
# dragging it straight into Google Drive/Slack/etc. without zipping first
# means the sharing service silently re-zips it on every download — and
# that re-zip drops the executable permission bit on the binary inside
# Contents/MacOS, so teammates get "can't be opened" no matter their CPU.
# `ditto` is what Finder's own "Compress" uses, so it round-trips Unix
# permissions and resource forks correctly, unlike a plain `zip`.
# The zip holds the .app plus RELEASE-NOTES.txt, so a zip forwarded on its
# own still says what it is.
MAC_STAGE="$(mktemp -d)"
ditto --norsrc --noextattr --noacl "${MAC_DIR}/${APP_NAME}.app" "${MAC_STAGE}/${APP_NAME}.app"
cp "${RELEASE_DIR}/RELEASE-NOTES.txt" "${MAC_STAGE}/"
ditto -c -k --norsrc --noextattr --noacl "${MAC_STAGE}" "${RELEASE_DIR}/${APP_NAME}-${VERSION}-mac.zip"
rm -rf "${MAC_STAGE}"

echo "→ Windows executable (via Docker)"
if ! docker ps >/dev/null 2>&1; then
  echo "   Docker isn't running — skipping the Windows build." >&2
  exit 1
fi
fyne-cross windows -arch=amd64 -icon icon.png -app-id "${APP_ID}" -name "${APP_NAME}" \
  -app-version "${VERSION}"
WIN_STAGE="$(mktemp -d)"
unzip -q "fyne-cross/dist/windows-amd64/${APP_NAME}.zip" -d "${WIN_STAGE}"
cp "${RELEASE_DIR}/RELEASE-NOTES.txt" "${WIN_STAGE}/"
WIN_ZIP="$(pwd)/${RELEASE_DIR}/${APP_NAME}-${VERSION}-win.zip"
(cd "${WIN_STAGE}" && zip -q -X -r "${WIN_ZIP}" .)
rm -rf "${WIN_STAGE}"

# Guides for both platforms + checksums, so anyone can confirm a download
# arrived intact (shasum -a 256 -c SHA256SUMS.txt / certutil on Windows).
cp GUIDE-macOS.md GUIDE-Windows.md "${RELEASE_DIR}/"
(cd "${RELEASE_DIR}" && shasum -a 256 ./*.zip | sed 's#  \./#  #' > SHA256SUMS.txt)
{
  echo
  echo "ตรวจสอบไฟล์ (SHA-256)"
  sed 's/^/  /' "${RELEASE_DIR}/SHA256SUMS.txt"
} >> "${RELEASE_DIR}/RELEASE-NOTES.txt"

echo
echo "พร้อมแจก — ลากทั้งโฟลเดอร์ ${RELEASE_DIR}/ ขึ้นโฟลเดอร์แชร์ (ห้ามลาก .app เปล่าๆ:"
echo "บริการแชร์ไฟล์จะ zip ใหม่ตอนดาวน์โหลดแล้ว permission หาย เปิดแอปไม่ได้):"
ls -1 "${RELEASE_DIR}" | sed 's/^/  /'
echo
echo "ขั้นที่เหลือ:"
echo "  1. git add -A && git commit -m \"chore: release ${VERSION}\" (latest.json ได้ notes ใหม่จาก CHANGELOG)"
echo "  2. git tag -a v${VERSION} -m \"STP Tunnel ${VERSION}\" && git push && git push --tags"
echo "  3. ./publish.sh — อัปโหลดโฟลเดอร์ ${RELEASE_DIR}/ แล้วสลับ gist ให้ทีมเห็นแจ้งเตือน"
