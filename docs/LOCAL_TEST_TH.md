# Warwick Private Consult — November 2026

Frontend: Vite + React + OpenAI Apps SDK UI · Backend: Go · Database: PostgreSQL

## จัดการรายการจบแล้วและล้างข้อมูลเทสจาก Staff

อัปเดตรุ่นนี้โดยเก็บข้อมูลเดิม:

```bash
bash ~/Downloads/Warwick_Staff_Manage_Test.sh
```

ดาวน์โหลดไฟล์ไว้ Downloads เปิด Docker Desktop และ Terminal ในโฟลเดอร์โปรเจกต์เดิม เมื่อเปิดเว็บแล้วรีเฟรช Staff จะมีแท็บรายการจบแล้ว ปุ่มเก็บรายการ/นำกลับมาแสดง และปุ่มลบข้อมูลทดสอบ/ล้างงานทดสอบนี้ (ต้องยืนยันก่อนลบ)

รอบจริงใช้เก็บประวัติและเปิดดูย้อนหลัง Completed ยังคงจบแล้ว ไม่เปลี่ยนเป็นการจองใหม่ โหมดเทสปิดส่งเมลลบรายการที่จบแล้วได้ หรือพิมพ์ CLEAR TEST DATA เพื่อล้างงานนี้เริ่มคิวใหม่ โดยเก็บบัญชี Staff และตาราง slots ไว้ รายละเอียดอยู่ STAFF_DATA_MANAGEMENT_TH.md การอัปเดตเองไม่ล้างข้อมูล

## รุ่นทดสอบล่าสุด — ตัวนับคิว / พักส่งอีเมล

ดาวน์โหลด `Warwick_Queue_Test.sh` ไป Downloads เปิด Docker Desktop และ Terminal ในโฟลเดอร์ `warwick-consult-postgres` เดิม แล้วรัน:

```bash
bash ~/Downloads/Warwick_Queue_Test.sh
```

เมื่อขึ้น Queue test is ready เปิด http://127.0.0.1:8080 แล้วรีเฟรช ระบบเก็บการจอง คิว ลิงก์ติดตาม config บัญชี Staff และ SMTP settings เดิม ใช้ฐานข้อมูลทดสอบเดิม ไม่ reset และไม่ถามรหัสเมล

รุ่นนี้ใช้ `compose.queue-test.yml` ปิด SMTP_HOST/SMTP_FROM สำหรับ app เสมอ แม้เคยตั้งค่าผู้ส่งไว้ จึงไม่มี worker ส่งเมลขณะเทส ตัวนับคิวและปุ่มคิวถัดไปทำงานครบตามรุ่นล่าสุด

เปิดครั้งถัดไป: `bash scripts/queue-test.sh`; หยุดเก็บข้อมูล: `bash scripts/queue-test.sh stop` จาก ZIP ใหม่แตกโฟลเดอร์แล้วใช้ `bash scripts/queue-test.sh` ได้เช่นกัน

วิธีเทส (เลือกรอบที่มีที่ว่างและไม่มีคิวเดิมปน): จอง5รายการ Beginner Math 10:30–12:00 เก็บลิงก์ของรายการที่5; Login Staff RD1/RD1 เช็คอินทั้ง5 กดเปิดคิวรอบนี้และบันทึก/จัดคิว เรียกคนแรก เปิดลิงก์คนที่5ควรขึ้นเหลืออีก4คิว ลองข้าม/ยกเลิก/จัดลำดับใหม่แล้วจำนวนเปลี่ยนตาม หลังเรียกคนที่5ขึ้นถึงคิวแล้ว หากมีคิวเดิมในห้อง จำนวนจะรวมรายการเดิมที่พร้อมเรียกด้วย ยังไม่เช็คอินหรือไม่จัดลำดับจะแสดงสถานะรอข้อมูลแทนตัวเลข

บัญชี Staff RD1, RD2, RD3, SM1, SM2, SM3, SM4; Password ตรงกับ Username ทุกตัว

เรื่องอีเมลพักไว้จนถึงขั้น Deploy ขั้นตอน SMTP ด้านล่างเป็นทางเลือกจากรุ่นก่อนหน้า ไม่ต้องทำในรอบนี้

## อัปเดตตัวนับคิว + ตั้งค่าผู้ส่ง Warwick

```bash
bash ~/Downloads/Warwick_Update_Queue_Count.sh
```

ดาวน์โหลดไฟล์ไป Downloads และเปิด Terminal ในโฟลเดอร์โปรเจกต์เดิม คำสั่งเก็บการจอง/ตาราง/ลิงก์/บัญชีเดิม ตัวนับส่วนตัวแสดงจากลำดับปัจจุบันถึงตัวเอง เช่น กำลังเรียกคิว1 คุณคิว5 = เหลืออีก4คิว ปรับเมื่อเรียก/ข้าม/ยกเลิก/จัดใหม่ นับตามวิชาปัจจุบันของ Combined

ตั้งค่าบัญชีผู้ส่ง Warwick ง่ายขึ้นโดยเตรียมค่าให้ทั้งหมด เหลือ Google App Password ช่องเดียว เมื่อสร้าง App Password แล้วใช้ `bash ~/Downloads/Warwick_Update_Queue_Count.sh --setup-warwick-email` หรือ `bash scripts/setup-warwick-email.sh` ตัวช่วยส่งเมลทดสอบเข้าบัญชีผู้ส่งด้วย รหัสเข้าเมลปกติไม่ได้ใส่ในไฟล์แจกจ่าย ดูขั้นตอนใน EMAIL_SETUP_TH.md และรายละเอียดการนับใน QUEUE_COUNT_TH.md

## อัปเดตอีเมลและคิวถัดไป — เก็บการจองเดิม

ดาวน์โหลด `Warwick_Update_Email_Queue.sh` ไป Downloads เปิด Docker Desktop และ Terminal ในโฟลเดอร์โปรเจกต์เดิม แล้วรัน:

```bash
bash ~/Downloads/Warwick_Update_Email_Queue.sh
```

ตั้งค่าบัญชีผู้ส่งจริงด้วย `bash scripts/setup-email.sh` (กรอกรหัสบนเครื่อง ไม่แสดงรหัส) ทดสอบส่งได้ด้วย `bash scripts/test-email.sh` แล้วลองจองใหม่เพื่อรับปุ่มติดตามคิวส่วนตัว รายการที่เคยจองตอนปิด SMTP ไม่ส่งย้อนหลังอัตโนมัติ

อีเมล HTML ใช้กรอบสัมมนา/Consult และปุ่มสีแดงตามรูปตัวอย่าง ข้อความเป็น “ช่วงเวลา Consult: xx:xx - xx:xx” จอรวมเพิ่มคิวถัดไป 2 คิวต่อห้อง Math/Verbal แสดงเฉพาะรหัสคิว

ลิงก์ทดสอบ `127.0.0.1` เปิดได้เฉพาะเครื่องที่รันเว็บ ก่อนให้ผู้ปกครองใช้งานจากอุปกรณ์อื่น ต้อง Deploy พร้อม HTTPS และตั้ง PUBLIC_URL จริง ดูขั้นตอน SMTP และข้อจำกัดใน `docs/EMAIL_SETUP_TH.md`

## อัปเดตกล่องวันและบัญชี Staff — เก็บข้อมูลจองเดิม

ดาวน์โหลด `Warwick_Update_Staff_Users.sh` ไป Downloads เปิด Docker Desktop แล้วเปิด Terminal ในโฟลเดอร์ `warwick-consult-postgres` เดิม:

```bash
bash ~/Downloads/Warwick_Update_Staff_Users.sh
```

คำสั่งนี้สำรอง source อัปเดตโค้ด และเปิดระบบใหม่ โดยเก็บวัน/รอบจอง, รายการจอง, คิว, ลิงก์ติดตาม, ประวัติ, `.env.local-test` และฐานข้อมูลเดิม ไม่เรียกคำสั่ง reset

เมื่อขึ้น `Local test is ready` รีเฟรชหน้า http://127.0.0.1:8080 แล้ว Login ใหม่ด้วยหนึ่งใน 7 บัญชีด้านล่าง บัญชี `staff` เดิมถูกแทนที่ ระบบเหลือเพียง 7 บัญชีตามคำขอ

High Level Verbal เลือกวันด้วยกล่อง 6 หรือ 7 November เหมือนกล่องเวลา พร้อมแสดง Zoom / On-site วันที่เลือกมีเครื่องหมายถูก เปลี่ยนวันแล้วต้องเลือกเวลาใหม่ กดวันเดิมยังเก็บเวลาที่เลือกไว้

จาก source ZIP ใหม่ อัปเดตโฟลเดอร์เดิมได้ด้วย `bash scripts/update-staff-users.sh`; เปิดครั้งถัดไปตามปกติใช้ `bash scripts/local-test.sh` โดยข้อมูลยังอยู่

## ล้างข้อมูลทดสอบเมื่อต้องการเริ่มใหม่

คำสั่งนี้แยกจากการอัปเดตบัญชี:

```bash
bash scripts/update-november.sh --reset-test-data
```

จะ **ล้างรายการจอง คิว ลำดับ ประวัติ รายการยกเลิก งานอีเมล และ session ของฐานข้อมูลทดสอบทั้งหมด** รวมข้อมูลงานเดิม เริ่มเลขคิวใหม่ เก็บ 7 บัญชี Staff, APP_SECRET และตัวฐานข้อมูล ไม่ลบ Docker volume ลิงก์ติดตามเก่าใช้ไม่ได้

`Warwick_Update_November.sh` ที่ดาวน์โหลดแยกมีค่าเริ่มต้นเป็น reset เช่นเดียวกัน ส่วน `--schedule-only` ใช้อัปเดตตาราง November โดยเก็บการจองและไม่ย้ายรายการเดิมอัตโนมัติ ใช้ไฟล์อัปเดตรุ่นล่าสุดเสมอ

## ตารางสัมมนารวม

| วิชา | วันที่ | เวลา |
| --- | --- | --- |
| Verbal | 7 November 2026 | 08:30–09:20 |
| Math | 7 November 2026 | 09:30–10:20 |

หน้าจอง Beginner แสดงวันและเวลาสัมมนาตามวิชาที่เลือก Math + Verbal แสดงทั้งสองช่วง พร้อมช่องยืนยัน

## ตาราง Private Consult

มีทั้งหมด **20 รอบ / 105 slots** โควตานับแยกตามระดับ + วิชาที่จอง + วัน + รอบ จำนวนที่ไม่ระบุในคำขอใช้ 4 slots

| ระดับ | วิชา | วันที่ | รอบและจำนวน slots |
| --- | --- | --- | --- |
| Beginner | Math | 7 November 2026 | 10:30–12:00 (7) · 16:25–17:00 (3) · 17:00–18:00 (4) · 18:00–19:00 (4) · 19:00–20:00 (4) |
| Beginner | Verbal | 7 November 2026 | 09:30–10:00 (2) · 10:00–11:00 (4) · 11:00–12:15 (5) · 16:35–17:00 (2) · 17:00–18:00 (4) |
| Beginner | Math + Verbal | 7 November 2026 | 13:00–14:30 (7) · 14:45–16:15 (7) |
| High Level | Math | 6 November 2026 | 10:00–11:30 (7) · 11:55–13:25 (7) |
| High Level | Verbal | 6 November 2026 | 10:00–11:30 (7) · 11:55–13:40 (8) |
| High Level | Math + Verbal | 6 November 2026 | 14:30–16:10 (7) · 16:10–18:10 (8) |
| High Level | Verbal | 7 November 2026 | 18:00–19:00 (4) · 19:00–20:00 (4) |

Beginner ทุกวิชาจองได้เฉพาะ 7 พ.ย. High Level Math/Combined เฉพาะ 6 พ.ย. และ High Level Verbal เลือกได้ 6 หรือ 7 พ.ย. รูปแบบเดิมคงไว้: 6 พ.ย. Online via Zoom / 7 พ.ย. On-site

## ใช้งานครั้งแรกจาก ZIP

ติดตั้งและเปิด Docker Desktop แตก ZIP แล้วเปิด Terminal ที่โฟลเดอร์ `warwick-consult-postgres`:

```bash
bash scripts/local-test.sh
```

ระบบใช้ PostgreSQL 17 ใน Compose project `warwick-consult-local-test` และฐานข้อมูล `warwick_consult_local_test` เว็บเปิดได้จากเครื่องนี้ที่ 127.0.0.1:8080

บัญชี Staff สำหรับทดสอบในเครื่อง (พิมพ์ตัวใหญ่):

| Username | Password |
| --- | --- |
| RD1 | RD1 |
| RD2 | RD2 |
| RD3 | RD3 |
| SM1 | SM1 |
| SM2 | SM2 |
| SM3 | SM3 |
| SM4 | SM4 |

ทั้ง 7 บัญชีใช้เช็กอิน จัดคิว แก้การจอง ยกเลิกและคืนรายการได้เหมือนกัน ประวัติระบุบัญชีผู้ทำรายการ รหัสใน `.env.local-test` เดิมไม่ได้ใช้ Login ในโหมดนี้ การเปิดระบบซ้ำไม่ล้างข้อมูลและไม่เปลี่ยน 7 บัญชีนี้

| หน้า | URL |
| --- | --- |
| จอง | http://127.0.0.1:8080/#/book |
| Staff | http://127.0.0.1:8080/#/staff |
| จอคิว | http://127.0.0.1:8080/#/display |

หยุดโดยเก็บข้อมูลไว้: `bash scripts/local-test.sh stop` Windows ใช้ `.\scripts\local-test.ps1` สำหรับเปิดตามปกติ (ตัวอัปเดตตาราง/reset ที่แนบเป็น Bash)

## วิธีทดสอบ

1. จอง Beginner Math 10:30–12:00 หรือ Combined 13:00–14:30 แล้วเก็บลิงก์ติดตามคิว
2. Login Staff > เช็กอิน · รวมทุกช่วงเวลา > ค้นหาชื่อ/Email/Wcode > เช็กอิน
3. กดเปิดคิวรอบนี้ > บันทึก / จัดคิว > เรียกคิว > เริ่มปรึกษา > จบ
4. Math + Verbal เลือกวิชาเริ่มต้นก่อนเรียกครั้งแรกได้ จบวิชาแรกแล้วจัดลำดับอีกห้อง รหัสคิวเดิมคงอยู่
5. ลองกรอง All / Math / Verbal / Math + Verbal ซึ่งแยกตามวิชาที่จอง
6. ลองแก้วัน / เวลา / วิชาก่อนเริ่มปรึกษา ลิงก์เดิมยังใช้ได้; เปลี่ยนวิชาแล้วรหัสคิวใหม่ออกตอนจัดคิว เปลี่ยนวันต้องเช็กอินใหม่
7. เอาออก / ยกเลิกเพื่อคืน slot แล้วเปิดแท็บรายการยกเลิกเพื่อคืนการจอง ต้องมีที่ว่างและจัดคิวใหม่
8. Combined ที่จบวิชาแรกแล้วสามารถยกเลิกระหว่างรอและคืนวิชาที่เหลือ โดยเก็บ stage เดิม; ไม่แก้วิชา/รอบหลังเริ่มปรึกษาแล้ว

## การเก็บข้อมูลและพัฒนา

ตารางระบบจริงอยู่ `backend/config/event.json`; ตารางทดสอบ `backend/config/event.local-test.json` มี sample=true กับ ID ขึ้นต้น local-test-. เวลา seminar อยู่ใน config/API ไม่ฝังแยกใน UI อีกแล้ว

ตัวเปิดปกติเก็บ `.env.local-test` และ named volume เดิม การล้างข้อมูลเป็นคำสั่งแยกที่อนุญาตเฉพาะ local-test, sample event, DB/user สำหรับทดสอบ และ host ในเครื่อง ไม่มี reset API บนเว็บ

โหมด local-test เริ่มต้นปิด SMTP แต่เปิดส่งจริงได้ผ่าน `.env.smtp` / `scripts/setup-email.sh` หากเปิดใช้จริงต้องตั้ง HTTPS, sample=false, SMTP และยืนยันทรัพยากรอาจารย์ร่วมกัน ดู `docs/POSTGRESQL.md` และ `docs/VALIDATION.md` สำหรับฐานข้อมูลและผลตรวจ

พัฒนา Frontend: `npm ci` / `npm run dev` ใน `frontend`; Vite proxy API ไป port 8080. Production build: `npm run build`. Dockerfile build Frontend/Go ให้เอง โดยไม่ต้องมี OpenAI API key สำหรับ UI components

อ่านรายการเหตุการณ์และข้อที่ยังต้องตัดสินใจใน `docs/STAFF_REVIEW_TH.md`. ไม่มีการเพิ่มวิธีแก้ประเด็นอื่นนอกคำขอ

