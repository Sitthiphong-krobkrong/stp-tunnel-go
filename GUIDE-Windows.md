# STP Tunnel — คู่มือย่อ (Windows)

เปิด SSH tunnel ไปยัง server ภายในองค์กร โดยไม่ต้องจำคำสั่ง `ssh -L` และไม่ต้องเปิด terminal ทิ้งไว้

## ติดตั้ง

ดาวน์โหลดไฟล์เดียวแล้วเปิดได้เลย ไม่ต้องติดตั้งอะไรเพิ่ม

**ครั้งแรกที่เปิด** — ถ้าขึ้น **"Windows protected your PC"** ให้กด **More info → Run anyway** (เกิดขึ้นเพราะแอปยังไม่ได้ซื้อ code signing certificate) ทำแบบนี้ครั้งเดียวต่อเครื่อง หลังจากนั้นเปิดได้ปกติ

## เริ่มใช้งาน

1. **New** → สร้าง project (1 project = 1 server ที่จะ SSH เข้าไป: Name, Bastion host, Bastion user, Bastion port)
2. **Add tunnel** → Service name, Local port, Target host (ปกติ `localhost` เมื่อมองจากฝั่ง server), Target port
3. ติ๊ก tunnel ที่ต้องการ → กด **Start** → ใส่ password (ติ๊ก *Remember this password* ไว้ครั้งเดียวพอ)
4. เข้าใช้งานผ่าน `localhost:<Local port>` บนเครื่องตัวเอง

คอลัมน์ **Latency** ขึ้น `timeout` = tunnel กำลังจะหลุด (สถานะจะเปลี่ยนเป็น ERROR ตามมาไม่นาน)

## Password และ key

- ปกติใช้ password — เก็บใน **Windows Credential Manager** ไม่ติดไปกับไฟล์ config หรือไฟล์ export
- เปลี่ยน password บน server แล้ว? → Edit ที่ project → **Forget saved password**
- ถ้า server ปิด password auth → ใส่ path private key (เช่น `C:\Users\<you>\.ssh\id_rsa`) ในช่อง **Identity file** ของ project

## แชร์ setup ให้ทั้งทีม (ไม่ต้องกรอกใหม่ทีละคน)

คนที่ตั้งค่าไว้แล้ว → เมนู **File → Export setup...** → ติ๊กเลือก project ที่จะแชร์ (ค่าเริ่มต้นคือ project ที่เปิดอยู่) ได้ไฟล์ `stp-tunnel-<ชื่อ project>.json` → ส่งให้เพื่อน (ไฟล์นี้ไม่มี password อยู่ข้างใน ส่งทางแชทได้ปลอดภัย)
เพื่อน → เมนู **File → Import setup...** เลือกไฟล์นั้น (ถ้าในไฟล์มีหลาย project จะให้ติ๊กเลือกว่าจะเอาอันไหน) → เหลือแค่กรอก password ของตัวเอง

## Start ไม่ผ่าน หรือ tunnel ค้างที่ ERROR — ต้องทำยังไง

ดูข้อความในช่อง **LOG** ด้านล่างของหน้าแอป (แถบสีเทาใต้ตาราง tunnel) แล้วเทียบกับตารางนี้ก่อน:

| ข้อความใน Log | สาเหตุ / วิธีแก้ |
|---|---|
| `Permission denied` | user หรือ password ผิด — Edit ที่ project เช็คอีกที |
| `host key ... does not match` | server ถูก rebuild หรือ IP ถูกเวียนใช้ใหม่ → รัน `ssh-keygen -R <host>` หรือติ๊ก **Skip host key check** ในฟอร์ม Edit project |
| `address already in use` | Local port นั้นมีโปรแกรมอื่นใช้อยู่ → เปลี่ยนเป็น port อื่น |
| `cannot reach <target>` | ต่อ server ได้ แต่ server เข้าถึงปลายทางไม่ได้ → เช็ค Target host / Target port |

**ถ้าข้อความไม่ตรงกับตารางนี้ หรือลองแล้วยังไม่หาย** — เปิดเมนู **Help → Copy diagnostics** ในแอป จะก็อปข้อมูลที่จำเป็นไว้ใน clipboard ให้เอง (เวอร์ชัน, OS, การตั้งค่า project, สถานะ tunnel, log 20 บรรทัดล่าสุด — **ไม่มี password**) แล้วแปะส่งมาได้ที่ **stp — sitthiphong.krobkrong@gmail.com**
