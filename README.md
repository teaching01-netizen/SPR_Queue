# Warwick Private Consult

ระบบจอง Private Consult และจัดคิวสำหรับ Warwick Institute — November 2026

Frontend: Vite + React + OpenAI Apps SDK UI · Backend: Go · Database: PostgreSQL

- จองตามระดับ วิชา วัน และรอบ พร้อมตรวจความจุที่ฐานข้อมูล
- Staff7บัญชี เช็กอิน จัด/สลับคิว แก้วันเวลาและวิชา ยกเลิก/คืนการจอง และเก็บรายการจบแล้ว
- Admin คืนผู้ปกครองกลับเป็นรอเช็กอินโดยใช้รหัสเดิม หรือยกเลิกรายการทดสอบเพื่อคืนโควตาแม้จบเคสแล้ว เฉพาะรายพร้อม audit ดู [คู่มือ](docs/BOOKING_RECOVERY_TH.md)
- Math+Verbal เลือกวิชาที่เริ่มก่อนและส่งต่ออีกวิชาได้
- ลิงก์ส่วนตัวติดตามสถานะ/จำนวนคิวที่เหลือ และจอสาธารณะแสดงคิวปัจจุบันกับคิวถัดไป
- อัปเดตข้อมูลทุก 5 วินาทีเมื่อเปิดหน้าเว็บ และหยุดเมื่อซ่อนแท็บหรือ offline
- อีเมลผ่าน Google Apps Script MailApp พร้อมแม่แบบเดิม เปิดส่งจริงเมื่อเชื่อม Web app URL และกุญแจครบ

## เริ่มต้น

1. [นำโค้ดเข้าGitHub](docs/GITHUB_SETUP_TH.md)
2. [Deployขึ้นRailway](docs/RAILWAY_DEPLOY_TH.md)
3. [ทดลองในเครื่อง](docs/QUICK_START_TH.md)

Railwayใช้เพียง Consult Web และ Postgres พร้อมpersistent volume ไม่มีระบบสำรอง/กู้คืนฐานข้อมูลหรือบริการBackupในรุ่นนี้ตามที่เลือกไว้ ฟังก์ชันคืนการจองที่ยกเลิกในหน้าStaffยังอยู่

สร้างค่าลับสำหรับฐานรอบจริงใหม่:

```bash
bash scripts/prepare-railway.sh
```

ไฟล์อยู่ .private/railway/ ห้าม commit หรืออัปโหลดไป GitHub ตัวสร้างสำหรับฐานใหม่ใช้รหัสสุ่มแยกกัน ส่วนบัญชีที่ใช้งานแล้วอ่านค่าจาก Railway และรองรับการกำหนด Password ให้ตรงกับ Username ของบัญชี Staff ทั้ง 7 ผ่านคำสั่ง provision โดยเฉพาะ ไม่เปลี่ยนรหัสอัตโนมัติเมื่อ Deploy

## ตรวจโค้ด

```bash
node --test scripts/*.test.cjs
```

รัน npm ci และ npm run build ใน frontend/; รัน go test -race -count=1 ./... และ go vet ./... ใน backend/ โดยใช้ TEST_DATABASE_URLของฐานทดสอบแยกสำหรับintegration tests ดูขอบเขตผลตรวจใน [VALIDATION.md](docs/VALIDATION.md) CIเตรียมPostgreSQL17สำหรับทดสอบและDockerbuildใน .github/workflows/ci.yml

Dockerfileที่root buildทั้งFrontendและGo แล้วเสิร์ฟหน้าเว็บกับAPIจากบริการเดียวกัน ค่าproductionและลิงก์HTTPSตั้งในRailway ไม่ใส่รหัสจริงในsource

## ตารางจองฉบับปรับปรุง 30 ตุลาคม 2026

ดู [ตารางใหม่และผลตรวจอีเมล](docs/SCHEDULE_UPDATE_TH.md) High Level วันที่ 6 พฤศจิกายน ใช้ช่วงเวลาเดิม ส่วน Beginner วันที่ 7 พฤศจิกายน ใช้เวลาใหม่ตามที่แจ้ง รวม 82 slots แยกโควตาแต่ละระดับ ไม่เปิด Verbal High Level วันที่ 7 พฤศจิกายน

## หน้าจองและอีเมลยืนยัน

ดู [รายการปรับ UI](docs/UI_UPDATE_TH.md) และ [เปิดส่งอีเมลจริง](docs/EMAIL_SETUP_TH.md) แม่แบบอีเมลเดิมมีช่วงเวลา Consult และปุ่มติดตามคิว ใช้ Google Apps Script เป็นตัวส่ง โดยเชื่อม Go ผ่านคำขอ HTTPS ที่มีลายเซ็น

## Staff Excel export

The Staff dashboard has an Export Excel button next to Refresh. It downloads an authenticated, event-scoped `.xlsx` snapshot with visible registrations (excluding cancelled and archived records) and a period summary, independently of UI filters. See [docs/STAFF_EXPORT_TH.md](docs/STAFF_EXPORT_TH.md). No new dependencies, database migrations or Railway variables are required.

Staff can also correct Wcode, nickname, school and parent email using Edit student details without changing booking identity, queues or consultation history.

## Google Sheets automatic sync

Optional production background sync mirrors the same 39 registration columns to the supplied Google Sheets tab via a separate signed Apps Script receiver. New registrations append at the bottom, corrections update by booking ID, and cancelled/archived rows are cleared. Configure SHEETS_SYNC_URL and SHEETS_SYNC_SECRET after deploying the Sheets script; see [docs/GOOGLE_SHEETS_SYNC_TH.md](docs/GOOGLE_SHEETS_SYNC_TH.md). PostgreSQL remains the source and registration continues if Google is unavailable.
