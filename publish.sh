#!/usr/bin/env bash
# Hand a finished build to the team: after ./build.sh all, this walks the
# upload to the shared folder and then flips the update feed, so everyone's
# app shows "Update Available" with a link straight to the new files.
#
#   ./publish.sh          → publish the version in VERSION
#   ./publish.sh --yes    → skip the "did you upload?" question (files already up)
#
# Order matters: the feed is only switched AFTER the zips are in the shared
# folder — the moment the gist changes, teammates get the notification, and
# a link to a folder without the new files is worse than no notification.

set -euo pipefail
cd "$(dirname "$0")"

GIST_ID="4a70a4fb82706c462054048e942096b1"
APP_NAME="STPTunnel"
ASSUME_YES="${1:-}"

die() { echo "✗ $*" >&2; exit 1; }

for tool in jq gh curl; do
  command -v "$tool" >/dev/null || die "ต้องมี $tool ก่อน"
done
gh auth status >/dev/null 2>&1 || die "gh ยังไม่ได้ login — รัน: gh auth login (ต้องมี scope 'gist')"

VERSION="$(tr -d '[:space:]' < VERSION)"
FEED_VERSION="$(jq -r .version latest.json)"
SHARE_URL="$(jq -r .url latest.json)"
MAC_ZIP="dist/mac/${APP_NAME}-${VERSION}.zip"
WIN_ZIP="dist/win/${APP_NAME}-${VERSION}.zip"

echo "Publish ${APP_NAME} ${VERSION}"
echo

# ---------- 1. sanity checks before anyone gets notified ----------

[ "${FEED_VERSION}" == "${VERSION}" ] ||
  die "latest.json บอก ${FEED_VERSION} แต่ VERSION คือ ${VERSION} — รัน ./build.sh all ก่อน (มัน sync ให้)"

for zip in "${MAC_ZIP}" "${WIN_ZIP}"; do
  [ -f "${zip}" ] || die "ไม่เจอ ${zip} — รัน ./build.sh all ก่อน"
done

LIVE_VERSION="$(gh api "gists/${GIST_ID}" --jq '.files["latest.json"].content' | jq -r .version)"
if [ "${LIVE_VERSION}" == "${VERSION}" ]; then
  echo "✓ ทีมได้แจ้งเตือน ${VERSION} ไปแล้ว — ไม่มีอะไรต้องทำ"
  exit 0
fi

if [ -n "$(git status --porcelain -- latest.json VERSION CHANGELOG.md)" ]; then
  echo "⚠ latest.json / VERSION / CHANGELOG.md ยังไม่ได้ commit — publish ได้ แต่อย่าลืม commit + push ตามด้วย"
  echo
fi

echo "ตอนนี้ทีมเห็น ${LIVE_VERSION} → จะเปลี่ยนเป็น ${VERSION}"
echo "ข้อความที่ทีมจะเห็นใน dialog \"Update Available\":"
jq -r .notes latest.json | sed 's/^/    /'
echo

# ---------- 2. upload: the one step that has to be done by hand ----------

if [ "${ASSUME_YES}" != "--yes" ]; then
  [ -t 0 ] || die "ต้องรันใน terminal (หรือใส่ --yes ถ้าอัปโหลดไฟล์แล้ว)"

  echo "ขั้นที่ 1: อัปโหลดไฟล์พวกนี้ขึ้นโฟลเดอร์แชร์ (ตัว .zip เท่านั้น ห้ามลากโฟลเดอร์ .app):"
  echo "    ${MAC_ZIP}   (+ dist/mac/GUIDE.md)"
  echo "    ${WIN_ZIP}   (+ dist/win/GUIDE.md)"
  echo
  echo "→ เปิด Finder ที่ dist/ กับโฟลเดอร์แชร์ใน browser ให้แล้ว ลากไฟล์ข้ามไปได้เลย"
  open dist 2>/dev/null || true
  open "${SHARE_URL}" 2>/dev/null || true
  echo

  read -r -p "อัปโหลดครบทั้ง mac และ win แล้ว? [y/N] " ANSWER
  [[ "${ANSWER}" =~ ^[Yy] ]] || die "ยกเลิก — ยังไม่มีใครได้แจ้งเตือน รันใหม่เมื่ออัปโหลดเสร็จ"
fi

# ---------- 3. flip the feed ----------

echo
echo "ขั้นที่ 2: อัปเดต gist ที่แอปของทุกคนเช็ค"
jq -n --rawfile content latest.json '{files: {"latest.json": {content: $content}}}' |
  gh api "gists/${GIST_ID}" --method PATCH --input - >/dev/null

LIVE_VERSION="$(gh api "gists/${GIST_ID}" --jq '.files["latest.json"].content' | jq -r .version)"
[ "${LIVE_VERSION}" == "${VERSION}" ] || die "gist ยังบอก ${LIVE_VERSION} — ลองรันใหม่"

echo "✓ เรียบร้อย — ทีมจะเห็น \"Update Available ${VERSION}\" ตอนเปิดแอปครั้งถัดไป"
echo "  (หรือกด Help > Check for updates เอง; ไฟล์ raw บน gist อาจใช้เวลาไม่กี่นาทีกว่า cache จะหมด)"
