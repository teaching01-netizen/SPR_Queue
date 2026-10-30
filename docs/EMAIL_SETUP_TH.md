# เปิดอีเมลยืนยันหลังลงทะเบียน

## สถานะปัจจุบัน

โค้ดมีระบบส่งอีเมลแล้ว ใช้แม่แบบ HTML เดิม มีช่วงเวลา Consult, เวลาสัมมนาของ Beginner และปุ่ม Check Queue Status พร้อมลิงก์ส่วนตัวของผู้ปกครอง การจองบันทึกงานส่งลง PostgreSQL และ worker ส่งผ่าน Resend HTTPS พร้อมลองใหม่เมื่อเกิดข้อผิดพลาด

ตรวจ Railway วันที่ 30 ตุลาคม 2026: ยังไม่มี EMAIL_FROM และ RESEND_API_KEY และผู้ใช้ยังไม่มีบัญชี Resend ที่พร้อมใช้งาน จึงยังไม่ได้เปิดส่งจริง การเพิ่มชื่อผู้ให้บริการอย่างเดียวโดยไม่มีค่าที่เหลือจะทำให้แอปเริ่มไม่ได้ จึงต้องตั้งให้ครบในครั้งเดียว

## 1. สร้างบัญชีบริการอีเมล

เปิด https://resend.com/signup และสมัครด้วยอีเมลที่ผู้ดูแลสถาบันเข้าใช้งานได้ จากนั้นเข้าหน้า Dashboard

## 2. ยืนยันโดเมนผู้ส่ง

ใน Resend ไปที่ Domains → Add Domain แล้วเพิ่มโดเมนที่สถาบันเป็นเจ้าของ เช่น warwick-institute.com

ให้ผู้ดูแล DNS ของโดเมนเพิ่มรายการสำหรับการส่งอีเมลตามชื่อและค่าที่ Resend แสดงจริง แล้วกดยืนยันจนสถานะเป็น Verified ไม่ใช้ค่าตัวอย่างจากเว็บอื่น ใช้เฉพาะการส่งอีเมลสำหรับงานนี้ ไม่เปลี่ยนการรับเมลของระบบงานเดิม

## 3. สร้าง API Key สำหรับเว็บนี้

ไปที่ API Keys → Create API Key ตั้งชื่อ SPR_Queue เลือก Sending access และจำกัดโดเมนเป็นโดเมนที่ยืนยันแล้ว เก็บค่า Key ไว้สำหรับเพิ่มใน Railway โดยตรง ไม่ใส่ใน source code, frontend หรือ GitHub

## 4. เพิ่มค่าที่ Consult Web บน Railway

เปิด project warwick-consult-production → production → Consult Web → Variables แล้วเพิ่ม/แก้พร้อมกัน:

| ชื่อตัวแปร | ค่า |
| --- | --- |
| EMAIL_PROVIDER | resend |
| EMAIL_FROM | Warwick Institute <อีเมลผู้ส่งที่อยู่ในโดเมน Verified> |
| RESEND_API_KEY | API Key จากขั้นตอนก่อน |

ตัวอย่างรูปแบบ EMAIL_FROM: `Warwick Institute <consult@example.com>` โดย example.com เป็นเพียงตัวอย่าง ต้องเปลี่ยนเป็นอีเมลใต้โดเมนที่ยืนยันแล้วจริง

ไม่ต้องใช้ App Password ของกล่องเมลงาน ไม่ต้องแก้ APP_SECRET, DATABASE_URL หรือบัญชี Staff เมื่อบันทึกตัวแปรครบแล้วรอ Deploy สำเร็จ

## 5. ทดสอบรับอีเมลก่อนเปิดให้ผู้ปกครอง

เลือกกล่องเมลทดสอบที่ผู้ดูแลเข้าถึงได้ แล้วตรวจทั้ง Inbox และ Spam พร้อมกดปุ่มติดตามคิวว่าตรงกับรายการที่จอง ค่าบนระบบ Sent หมายถึงผู้ให้บริการรับคำขอแล้ว ไม่ใช่หลักฐานว่าเข้า Inbox หากยังไม่ได้กำหนดผู้รับทดสอบ จะไม่ส่งไปหาใครโดยพลการ

เมื่อเปิดบริการแล้ว การลงทะเบียนใหม่จะสร้างงานอีเมลอัตโนมัติ รายการที่ลงทะเบียนในช่วงปิดอีเมลจะไม่ถูกส่งย้อนหลังเอง

## เอกสารทางการ

- Verified domains: https://resend.com/docs/dashboard/domains/introduction
- API keys: https://resend.com/docs/dashboard/api-keys/introduction
- Sending-only API keys: https://resend.com/docs/api-reference/api-keys/create-api-key

ตรวจเอกสารวันที่ 30 ตุลาคม 2026
