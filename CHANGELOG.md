# Changelog

ทุกการเปลี่ยนแปลงที่ส่งผลกับผู้ใช้จะบันทึกไว้ที่นี่ รูปแบบอ้างอิงจาก [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) เลขเวอร์ชันอ้างอิง [Semantic Versioning](https://semver.org/) (`MAJOR.MINOR.PATCH`)

อัปเดตไฟล์นี้พร้อมกับตอนที่ bump `VERSION` ใน [build.sh](build.sh) ทุกครั้งที่แจกรอบใหม่ — ดูหัวข้อ "Build สำหรับแจกจ่าย" ใน [README.md](README.md)

## เขียน entry ยังไง

หัวข้อ **Added / Changed / Fixed / Security / Known issues** คือสิ่งที่ผู้ใช้เห็น — `./build.sh all` ดึงไปทำเป็น `RELEASE-NOTES.txt` ที่แจกคู่กับไฟล์ zip และข้อความใน dialog "Update Available" ของแอป ([tools/relnotes](tools/relnotes/main.go)) เขียนให้คนใช้แอปอ่านรู้เรื่อง: ทำอะไรได้ใหม่ / อะไรเปลี่ยน / อะไรหายพัง สั้นๆ ไม่ใส่ชื่อไฟล์ ชื่อฟังก์ชัน หรือ path

รายละเอียดฝั่ง dev (ไฟล์ไหน ทำไม วิธีทดสอบ) ใส่ใน **Internal** — ไม่ออกไปถึงผู้ใช้

## สรุป release

เพิ่มแถวใหม่ไว้บนสุดทุกครั้งที่ bump version — รายละเอียดเต็มของแต่ละเวอร์ชันอยู่ด้านล่าง

| Version | วันที่ | สรุป |
|---|---|---|
| 1.2.1 | 2026-10-07 | ฟอนต์ไทย Noto Sans Thai ฝังในแอป — หน้าตาเหมือนกันทุกเครื่อง ตัวหนาไทยหนาจริง |
| 1.2.0 | 2026-10-07 | Export/Import เลือกได้ราย project (+ ตั้งชื่อใหม่เมื่อชื่อ project ชนกัน), คู่มือ Getting started 8 ขั้นด้วยภาพหน้าจอจริงเด้งตอนเปิดแอปครั้งแรก, ภาพ diagram ใน README |
| 1.1.0 | 2026-09-18 | ปุ่ม Test connection ก่อนใช้จริง, macOS build เป็น universal binary (arm64+amd64) + zip ด้วย `ditto` อัตโนมัติ (แก้ปัญหาเปิดไม่ได้บน Intel Mac และตอนอัปขึ้น Google Drive), เปิดใช้ check-for-updates จริง, เมนู (⋮) สำหรับ diagnostics/update, ยืนยันก่อนปิดแอปถ้ายังมี tunnel รันอยู่ |
| 1.0.0 | 2026-09-17 | เวอร์ชันแรก — tunnel manager ครบชุด, system tray menu, คอลัมน์ Latency, ไอคอนแอปใหม่ |

## [Unreleased]

## [1.2.1] - 2026-10-07

### Fixed

- ตัวอักษรไทยในแอปหน้าตาเหมือนกันทั้ง Mac และ Windows และตัวหนาภาษาไทย (หัวข้อ, ปุ่ม) เป็นตัวหนาจริงแล้ว — เดิมไทยบางกว่าอังกฤษที่อยู่ข้างกัน

### Internal

- ฝังฟอนต์ Noto Sans Thai (Regular/Bold) ไว้ในแอป ([fonts/](fonts/), SIL Open Font License — [fonts/OFL.txt](fonts/OFL.txt)) เป็นฟอนต์หลักของธีม ตัวอังกฤษที่ไม่มีในฟอนต์นี้ Fyne ใช้ Noto Sans ตัวเดิมต่อให้เอง ฟอนต์ mono ไม่เปลี่ยน
- สร้างภาพหน้าจอใน docs/guide/ ใหม่ด้วยฟอนต์ใหม่

## [1.2.0] - 2026-10-07

### Added

- คู่มือ Getting started เด้งขึ้นมาตอนเปิดแอปครั้งแรก — 8 ขั้นตอนพร้อมภาพหน้าจอจริงและกรอบแดงบอกว่าต้องกดตรงไหน เปิดดูซ้ำได้ที่ Help > Getting started หรือปุ่ม ⋮ มุมขวาบน
- Export setup เลือกได้ว่าจะแชร์ project ไหน (ค่าเริ่มต้นคือ project ที่เปิดอยู่) และตั้งชื่อไฟล์ตามชื่อ project ให้
- Import setup ถ้าในไฟล์มีหลาย project เลือกได้ว่าจะเอาอันไหน บอกด้วยว่าอันไหนใหม่ / มีอยู่แล้ว / มี tunnel ใหม่เพิ่ม และสลับไปที่ project ที่เพิ่ง import ให้ทันที

### Fixed

- Import project ที่ชื่อซ้ำกับ project ที่มีอยู่แล้ว (แต่เป็นคนละอัน) เดิมทำให้รายการ Project มีชื่อซ้ำสองอันและเลือกผิดอันได้ ตอนนี้ตั้งชื่อใหม่เป็น "ชื่อ (2)" ให้อัตโนมัติ

### Internal

- คู่มือจำว่าเคยดูแล้วด้วยไฟล์ `guide-seen` ข้าง `config.json` (Fyne Preferences ไม่บันทึกลงดิสก์ถ้าไม่มี app ID เช่นตอน `go run`) ภาพหน้าจอใน [docs/guide/](docs/guide/) สร้างจาก UI จริงด้วย `go test -tags screenshots -run TestGuideScreenshots .`
- ภาพ diagram ใน README (ภาพรวม / ตอนกด Start / สถานะ tunnel) ต้นฉบับ `.json` อยู่ใน [docs/architecture/](docs/architecture/)
- `./publish.sh` — ปล่อยอัปเดตให้ทีม: เช็คว่าไฟล์ครบ, เปิดโฟลเดอร์ให้ลากขึ้นโฟลเดอร์แชร์, รอยืนยันว่าอัปโหลดแล้วจึงค่อย sync gist
- Release notes สร้างจาก CHANGELOG อัตโนมัติ ([tools/relnotes](tools/relnotes/main.go)) — `RELEASE-NOTES.txt` และ `notes` ใน `latest.json` ใช้เฉพาะหัวข้อ Added / Changed / Fixed / Security / Known issues ส่วน Internal ไม่ออกไปถึงผู้ใช้ และ `./build.sh all` จะหยุดถ้าเวอร์ชันนั้นไม่มี release notes
- `./build.sh all` ส่งออกเป็นโฟลเดอร์เดียวต่อเวอร์ชัน `dist/<version>/` (zip ของ mac/win ชื่อไม่ชนกันแล้ว + RELEASE-NOTES.txt + GUIDE ทั้งสอง OS + SHA256SUMS.txt) ลากทั้งโฟลเดอร์ขึ้นโฟลเดอร์แชร์ได้เลย

## [1.1.0] - 2026-09-18

### Added

- [VERSION](VERSION) เป็น source of truth เดียวของเลขเวอร์ชัน — `./build.sh all` เช็ค commit ตั้งแต่ tag `v<เวอร์ชันปัจจุบัน>` อัตโนมัติ ถ้าเจอ prefix แบบ Conventional Commits (`feat:`/`fix:`/`BREAKING CHANGE`) จะถามก่อนว่าจะ bump เวอร์ชันให้ไหม (ดู "Build สำหรับแจกจ่าย" ใน [README.md](README.md))
- เปิดใช้ "Check for updates" จริง — [latest.json](latest.json) ใหม่ + `updateFeedURL` ใน [release.go](release.go) ชี้ไปที่ public GitHub Gist ที่ sync มาจากไฟล์นั้น เดิมค่านี้ว่างเปล่าฟีเจอร์เลยไม่เคยทำงาน (ตอนแรกลองชี้ตรงไป raw.githubusercontent.com ของ repo นี้ แต่ repo private เลยโดน 404 เสมอ — ย้ายมาใช้ gist แทนเพราะ repo คงต้องเป็น private ต่อไป ดู "แจ้งเตือนเวอร์ชันใหม่" ใน [README.md](README.md))
- เพิ่มไอคอนเมนู (⋮) มุมขวาบนของหน้าต่างหลัก กดแล้วเป็น dropdown "Copy diagnostics" / "Check for updates" — เดิมสองอันนี้อยู่แค่ในเมนู Help บนแถบเมนูระบบซึ่งพลาดง่าย
- ถามยืนยันก่อนปิดแอปถ้ายังมี tunnel รันอยู่ ("N tunnel(s) are still running. Quitting will stop them all.") — ปิดแอปทันทีเหมือนเดิมถ้าไม่มี tunnel ไหนรันอยู่
- [GUIDE-macOS.md](GUIDE-macOS.md) / [GUIDE-Windows.md](GUIDE-Windows.md) — คู่มือย่อสำหรับผู้ใช้ปลายทาง แยกไฟล์ตาม OS ไม่ปนกัน `build.sh all` ก็อปไฟล์ที่ตรง OS ใส่เป็น `GUIDE.md` ใน `dist/mac/` และ `dist/win/` ให้อัตโนมัติ แจกทีมได้โดยไม่ต้องทำอะไรเพิ่ม
- ปุ่ม **"Test connection"** ใน New/Edit Project — ลองต่อ SSH ด้วยค่าที่กรอกในฟอร์ม (host/user/port/identity file/skip host key) ได้ทันทีโดยไม่ต้อง Save หรือไปสร้าง tunnel ก่อน ใช้ auth logic เดียวกับตอนกด Start (key file → password ที่จำไว้ → ถามรหัสผ่าน)

### Fixed

- แก้ tunnel ที่ RUNNING อยู่แล้วกด Edit เปลี่ยน Local port/Target host/Target port — เดิมค่าใหม่ขึ้นในตารางทันทีแต่ตัวท่อจริงยังวิ่งด้วยค่าเก่าเงียบๆ ไม่มีใครรู้ ตอนนี้เตือนให้ Stop/Start ใหม่ถ้าเปลี่ยนค่าพวกนี้ระหว่างที่ tunnel รันอยู่
- แก้ race condition ใน `TunnelRuntime`: ถ้า Stop แล้ว Start tunnel เดิมเร็วมากๆ ติดกัน goroutine ที่เฝ้า session เก่าอาจไปลบ/รายงานว่า session **ใหม่** หลุดแทน ทำให้ tunnel ใหม่ที่ยังรันอยู่จริงถูกมองว่าเป็น orphan (Stop ไม่ได้, Start ใหม่ชน "address already in use")
- แก้ binary ที่ build ด้วย `go build` เฉยๆ (ลืม `-ldflags`) เคยรายงานตัวเองว่า `1.0.0` เหมือน release จริงแบ่งแยกไม่ออก — ตอนนี้ fallback เป็น `dev` แทน
- เพิ่ม log ตอนหา home directory ไม่เจอแล้วต้องปิด host-key verification ให้ (เดิมปิดเงียบๆ ไม่ขึ้นใน LOG panel เลย)
- ลบ `fileExists` ที่ไม่ได้ใช้ที่ไหนแล้ว (dead code)
- `build.sh` เดิม build .app ให้แค่ CPU ของเครื่องที่ build (arm64 บน Apple Silicon) — เครื่องทีมที่เป็น Intel Mac เปิดแอปไม่ได้เลย ("The application ... can't be opened") ตอนนี้ cross-build อีก arch แล้ว `lipo` รวมเป็น universal binary ก่อน re-sign
- `build.sh` เดิมส่งมอบแค่โฟลเดอร์ `.app` เปล่าๆ ให้ไปจัดการเอง — ถ้าลากขึ้น Google Drive/Slack ตรงๆ โดยไม่ zip ก่อน บริการเหล่านั้นจะ auto-zip ให้ตอนดาวน์โหลดแล้วทำ Unix executable permission ของ binary ข้างในหาย เปิดไม่ได้ไม่ว่า CPU รุ่นไหน ตอนนี้ `build.sh` zip ด้วย `ditto` ให้เสร็จเป็น `STPTunnel-<version>.zip` พร้อมแจกทันที
- [GUIDE-macOS.md](GUIDE-macOS.md) เพิ่มขั้นตอนสำหรับ dialog "Apple could not verify ... is free of malware" (ไม่มีปุ่ม Open Anyway ให้กดตรงๆ) ที่เจอใน macOS รุ่นใหม่ๆ เช่น Sonoma/Sequoia — ต้องไป System Settings → Privacy & Security แทน

### Known issues

- **System tray icon ไม่โชว์บน macOS** — `fyne.io/systray` เวอร์ชันที่ Fyne v2.8.1 บังคับใช้มีบั๊ก: status item ลงทะเบียนกับ Control Center สำเร็จ แต่ไอคอนไม่ถูก apply เลยมองไม่เห็น (upstream: [fyne-io/fyne#4548](https://github.com/fyne-io/fyne/issues/4548)) เมนู tray (Show/Stop all/Quit) ยังอยู่ในโค้ดเผื่อแก้ได้ในอนาคต แต่**ปิดพฤติกรรม "กด X แล้วซ่อนลง tray" ไว้ก่อน** — ถ้าเปิดทั้งที่ไอคอนมองไม่เห็น ผู้ใช้จะกด X แล้วแอปหายไปเลย เรียกกลับไม่ได้นอกจาก Force Quit ตอนนี้กด X = quit ปกติเหมือนเดิม

## [1.0.0] - 2026-09-17

### Added

- จัดการหลาย project (bastion server) แต่ละ project เปิดได้หลาย tunnel (`-L` port-forward) พร้อมกัน
- Auth ด้วย password (เก็บใน OS keychain) หรือ identity file (`~/.ssh/...`)
- ตรวจ host key กับ `~/.ssh/known_hosts` พร้อม retry อัตโนมัติเมื่อชนิด key ไม่ตรง
- Import/Export setup เป็นไฟล์ JSON เดียว แชร์ให้ทีมได้โดยไม่มี password ติดไปด้วย
- **System tray menu** — Show / Stop all tunnels / Quit (ไอคอนบน macOS ยังมีปัญหา ดู "Known issues" ด้านบน)
- **คอลัมน์ Latency** — heartbeat วัด round-trip ไปที่ bastion ทุก 5 วินาทีต่อ tunnel ที่ RUNNING โดยไม่แตะ target จริง
- ไอคอนแอปใหม่ generate ด้วย Go ล้วน ([tools/genicon](tools/genicon)) แทนไอคอนสีฟ้าแบนๆ เดิม
- Help → Copy diagnostics และ Check for updates

### Fixed

- `build.sh`: `fyne-cross` ใส่ `-ldflags "-X ..."` ไม่ได้ (บั๊กใน fyne-cross เอง ตัด string ตาม whitespace แล้วพัง) — เปลี่ยนไป stamp version ผ่านการแก้ default ใน `release.go` ชั่วคราวระหว่าง build แทน
- `build.sh`: จุด fallback ตอน mac `--release` build ล้มเหลว (ไม่มี code-signing certificate) เคยลืมใส่ `-app-version` ทำให้ `.app` ได้เลขเวอร์ชัน `1.0.0` ของ fyne เองเสมอ ไม่ว่าจะตั้ง `VERSION` เป็นอะไรก็ตาม
