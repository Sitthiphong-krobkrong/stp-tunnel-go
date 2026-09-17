# STP Tunnel

`stp-tunnel` · part of the **STP Tools** family

**เวอร์ชัน:** 1.0.0 · **อัปเดตล่าสุด:** 2026-09-17 · **ผู้พัฒนา:** stp · [CHANGELOG](CHANGELOG.md)

A small desktop app for opening SSH port-forwards (tunnels) to internal servers — one window, a list of saved tunnels, start/stop with a click.

แอปสำหรับเปิด SSH tunnel ไปยัง server ภายใน โดยไม่ต้องจำคำสั่ง `ssh -L` และไม่ต้องเปิด terminal ทิ้งไว้

---

## ทำไมต้องมีตัวนี้

### ปัญหา: เครื่องเราต่อ server ภายในตรงๆ ไม่ได้

```
    เครื่องคุณ                              Internal network
  ┌──────────────┐                    ┌──────────────────────────┐
  │              │                    │  Camunda    172.x.x:8080 │
  │   Laptop     │ ─────  ✗  ───────▶ │  PostgreSQL 172.x.x:5432 │
  │              │   ต่อไม่ถึง          │  MongoDB    172.x.x:27017│
  └──────────────┘                    └──────────────────────────┘

  service พวกนี้ตั้งใจไม่เปิดออก internet (ปลอดภัยกว่า)
  แต่ก็แปลว่าเราเข้าไม่ถึงจากเครื่องตัวเองด้วย
```

### ทางออก: ยืมทางผ่าน server ที่เรา SSH เข้าได้

```
    เครื่องคุณ              Bastion server              Internal network
  ┌──────────────┐        ┌──────────────┐        ┌──────────────────────┐
  │localhost:8080│ ══SSH══▶│              │───────▶│ Camunda    :8080     │
  │localhost:5432│ ══SSH══▶│ 172.31.168.  │───────▶│ PostgreSQL :5432     │
  │localhost:7000│ ══SSH══▶│     144      │───────▶│ MongoDB    :27017    │
  └──────────────┘        └──────────────┘        └──────────────────────┘
         ▲                 เครื่องนี้เรา SSH เข้าได้        เครื่องนี้ bastion คุยได้
         │
    เปิด browser / DB client มาที่ localhost ได้เลย
    เหมือน service รันอยู่บนเครื่องเราเอง
```

### แล้วแอปนี้เข้ามาช่วยตรงไหน

```
  ทำเองด้วย terminal                      ใช้ stp-tunnel
  ─────────────────────                  ─────────────────────
  เปิด terminal                           เปิดแอป
  พิมพ์ ssh -L 8080:172.x:8080 ...        ติ๊กช่องหน้า service
  ทิ้ง terminal เปิดค้างไว้                 กด Start
  เปิด terminal ใหม่อีกอัน                  ครบทุกตัวพร้อมกัน
  พิมพ์คำสั่งยาวๆ อีกรอบ                    ─────────────────────
  ...ทำซ้ำทุก service                      ปิดเครื่อง เปิดใหม่:
  ปิดเครื่อง เปิดใหม่ = พิมพ์ใหม่ทั้งหมด        รายการยังอยู่ครบ กด Start ต่อได้เลย
```

### เกิดอะไรขึ้นตอนกด Start

```
  กด Start
     │
     ▼
  ถาม password  ────────▶ (ครั้งแรกของรอบเท่านั้น จำไว้ในหน่วยความจำ)
     │
     ▼
  ต่อ SSH ไปที่ bastion
     │
     ▼
  เปิด port บนเครื่องคุณ (localhost:8080)
     │
     ▼
  ทุก connection ที่เข้า port นี้ → ส่งผ่านท่อ SSH → ออกที่ปลายทาง
     │
     ▼
  สถานะเปลี่ยนเป็น RUNNING  ◀────  กด Stop เมื่อไหร่ก็ปิดท่อทันที
```

**tunnel แต่ละอันมีท่อ SSH ของตัวเอง** — ตัวหนึ่งหลุดหรือถูกปิด ตัวอื่นไม่กระทบ

---

## สำหรับผู้ใช้งาน

> ส่วนนี้มีฉบับย่อแยกตาม OS ไว้ที่ [GUIDE-macOS.md](GUIDE-macOS.md) และ [GUIDE-Windows.md](GUIDE-Windows.md) ด้วย — `./build.sh all` จะก็อปไฟล์ที่ตรง OS ใส่ไปเป็น `GUIDE.md` ใน `dist/mac/` และ `dist/win/` ให้อัตโนมัติทุกครั้ง (ผู้ใช้แต่ละฝั่งเห็นแค่คู่มือของ OS ตัวเอง ไม่ปนกัน) เอาไปแจกทีมได้เลยไม่ต้องทำอะไรเพิ่ม

### ติดตั้ง

ดาวน์โหลดไฟล์เดียวแล้วเปิดได้เลย — **ไม่ต้องติดตั้งอะไรเพิ่ม** ไม่ต้องมี Go, ไม่ต้องมี OpenSSH, ไม่ต้องมี runtime ใดๆ

| ระบบปฏิบัติการ | ไฟล์ | ครั้งแรกที่เปิด |
|---|---|---|
| macOS | `STPTunnel.app` | **คลิกขวา → Open** แล้วกด Open ยืนยัน (ถ้า double-click ธรรมดาจะโดน Gatekeeper บล็อก) |
| Windows | `STPTunnel.exe` | ถ้าขึ้น "Windows protected your PC" ให้กด **More info → Run anyway** |

ทำแบบนี้ครั้งเดียวต่อเครื่อง หลังจากนั้นเปิดได้ปกติ (เกิดขึ้นเพราะแอปยังไม่ได้ซื้อ code signing certificate)

### เริ่มใช้งาน

1. **New** → สร้าง project (1 project = 1 server ที่จะ SSH เข้าไป)
   - Name — ชื่อเรียกของคุณเอง เช่น `bma-oss1`
   - Bastion host — IP หรือ hostname ของ server เช่น `172.31.168.144`
   - Bastion user — username ที่ใช้ login เช่น `sysadmin`
   - Bastion port — ปกติ `22`
2. **Add tunnel** → ระบุว่าจะ forward อะไร
   - Service name — ชื่อเรียก เช่น `camunda`
   - Local port — port บนเครื่องคุณ เช่น `8080`
   - Target host — ปลายทาง **เมื่อมองจากฝั่ง server** ปกติคือ `localhost`
   - Target port — port ของ service ปลายทาง เช่น `8080`
3. ติ๊ก checkbox หน้า tunnel ที่ต้องการ → กด **Start** → ใส่ password
4. เปิดใช้งานผ่าน `localhost:<Local port>` บนเครื่องตัวเอง (คลิกที่คอลัมน์ URL ได้เลย)

คอลัมน์ **Latency** บอกว่า tunnel ที่ RUNNING อยู่ยังต่อกับ bastion ไหวอยู่ไหม — วัดจาก heartbeat ทุก 5 วินาที ถ้าขึ้น `timeout` แปลว่าท่อกำลังจะหลุด (สถานะจะเปลี่ยนเป็น ERROR ตามมาไม่นาน)

### กด X ปิดหน้าต่าง = ปิดแอป (หยุดทุก tunnel ด้วย)

ปกติของแอป desktop ทั่วไป — ไม่ได้ซ่อนลง tray ให้ (ดูหมายเหตุด้านล่างว่าทำไม)

### ตัวอย่างที่เห็นภาพ

ต้องการต่อ database ที่ไม่เปิด public แต่ server `172.31.168.144` เข้าถึงได้:

```
Local port  : 5432          ← เปิดที่เครื่องคุณ
Target host : db.internal   ← ชื่อเครื่อง DB เมื่อมองจากฝั่ง server
Target port : 5432          ← port ของ Postgres
```

กด Start แล้วชี้ DB client มาที่ `localhost:5432` ได้เลย

### Password และ key

- **ปกติใช้ password** — แอปถามครั้งแรกที่กด Start ติ๊ก *Remember this password* ไว้แล้วครั้งต่อไปไม่ต้องกรอกอีก
- **password ถูกเก็บใน keychain ของระบบปฏิบัติการ** (macOS Keychain / Windows Credential Manager) ไม่ได้อยู่ในไฟล์ config และไม่ติดไปกับไฟล์ export
  - ครั้งแรกที่บันทึก macOS จะถามว่าอนุญาตให้แอปใช้ Keychain ไหม → กด **Always Allow**
- **เปลี่ยน password บน server แล้ว?** กด Edit ที่ project → **Forget saved password** แล้วกรอกใหม่ครั้งถัดไป
- **ถ้า server ปิด password auth** — ใส่ path ของ private key เดิมที่ใช้ได้อยู่แล้ว (เช่น `~/.ssh/id_rsa`) ในช่อง **Identity file** ของ project (กด Edit)

### แชร์ให้ทั้งทีม (ไม่ต้องกรอกใหม่ทีละคน)

คนที่ตั้งค่าไว้แล้ว → เมนู **File → Export setup...** ได้ไฟล์ `stp-tunnel-setup.json` → ส่งให้เพื่อน
เพื่อน → เมนู **File → Import setup...** เลือกไฟล์นั้น → ได้ project และ tunnel ครบทันที เหลือแค่ใส่ password ของตัวเอง

ไฟล์นี้**ไม่มี password อยู่ข้างใน** ส่งทางแชทได้ปลอดภัย และถ้า import ซ้ำของที่มีอยู่แล้ว ระบบจะข้ามให้ ไม่เกิดรายการซ้ำ

### แจ้งปัญหา

เมนู **Help → Copy diagnostics** จะก็อปข้อมูลที่จำเป็นทั้งหมดไว้ใน clipboard (เวอร์ชัน, OS, การตั้งค่า project, สถานะ tunnel, log 20 บรรทัดล่าสุด — **ไม่มี password**) แล้วแปะส่งมาได้เลย ไม่ต้องถามกลับไปมาว่าใช้เวอร์ชันไหน error ว่าอะไร

### เจอปัญหาบ่อยๆ

| ข้อความใน Log | สาเหตุ / วิธีแก้ |
|---|---|
| `Permission denied` | user หรือ password ผิด — กด Edit ที่ project เช็คอีกที |
| `host key ... does not match ~/.ssh/known_hosts` | server ถูก rebuild หรือ IP ถูกเวียนใช้ใหม่ → รัน `ssh-keygen -R <host>` เพื่อลบ entry เก่า หรือติ๊ก **Skip host key check** ในฟอร์ม Edit project |
| `listen on local port ...: address already in use` | Local port นั้นมีโปรแกรมอื่นใช้อยู่ → เปลี่ยนเป็น port อื่น |
| `cannot reach <target>` | ต่อ server ได้ แต่ server เข้าถึงปลายทางไม่ได้ → เช็ค Target host / Target port |

### ข้อมูลถูกเก็บไว้ที่ไหน

รายการ project และ tunnel ทั้งหมดถูกบันทึกอัตโนมัติที่:

- macOS — `~/Library/Application Support/stp/tunnel/config.json`
- Windows — `%AppData%\stp\tunnel\config.json`

ลบไฟล์นี้ = เริ่มใหม่หมด (password ไม่ได้อยู่ในไฟล์นี้)

---

## สำหรับผู้พัฒนา

### Stack

| ส่วน | ใช้อะไร |
|---|---|
| ภาษา | Go |
| GUI | [Fyne](https://fyne.io) v2 (pure Go, compile รวมเป็น binary เดียว ไม่พึ่ง webview) |
| SSH | `golang.org/x/crypto/ssh` — ต่อ SSH เองในโปรเซส ไม่ได้เรียก `ssh` ของระบบ |
| เก็บ password | `github.com/zalando/go-keyring` — ฝากไว้กับ keychain ของ OS |
| เก็บข้อมูล | JSON ไฟล์เดียวใน user config dir |

### ทำไมเขียนด้วย Go

**เหมาะ** เพราะแกนกลางของแอปคือ "หลายท่อ 2 ทาง ทำงานพร้อมกัน":

```
  1 tunnel = goroutine 3-4 ตัวทำงานขนานกัน
  ┌────────────────────────────────────────────┐
  │ acceptLoop   → รอ connection เข้า port      │
  │ client.Wait  → เฝ้าว่า SSH หลุดไหม           │
  │ io.Copy ×2   → ส่งข้อมูลไป-กลับ             │
  └────────────────────────────────────────────┘
      5 tunnel = ~20 goroutine — โค้ดยังอ่านเหมือนเขียนแบบตรงไปตรงมา
```

- **port forwarding เขียนด้วย stdlib ล้วนไม่กี่บรรทัด** — `net.Listen` → `Accept` → `client.Dial` → `io.Copy` สองทาง
- **ได้ไฟล์เดียว static-link** ตรงกับโจทย์ "แจกทีมแล้วใช้ได้เลย"
- **`golang.org/x/crypto/ssh`** เป็น SSH implementation ที่ทีม Go ดูแลเอง ไม่ต้องพึ่ง OpenSSH ในเครื่องผู้ใช้
- **กินแรมน้อย** เหมาะกับแอปที่เปิดค้างถือ connection ทั้งวัน

**ไม่เหมาะ** ตรงส่วน GUI — Go อ่อนเรื่องนี้ที่สุด ที่เจอจริงระหว่างทำ: `widget.Table` ใส่ widget ที่กดได้ไม่ได้ (ต้องเปลี่ยนไปใช้ `List`), ต้องเขียน custom theme เองเพื่อให้ดูไม่ดิบ, จัดความกว้างคอลัมน์เองทีละช่อง, และ cgo ทำให้ cross-compile ต้องพึ่ง Docker

สรุป: **เครื่องยนต์เหมาะกับ Go มาก / หน้าตาเป็นส่วนที่ Go แย่ที่สุด** — แอปนี้เครื่องยนต์คือ network concurrency (ยากและสำคัญ) หน้าตาคือฟอร์มกับตาราง (ง่าย) การแลกเปลี่ยนจึงคุ้ม ถ้าเป็นแอปที่หน้าตาซับซ้อนกว่านี้มาก Go จะเป็นตัวเลือกที่แย่

### โครงสร้างไฟล์

```
main.go     — UI ทั้งหมด (layout, dialogs, การกระทำของปุ่ม)
model.go    — โครงสร้างข้อมูล Project/TunnelConfig + อ่าน/เขียน config.json + merge ตอน import
tunnel.go   — เชื่อมต่อ SSH จริง, forward port, จัดการ lifecycle ของแต่ละ tunnel, heartbeat วัด latency
menu.go     — เมนู File/Help: import/export setup, diagnostics, เช็คอัปเดต, Quit
keyring.go  — อ่าน/เขียน/ลบ password ใน keychain ของระบบ
release.go  — version ที่ฝังตอน build, เช็คอัปเดต, รวบรวม diagnostics
theme.go    — custom dark theme
tray.go     — system tray icon/menu (Show/Stop all/Quit)

tools/genicon/main.go — สร้าง icon.png ใหม่ (ดู "ไอคอนแอป" ด้านล่าง)
```

### ไอคอนแอป

`icon.png` ไม่ได้วาดด้วยโปรแกรมกราฟิกข้างนอก แต่ generate ด้วยโค้ด Go ล้วนใน [tools/genicon/main.go](tools/genicon/main.go) — สอดคล้องกับแนวทางของทั้งโปรเจกต์คือ "ไม่พึ่ง tool ภายนอก" (เหมือนที่ SSH เขียนเองแทนเรียก `ssh` ของระบบ)

รันใหม่เมื่อไหร่ก็ได้ด้วย:

```bash
go run ./tools/genicon
```

ดีไซน์: กระเบื้องมุมโค้งสีเข้ม ไล่เฉดตาม `theme.go`, มีเส้นโค้งสีฟ้า (tunnel) พาดจากจุด local (สีฟ้า accent) ไปจุด remote (สีเขียว — สีเดียวกับสถานะ RUNNING ในตาราง) พร้อมหัวลูกศรบอกทิศทาง — สื่อว่าแอปนี้ "เปิดทางเชื่อมจากเครื่องเราไปเซิร์ฟเวอร์ปลายทาง" ตรงตัว

ไฟล์นี้ถูก `//go:embed` เข้าไปในตัวแอปเองด้วย (ดู [tray.go](tray.go)) สำหรับไอคอน system tray ดังนั้นแก้ `icon.png` แล้วต้อง build ใหม่ทั้งไอคอนใน tray และ `.app`/`.exe` (`fyne package`/`fyne-cross` อ่านจากไฟล์เดียวกัน)

### รันตอนพัฒนา

```bash
go run .
go build -o stp-tunnel .
go vet ./...
```

### Build สำหรับแจกจ่าย

ใช้ `build.sh` — มันฝัง version ลง binary ให้อัตโนมัติ (ถ้า build มือแล้วลืม `-ldflags` แอปจะรายงานตัวเองว่า `dev`)

```bash
./build.sh              # build เฉพาะเครื่องนี้
./build.sh all          # macOS .app + Windows .exe (ต้องเปิด Docker)
VERSION=1.2.0 ./build.sh all   # กำหนดเลขเวอร์ชันเอง
```

เวอร์ชันปัจจุบันอยู่ในไฟล์ [VERSION](VERSION) — นี่คือ source of truth เพียงจุดเดียว
เวอร์ชันจะแสดงที่**มุมซ้ายล่างของหน้าต่างตลอดเวลา** เพื่อนดูได้ทันทีโดยไม่ต้องเปิดเมนู

`./build.sh all` (ไม่ระบุ `VERSION` เอง) จะเช็ค commit ตั้งแต่ tag `v<เวอร์ชันปัจจุบัน>` ก่อนเสมอ — ถ้าเจอ commit ที่ใช้ prefix แบบ [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, หรือ `BREAKING CHANGE`) จะถามว่าจะ bump เวอร์ชันให้ไหม (feat = minor, fix = patch, breaking = major) ก่อนเริ่ม build ตอบ `y` แล้วมันจะอัปเดต [VERSION](VERSION) ให้ — ที่เหลือ (CHANGELOG.md, บรรทัดเวอร์ชันด้านบนของไฟล์นี้, `git tag`) ยังต้องทำเองเหมือนเดิม

อย่าลืมอัปเดตบรรทัด **เวอร์ชัน/อัปเดตล่าสุด** บนสุดของไฟล์นี้ให้ตรงกันทุกครั้งที่แจกรอบใหม่ด้วย พร้อมเพิ่มแถวใหม่ในตาราง "สรุป release" และ entry เต็มใน [CHANGELOG.md](CHANGELOG.md) — และอัปเดต [latest.json](latest.json) ให้ตรงเวอร์ชันแล้ว push ขึ้น `main` ด้วย (ดูหัวข้อ "แจ้งเตือนเวอร์ชันใหม่" ด้านล่าง) ไม่งั้นเพื่อนกด Check for updates แล้วจะไม่เจออะไรใหม่

ถ้าแก้พฤติกรรมที่ผู้ใช้เจอ (ไม่ใช่แค่โค้ดภายใน) อย่าลืมแก้ [GUIDE-macOS.md](GUIDE-macOS.md) และ [GUIDE-Windows.md](GUIDE-Windows.md) ให้ตรงกับส่วน "สำหรับผู้ใช้งาน" ด้านบนด้วย — ไฟล์พวกนี้ไม่ได้ sync กันอัตโนมัติ

เครื่องมือที่ต้องมีครั้งแรก:

```bash
go install fyne.io/tools/cmd/fyne@latest
go install github.com/fyne-io/fyne-cross@latest
```

### แจ้งเตือนเวอร์ชันใหม่

`updateFeedURL` ใน [release.go](release.go) ชี้ไปที่ **public GitHub Gist** — ไม่ใช่ไฟล์ในนี้โดยตรง เพราะ repo `STPTunnelGo` เป็น private และ raw.githubusercontent.com เสิร์ฟไฟล์จาก private repo ให้แอปที่ไม่มี token ไม่ได้ (ฝัง token ลงแอปที่แจกออกไปก็ไม่ปลอดภัย ใครแกะแอปก็เอาไปใช้ต่อได้)

[latest.json](latest.json) ในนี้คือ**ไฟล์ต้นฉบับที่แก้** ทุกรอบที่แจกรุ่นใหม่:

```json
{"version": "1.0.1", "url": "https://.../releases", "notes": "แก้ปัญหา host key"}
```

แก้เสร็จแล้ว sync ขึ้น gist ด้วยคำสั่งนี้ (URL ที่แอปเรียกจะไม่เปลี่ยน เพราะ URL แบบไม่มี revision hash จะพาไปหา revision ล่าสุดเสมอ):

```bash
jq -n --rawfile content latest.json '{files: {"latest.json": {content: $content}}}' \
  | gh api gists/4a70a4fb82706c462054048e942096b1 --method PATCH --input -
```

แอปจะเช็คตอนเปิด (เงียบๆ ไม่กวนถ้าไม่มีอะไรใหม่) และเช็คได้เองผ่านเมนู **Help → Check for updates** หรือไอคอน ⋮ มุมขวาบน — เจอเวอร์ชันใหม่แล้วจะเด้ง dialog พร้อมปุ่มลิงก์ (จาก `url` ในไฟล์) ให้กดไปโหลดเอง **ไม่ใช่ auto-download/ติดตั้งอัตโนมัติ**

`url` ในไฟล์ตอนนี้ชี้ไปหน้า [GitHub Releases](https://github.com/Sitthiphong-krobkrong/STPTunnelGo/releases) ของ repo นี้ — ใช้ได้เพราะเป็นแค่ลิงก์ให้เปิดใน browser (เพื่อนที่ login GitHub แล้วเป็น collaborator ของ repo อยู่แล้วจะเข้าดูได้ปกติ) ถ้ายังไม่เคยสร้าง release จริงบน GitHub ให้เปลี่ยนไปชี้ที่อื่นที่แจกไฟล์จริงได้ก่อน (เช่น Drive) จนกว่าจะตั้ง GitHub Release เสร็จ

ถ้าลบ/ล้างค่า `updateFeedURL` ให้ว่าง ฟีเจอร์นี้จะไม่ทำงานเฉยๆ ไม่ error

> `GOOS=windows go build` เฉยๆ ใช้ไม่ได้ เพราะ Fyne ผูกกับ OpenGL ผ่าน CGO จึงต้อง cross-compiler ของแต่ละแพลตฟอร์ม ซึ่ง `fyne-cross` เตรียมไว้ให้ใน Docker image

> **ข้อควรระวัง:** Docker image ของ fyne-cross ยังใช้ Go 1.25 ดังนั้น `go` directive ใน `go.mod` ต้องไม่สูงกว่านั้น ถ้ารัน `go get` แล้ว dependency ดันเวอร์ชันขึ้นไป Windows build จะพังทันที

### หมายเหตุเชิงออกแบบ

- **แต่ละ tunnel = SSH connection ของตัวเอง** — tunnel หนึ่งหลุดหรือถูกหยุด ไม่กระทบตัวอื่น
- **ใช้ `widget.List` ไม่ใช่ `widget.Table`** — Table ของ Fyne ดักจับ tap ไปทำ cell selection ของตัวเอง ทำให้ widget ที่ interactive (checkbox, link, ปุ่ม) ข้างใน cell กดไม่ได้
- **การเชื่อมต่อทำใน goroutine เสมอ** — `ssh.Dial` ใช้เวลาได้ถึง 15 วินาที ถ้าทำบน UI thread หน้าต่างจะค้าง (อัปเดต UI กลับผ่าน `fyne.Do`)
- **password ไม่เคยอยู่ในไฟล์ของแอป** — ฝากไว้กับ keychain ของ OS ทั้งหมด การเข้ารหัสเองแล้วเก็บข้างๆ config เป็นแค่การอำพราง (กุญแจต้องอยู่ในเครื่องเหมือนกัน ใครเปิดไฟล์ได้ก็ถอดได้)
- **Host key** — ตรวจกับ `~/.ssh/known_hosts` ถ้ามี entry อยู่แล้ว, ยอมรับอัตโนมัติถ้าเป็น host ใหม่ (เทียบเท่า `StrictHostKeyChecking=accept-new`) และมี retry อัตโนมัติเมื่อ key เป็นคนละชนิดกับที่บันทึกไว้
- **Latency วัดด้วย SSH global request เปล่าๆ** (`keepalive@openssh.com`) ไม่ใช่การต่อไปที่ target จริง — เพื่อไม่ให้ heartbeat ไปรบกวน service ปลายทาง (DB, API) ทุก 5 วินาที
- **System tray มีเมนูแต่ไม่ intercept การปิดหน้าต่าง** ([tray.go](tray.go)) — ตั้งใจ **ไม่** ทำ "ปิด X = ซ่อนลง tray" เพราะ `fyne.io/systray` เวอร์ชันปัจจุบันมีบั๊กบน macOS: status item ลงทะเบียนกับ Control Center สำเร็จ แต่ตัวไอคอนไม่ถูก apply เลยมองไม่เห็น (upstream issue [fyne-io/fyne#4548](https://github.com/fyne-io/fyne/issues/4548)) ถ้า intercept การปิดไว้ทั้งที่มองไม่เห็นไอคอน ผู้ใช้จะกด X แล้วแอปหายไปเลย เรียกกลับไม่ได้นอกจาก Force Quit — รอแก้ที่ต้นทางก่อนค่อยเปิดพฤติกรรมนี้กลับมา
