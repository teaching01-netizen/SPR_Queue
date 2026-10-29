# PostgreSQL configuration and storage

## Local test profile

ตัวเปิด `scripts/local-test.sh` / `.ps1` ใช้ `compose.local-test.yml` และ project name เฉพาะ `warwick-consult-local-test` Database `warwick_consult_local_test` เก็บใน volume `local_test_postgres` ของ project นี้ ไม่มี published database port และไม่ใช้ production DATABASE_URL จาก shell

Initializer สร้าง `.env.local-test` กับ `backend/config/event.local-test.json` โดยปกป้องไฟล์เดิม App environment `local-test` ต้องใช้ sample event ID ที่ขึ้นต้น `local-test-`; SMTP ถูกปิดใน Compose ดู `LOCAL_TEST_TH.md`

## Connection

Backend ใช้ `database/sql` ผ่าน pgx v5 โดย `DATABASE_URL` เป็นค่าที่จำเป็น ไม่มี fallback กลับไปใช้ SQLite

Local Compose เริ่มด้วย `go run -buildvcs=false ./cmd/server -init` ใน backend แล้ว `docker compose --env-file backend/.env up -d --wait db` ที่ root รหัสฐานข้อมูลสร้างแบบสุ่ม ไม่ใช่ค่าร่วมใน source ส่วน server ที่มีอยู่แล้วให้สร้าง database/role แล้วปรับ URL แทน

Connection pool ต่อ Go instance: สูงสุด 12 connections, idle 6, lifetime 30 นาที; connect timeout 5 วินาที, statement timeout 15 วินาที, lock timeout 10 วินาที พิจารณาจำนวน connections รวมเมื่อเปิดหลาย instances

`sslmode=disable` ใน setup local ใช้กับ database ในเครื่อง สำหรับ database ภายนอกให้เลือก TLS/certificate ตาม server ผู้ให้บริการ

## Schema and migrations

SQL อยู่ใน `backend/internal/consult/migrations/001_initial.sql` และฝังใน Go binary Migration ทำใน transaction ภายใต้ PostgreSQL advisory lock; `schema_migrations` เก็บชื่อ/checksum/applied_at ไม่รัน SQL เดิมซ้ำ และปฏิเสธ checksum ที่เปลี่ยน การอัปเดตครั้งต่อไปให้เพิ่มไฟล์เลขลำดับใหม่แทนการแก้ไฟล์ที่ใช้ไปแล้ว

| Table | ข้อมูล |
| --- | --- |
| periods | รอบ, capacity, mode; composite key ตาม scope |
| bookings | ข้อมูลจอง/คิว; searchable scope columns + JSONB detail |
| meta / sequences | revision, running queue sequence |
| users / sessions | password hash และ session token hash |
| audit | ประวัติคำสั่งพร้อม actor |
| outbox | งานอีเมลและ delivery lease/retry |
| settings | fingerprint ของ APP_SECRET |

Foreign key ตรวจว่ารอบมีอยู่, JSONB check ตรวจให้ detail ตรงกับ scope/status columns, unique indexes ตรวจ request key/queue code/token และจำกัดหนึ่ง Calling/Serving ต่อ event + level + date + current subject

Booking/queue/email writes ใช้ READ COMMITTED และล็อก meta guard row ก่อนอ่าน/เขียน; trigger ที่ database ตรวจ capacity อีกชั้น ใช้งานหลาย Go instances ได้โดยใช้ config และ APP_SECRET เดียวกัน Writers จะรอเรียงกันทั้งระบบเพื่อรักษากฎ revision/queue รุ่นนี้ยังไม่ได้ปรับ throughput ให้รองรับภาระงานขนาดใหญ่ Read-only snapshots ใช้ REPEATABLE READ เพื่อให้อ่าน revision และรายการจาก snapshot เดียวกัน

## Persistence and changes

Compose เก็บข้อมูลใน named volume `postgres_data`; ปิด/เปิด container ไม่ลบข้อมูล หลีกเลี่ยง `docker compose down -v` หากต้องเก็บข้อมูล รหัสใหม่ใน `.env` จะไม่เปลี่ยนรหัสของ PostgreSQL volume เดิมโดยอัตโนมัติ

เก็บ APP_SECRET เดิมของฐานข้อมูลไว้ในค่าลับของบริการ Tracking tokens ถูกเข้ารหัสด้วย secret นี้; เปลี่ยน key จะทำให้ server ปฏิเสธ database นั้น ระบบสำรอง/กู้คืนฐานข้อมูลถูกถอดออกตามที่เลือกไว้

รุ่นนี้เริ่ม PostgreSQL ว่าง ไม่มีการนำ booking จาก SQLite มาให้อัตโนมัติ หากมีฐานข้อมูลเดิมจริง ให้เก็บไว้และจัดงานนำเข้าที่รักษา IDs/tokens/sequences แยกต่างหาก

## Tests

ตั้ง `TEST_DATABASE_URL` ของ PostgreSQL สำหรับทดสอบโดยเฉพาะ แล้ว `go test -race -count=1 ./...` ใน backend Fixtures สร้าง schema ชื่อสุ่ม แยกแต่ละ test ผ่าน connection `search_path` และลบหลังจบ ต้องมีสิทธิ์ CREATE/DROP SCHEMA

หาก URL ไม่ตั้ง integration tests จะ skip; หาก URL ตั้งแต่ server ใช้ไม่ได้ test จะ fail ไม่ซ่อน connection errors CI ที่แนบตั้ง PostgreSQL 17 จริง แต่ยังไม่ได้รัน workflow ในงานนี้ ดูผลตรวจและข้อจำกัดใน `VALIDATION.md`

## Railway production — 29 October 2026

ใช้ Consult Web + Postgres พร้อม persistent volume ดู RAILWAY_DEPLOY_TH.md และ GITHUB_SETUP_TH.md ไม่มี backup service, bucket, cron หรือ restore workflow
