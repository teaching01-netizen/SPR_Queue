# นำ Warwick Consult ขึ้น Railway

อัปเดต 30 ตุลาคม 2026 · Vite/React + Go + PostgreSQL

ระบบสำรองและกู้คืนฐานข้อมูลถูกถอดออกตามที่เลือกไว้ ใช้เพียง Consult Web และ Postgres พร้อม persistent volume ฟังก์ชันคืนการจองที่ยกเลิกและเก็บประวัติในหน้า Staff ยังใช้งานตามเดิม

## 1. นำโค้ดเข้า GitHub ก่อน

ทำตาม [GITHUB_SETUP_TH.md](GITHUB_SETUP_TH.md) ใช้ repository ส่วนตัวสำหรับแอปนี้โดยเฉพาะ ใน root ต้องมี Dockerfile, backend/, frontend/, scripts/ และ .github/ ไม่อัปโหลด ZIP เป็นไฟล์เดี่ยวแทน source code

## 2. สถานะ Railway ที่ใช้งานอยู่

Project: [warwick-consult-production](https://railway.com/project/60d16b28-22cf-4a5e-9bee-14f7eb3998ab?environmentId=ec81e3a0-a2fa-4529-b4d6-2e89e7ec8c30)

- Environment: production
- Consult Web: 1f385e9c-478b-4dec-99e2-69cf08af194c
- Postgres: 522dda9f-650c-4e7a-933f-52cc3d8570e0
- postgres-volume: 6f2a0411-65b7-4d30-9334-b1bc5f12222b
- Region: Singapore, หนึ่ง replicaต่อบริการ

บริการ Deploy สำเร็จแล้ว เชื่อม teaching01-netizen/SPR_Queue สาขา main และเปิดเว็บ https://consult-web-production.up.railway.app ไม่ต้องสร้าง project หรือฐานข้อมูลซ้ำ

โค้ดตารางจองฉบับใหม่ในชุดนี้ยังต้องอัปโหลดเข้า GitHub แล้วรอ Railway Deploy สำเร็จก่อนจึงจะเปลี่ยนบนเว็บจริง ดู SCHEDULE_UPDATE_TH.md

ยกเลิก staged creation ของ Consult Backup แล้ว Bucket consult-backups ถูกลบแล้วและไม่มี provisioned instances ก่อนหน้านี้มีรายการสร้าง bucket ค้างใน pending patch ตรวจ30ตุลาคม2026 ผู้ใช้Discardรายการค้างแล้ว เหลือ Consult Web, Postgres และ postgres-volume เท่านั้น ยังไม่ได้เปิด scheduled snapshot ของ volumeจากงานนี้

## 3. สร้างค่าเว็บและบัญชี Staff

ในโฟลเดอร์ที่แตกไฟล์แล้ว รัน:

```bash
bash scripts/prepare-railway.sh
```

สร้างใน .private/railway/ เฉพาะ railway-web.env, staff-credentials.csv และ README_PRIVATE.txt ไม่สร้างตัวแปร Backup, AWS, bucket หรือ recovery keys

ตัวสร้างสำหรับฐานใหม่ให้บัญชี RD1, RD2, RD3, SM1, SM2, SM3, SM4 มีรหัสสุ่มแยกกัน RD1 เป็น admin บัญชีอื่นเป็น operator ส่วนบัญชีบนเว็บที่มีอยู่ใช้ค่าที่ตั้งใน Railway แล้ว ไม่ต้องรันตัวสร้างหรือเปลี่ยน APP_SECRET ใหม่ กุญแจ APP_SECRET ใช้เข้ารหัสลิงก์ติดตามคิว ต้องเก็บค่าเดิมไว้กับฐานเดิม ตัวสร้างไม่ทับโฟลเดอร์ที่มีอยู่แล้ว ห้ามใส่ .private, env หรือ credentials ใน GitHub

## 4. เชื่อม GitHub กับ Consult Web

1. ใน Consult Web เลือก Source เป็น repository ที่ยืนยันแล้ว เลือก branch main และ root directory ที่มี Dockerfile
2. ใช้ Dockerfileที่root ไม่ใส่ custom Build/Start command Docker build Frontend แล้วให้ Go เสิร์ฟหน้าเว็บและAPIจาก originเดียวกัน
3. ใส่ Variables จาก railway-web.env: APP_ENV=production, APP_SECRET, STAFF_ACCOUNTS_JSON, EVENT_CONFIG=config/event.production.json, EMAIL_PROVIDER=off
4. DATABASE_URL=${{Postgres.DATABASE_URL}} ใช้ private networking ชื่อบริการต้องตรงตัวพิมพ์
5. Healthcheck Path=/api/health, timeout300วินาที ระบบอ่านPORTจากRailway ไม่ต้องกำหนดHTTP_ADDRเอง
6. Generate Domain ที่ Public Networking ให้มี HTTPS; PUBLIC_URLว่างได้เมื่อมี RAILWAY_PUBLIC_DOMAIN หากใช้โดเมนเองตั้ง PUBLIC_URLเป็นHTTPS originโดยไม่มีpath/query
7. ตรวจ Review changes ว่าเหลือเพียงเว็บ ฐานข้อมูล และ volume ไม่มี Backup/bucket/cron แล้วจึง Deploy
8. รอ deploymentสถานะSUCCESS ตรวจ /api/health, /#/book, /#/staff, /#/display และลิงก์ส่วนตัวจากมือถือ

ตาราง production ฉบับใหม่รวม 82 slots ใน 13 ช่วงเวลา: High Level เฉพาะ 6 พฤศจิกายน ใช้เวลาเดิม และ Beginner เฉพาะ 7 พฤศจิกายน 2026 ใช้เวลาใหม่ สัมมนารวมวันที่ 7 พฤศจิกายน: Math 11:30–13:00, Verbal 13:00–14:00 ดูตารางใน SCHEDULE_UPDATE_TH.md โค้ดไม่ล้างข้อมูลเมื่อ Deploy ใหม่

บัญชี7บัญชีสร้างเมื่อเริ่มฐานว่างครั้งแรก การDeployด้วยค่าเดิมไม่สุ่มรหัสใหม่ หากเปลี่ยนรหัส/สิทธิ์ภายหลังให้แก้ STAFF_ACCOUNTS_JSON แล้วเรียก /app/server -provision-staff เป็นงานดูแลครั้งเดียวด้วยตัวแปรproductionและฐานเดิมก่อนเปิดเว็บรุ่นใหม่ ไม่ใช้สคริปต์resetเทสกับรอบจริง

## 5. แพ็กเกจ Hobby $5 และการอัปเดตหน้าเว็บ

Hobbyมีเครดิตทรัพยากร$5ต่อรอบบิล หากใช้เกินมีส่วนต่าง ยังมีค่าCPU/RAMของเว็บและPostgresที่เปิดอยู่แม้ไม่มีคนเข้า จึงต้องตรวจ Usage/Estimated Usage หลัง Deploy รวมโปรเจกต์อื่นในworkspaceด้วย ไม่ได้รับประกันยอดรวม$5

| หน้า | รอบอัปเดตเมื่อเปิดใช้งาน |
| --- | --- |
| จอง / แก้ช่วงเวลา | 5วินาที |
| จองสำเร็จ | รอบว่างยังอัปเดต5วินาทีจนออกจากหน้านี้ |
| ผู้ปกครอง ทุกสถานะและทุกจำนวนคิว | 5วินาที |
| Staff / จอเรียกคิว | 5วินาที |

ซ่อนแท็บหรือofflineจะหยุดดึงข้อมูล กลับมาแล้วดึงทันทีถ้าข้อมูลเกินรอบ ไม่ซ้อนคำขอเมื่อเครือข่ายช้า หากผิดพลาดretry30/60/120วินาทีและผู้ใช้กดรีเฟรชได้ การแก้คิวในหน้าเดิมอัปเดตทันที เครื่องอื่นเห็นตามรอบ รอบปกติกลับมาเป็น5วินาทีทั้งหมดตามที่เลือก30ตุลาคม2026

Frontend+Goเป็นบริการเดียว ไม่ต้องเพิ่มบริการFrontendแยก ชุดทดสอบรันในเครื่องหรือGitHub Actions ไม่เปิดstagingบนRailwayค้างไว้ ยังไม่ได้ตั้งhard limitปิดเว็บอัตโนมัติ

## 6. ส่งอีเมลผ่าน Google Apps Script

ใช้ Google Apps Script MailApp เป็นผู้ส่ง ตามขั้นตอนใน [EMAIL_SETUP_TH.md](EMAIL_SETUP_TH.md) สร้าง Web app ที่ execute as Me และให้ Railway เรียกได้ พร้อมกุญแจเฉพาะการส่งเมล

```text
EMAIL_PROVIDER=apps-script
APPS_SCRIPT_URL=https://script.google.com/macros/s/YOUR_DEPLOYMENT_ID/exec
APPS_SCRIPT_SECRET=YOUR_PRIVATE_64_CHARACTER_HEX_SECRET
```

อัปโหลดโค้ดที่รองรับ Apps Script ก่อนเปิด provider ค่าที่ขาด/URL ที่ไม่ใช่ Web app จริงจะถูกปฏิเสธ แม่แบบเดิมยังมีช่วงเวลา Consult กับปุ่มติดตามคิว ส่งจากบัญชีที่ Deploy Script ไม่ต้องใช้ Resend หรือ SMTP App Password

การส่งจริงยังรอการ Deploy/อนุญาตสิทธิ์ของ Apps Script และ URL /exec จากผู้ใช้ ไม่อ้างว่ารับถึง Inbox จนกว่าจะทดสอบกับผู้รับที่กำหนด ดูการปรับ UI ใน [UI_UPDATE_TH.md](UI_UPDATE_TH.md)
