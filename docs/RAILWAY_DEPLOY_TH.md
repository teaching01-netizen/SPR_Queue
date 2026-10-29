# นำ Warwick Consult ขึ้น Railway

อัปเดต 29 ตุลาคม 2026 · Vite/React + Go + PostgreSQL

ระบบสำรองและกู้คืนฐานข้อมูลถูกถอดออกตามที่เลือกไว้ ใช้เพียง Consult Web และ Postgres พร้อม persistent volume ฟังก์ชันคืนการจองที่ยกเลิกและเก็บประวัติในหน้า Staff ยังใช้งานตามเดิม

## 1. นำโค้ดเข้า GitHub ก่อน

ทำตาม [GITHUB_SETUP_TH.md](GITHUB_SETUP_TH.md) ใช้ repository ส่วนตัวสำหรับแอปนี้โดยเฉพาะ ใน root ต้องมี Dockerfile, backend/, frontend/, scripts/ และ .github/ ไม่อัปโหลด ZIP เป็นไฟล์เดี่ยวแทน source code

## 2. สถานะ Railway ที่เตรียมไว้

Project: [warwick-consult-production](https://railway.com/project/60d16b28-22cf-4a5e-9bee-14f7eb3998ab?environmentId=ec81e3a0-a2fa-4529-b4d6-2e89e7ec8c30)

- Environment: production
- Consult Web: 1f385e9c-478b-4dec-99e2-69cf08af194c
- Postgres: 522dda9f-650c-4e7a-933f-52cc3d8570e0
- postgres-volume: 6f2a0411-65b7-4d30-9334-b1bc5f12222b
- Region: Singapore, หนึ่ง replicaต่อบริการ

บริการยังเป็น staged ยังไม่ Deploy และยังไม่เชื่อม source repository ไม่ต้องสร้าง project/ฐานข้อมูลซ้ำ

ยกเลิก staged creation ของ Consult Backup แล้ว Bucket consult-backups ถูกลบแล้วและไม่มี provisioned instances แต่ Railway ยังมีรายการสร้าง bucket ID 6338ee60-37e8-4378-be4e-c43ef0542cd1 ค้างใน pending patch โดยแสดงชื่อพร้อมวันที่ต่อท้าย ต้องเปิด Review changes และ Discard เฉพาะรายการ bucketนี้ก่อน Deploy เก็บรายการ Consult Web, Postgres และ postgres-volume ไว้ ยังไม่ได้เปิด scheduled snapshot ของ volumeจากงานนี้

## 3. สร้างค่าเว็บและบัญชี Staff

ในโฟลเดอร์ที่แตกไฟล์แล้ว รัน:

```bash
bash scripts/prepare-railway.sh
```

สร้างใน .private/railway/ เฉพาะ railway-web.env, staff-credentials.csv และ README_PRIVATE.txt ไม่สร้างตัวแปร Backup, AWS, bucket หรือ recovery keys

บัญชี RD1, RD2, RD3, SM1, SM2, SM3, SM4 มีรหัสสุ่มแยกกัน RD1เป็นadmin บัญชีอื่นเป็นoperator กุญแจ APP_SECRET ใช้เข้ารหัสลิงก์ติดตามคิว ต้องเก็บค่าเดิมไว้กับฐานเดิม ตัวสร้างไม่ทับโฟลเดอร์ที่มีอยู่แล้ว ห้ามใส่ .private, env หรือ credentials ใน GitHub

## 4. เชื่อม GitHub กับ Consult Web

1. ใน Consult Web เลือก Source เป็น repository ที่ยืนยันแล้ว เลือก branch main และ root directory ที่มี Dockerfile
2. ใช้ Dockerfileที่root ไม่ใส่ custom Build/Start command Docker build Frontend แล้วให้ Go เสิร์ฟหน้าเว็บและAPIจาก originเดียวกัน
3. ใส่ Variables จาก railway-web.env: APP_ENV=production, APP_SECRET, STAFF_ACCOUNTS_JSON, EVENT_CONFIG=config/event.production.json, EMAIL_PROVIDER=off
4. DATABASE_URL=${{Postgres.DATABASE_URL}} ใช้ private networking ชื่อบริการต้องตรงตัวพิมพ์
5. Healthcheck Path=/api/health, timeout300วินาที ระบบอ่านPORTจากRailway ไม่ต้องกำหนดHTTP_ADDRเอง
6. Generate Domain ที่ Public Networking ให้มี HTTPS; PUBLIC_URLว่างได้เมื่อมี RAILWAY_PUBLIC_DOMAIN หากใช้โดเมนเองตั้ง PUBLIC_URLเป็นHTTPS originโดยไม่มีpath/query
7. ตรวจ Review changes ว่าเหลือเพียงเว็บ ฐานข้อมูล และ volume ไม่มี Backup/bucket/cron แล้วจึง Deploy
8. รอ deploymentสถานะSUCCESS ตรวจ /api/health, /#/book, /#/staff, /#/display และลิงก์ส่วนตัวจากมือถือ

ตาราง productionรวม105slots วันที่6–7พฤศจิกายน2026 สัมมนารวมวันที่7พฤศจิกายน: Verbal08:30–09:20, Math09:30–10:20 ตรวจรายละเอียดก่อนเปิดรับจอง โค้ดไม่ล้างข้อมูลเมื่อDeployใหม่

บัญชี7บัญชีสร้างเมื่อเริ่มฐานว่างครั้งแรก การDeployด้วยค่าเดิมไม่สุ่มรหัสใหม่ หากเปลี่ยนรหัส/สิทธิ์ภายหลังให้แก้ STAFF_ACCOUNTS_JSON แล้วเรียก /app/server -provision-staff เป็นงานดูแลครั้งเดียวด้วยตัวแปรproductionและฐานเดิมก่อนเปิดเว็บรุ่นใหม่ ไม่ใช้สคริปต์resetเทสกับรอบจริง

## 5. แพ็กเกจ Hobby $5 และการอัปเดตหน้าเว็บ

Hobbyมีเครดิตทรัพยากร$5ต่อรอบบิล หากใช้เกินมีส่วนต่าง ยังมีค่าCPU/RAMของเว็บและPostgresที่เปิดอยู่แม้ไม่มีคนเข้า จึงต้องตรวจ Usage/Estimated Usage หลัง Deploy รวมโปรเจกต์อื่นในworkspaceด้วย ไม่ได้รับประกันยอดรวม$5

| หน้า | รอบอัปเดตเมื่อเปิดใช้งาน |
| --- | --- |
| จอง / แก้ช่วงเวลา | 30วินาที |
| จองสำเร็จ | หยุดดึงรอบว่าง |
| ผู้ปกครอง | 10วินาที; เหลือไม่เกิน2คิวหรือCalling/Servingใช้5วินาที |
| Completed / Cancelled / No-show | 60วินาที เพื่อตรวจพบการคืนรายการ |
| Staff / จอเรียกคิว | 5วินาที |

ซ่อนแท็บหรือofflineจะหยุดดึงข้อมูล กลับมาแล้วดึงทันทีถ้าข้อมูลเกินรอบ ไม่ซ้อนคำขอเมื่อเครือข่ายช้า หากผิดพลาดretry30/60/120วินาทีและผู้ใช้กดรีเฟรชได้ การแก้คิวในหน้าเดิมอัปเดตทันที เครื่องอื่นเห็นตามรอบ หน้าเว็บลดคำขอไม่ได้แปลว่าบิลลดในสัดส่วนเดียวกัน

Frontend+Goเป็นบริการเดียว ไม่ต้องเพิ่มบริการFrontendแยก ชุดทดสอบรันในเครื่องหรือGitHub Actions ไม่เปิดstagingบนRailwayค้างไว้ ยังไม่ได้ตั้งhard limitปิดเว็บอัตโนมัติ

## 6. อีเมลจริงเมื่อพร้อม

Hobbyใช้SMTPไม่ได้ โค้ดรองรับResendผ่านHTTPSโดยไม่ใช้รหัสผ่านเมลงาน:

```text
EMAIL_PROVIDER=resend
EMAIL_FROM=Warwick Institute <consult@YOUR_VERIFIED_DOMAIN>
RESEND_API_KEY=YOUR_PRIVATE_API_KEY
```

ต้องยืนยันsending domainและDNSก่อน ส่งทดสอบไปผู้รับที่กำหนดแล้วตรวจInbox/Spamจริง Sentหมายถึงผู้ให้บริการรับคำขอ ยังไม่ยืนยันว่าเข้าInbox อีเมลใช้templateเดิม มีช่วงเวลาConsultและปุ่มลิงก์ส่วนตัว หากยังไม่พร้อมให้คงEMAIL_PROVIDER=off และใช้Staffคัดลอกลิงก์

เอกสารทางการ:
- https://docs.railway.com/pricing/plans
- https://docs.railway.com/networking/outbound-networking
- https://docs.railway.com/networking/public-networking
- https://docs.railway.com/deployments/healthchecks
- https://resend.com/docs/api-reference/emails/send-email
