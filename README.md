# Warwick Private Consult

ระบบจอง Private Consult และจัดคิวสำหรับ Warwick Institute — November 2026

Frontend: Vite + React + OpenAI Apps SDK UI · Backend: Go · Database: PostgreSQL

- จองตามระดับ วิชา วัน และรอบ พร้อมตรวจความจุที่ฐานข้อมูล
- Staff7บัญชี เช็กอิน จัด/สลับคิว แก้วันเวลาและวิชา ยกเลิก/คืนการจอง และเก็บรายการจบแล้ว
- Math+Verbal เลือกวิชาที่เริ่มก่อนและส่งต่ออีกวิชาได้
- ลิงก์ส่วนตัวติดตามสถานะ/จำนวนคิวที่เหลือ และจอสาธารณะแสดงคิวปัจจุบันกับคิวถัดไป
- อัปเดตหน้าจอตามการใช้งานและหยุดเมื่อซ่อนแท็บหรือoffline
- อีเมลผ่านResendHTTPSรองรับไว้ แต่ปิดจนกว่าจะตั้งค่าจริง

## เริ่มต้น

1. [นำโค้ดเข้าGitHub](docs/GITHUB_SETUP_TH.md)
2. [Deployขึ้นRailway](docs/RAILWAY_DEPLOY_TH.md)
3. [ทดลองในเครื่อง](docs/QUICK_START_TH.md)

Railwayใช้เพียง Consult Web และ Postgres พร้อมpersistent volume ไม่มีระบบสำรอง/กู้คืนฐานข้อมูลหรือบริการBackupในรุ่นนี้ตามที่เลือกไว้ ฟังก์ชันคืนการจองที่ยกเลิกในหน้าStaffยังอยู่

สร้างค่าลับสำหรับฐานรอบจริงใหม่:

```bash
bash scripts/prepare-railway.sh
```

ไฟล์อยู่ .private/railway/ ห้ามcommitหรืออัปโหลดไปGitHub บัญชีรอบจริงใช้รหัสสุ่มแยกกัน บัญชีusername=passwordใช้เฉพาะโหมดเทส

## ตรวจโค้ด

```bash
node --test scripts/*.test.cjs
```

รัน npm ci และ npm run build ใน frontend/; รัน go test -race -count=1 ./... และ go vet ./... ใน backend/ โดยใช้ TEST_DATABASE_URLของฐานทดสอบแยกสำหรับintegration tests ดูขอบเขตผลตรวจใน [VALIDATION.md](docs/VALIDATION.md) CIเตรียมPostgreSQL17สำหรับทดสอบและDockerbuildใน .github/workflows/ci.yml

Dockerfileที่root buildทั้งFrontendและGo แล้วเสิร์ฟหน้าเว็บกับAPIจากบริการเดียวกัน ค่าproductionและลิงก์HTTPSตั้งในRailway ไม่ใส่รหัสจริงในsource
