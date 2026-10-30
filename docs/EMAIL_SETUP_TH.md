# ส่งอีเมลยืนยันผ่าน Google Apps Script

ใช้บริการ MailApp ของ Apps Script เป็นผู้ส่งตามที่เลือก ไม่ต้องตั้งค่า Resend หรือ App Password แม่แบบ HTML และข้อความยังสร้างด้วย Go renderer เดิม พร้อมช่วงเวลา Consult, เวลาสัมมนา Beginner และปุ่มติดตามคิวส่วนตัว

## 1. อัปโหลดโค้ดเว็บก่อน

แตก SPR_Queue_Apps_Script_Update.zip แล้วอัปโหลดไฟล์และโฟลเดอร์ทั้งหมดทับที่ root ของ teaching01-netizen/SPR_Queue กด Commit changes และรอ Railway Deploy สำเร็จ ชุดนี้รวม UI ฉบับล่าสุดและตารางที่แก้แล้ว (6 พฤศจิกายนเวลาเดิม / 7 พฤศจิกายนเวลาใหม่ รวม 82 slots)

## 2. สร้าง Apps Script ด้วยบัญชีผู้ส่ง

1. เปิด https://script.google.com/home ด้วยบัญชี Google ที่ต้องการใช้เป็นผู้ส่งเมล
2. กด New project ตั้งชื่อ SPR_Queue Mail Relay
3. เปิดไฟล์ ops/apps-script/Code.gs จากชุดอัปเดต คัดลอกทั้งหมดไปวางทับ Code.gs ใน Apps Script แล้ว Save
4. ไปที่ Project Settings → Script Properties → Add script property ใส่ชื่อ APPS_SCRIPT_SECRET และค่าจากไฟล์ส่วนตัว SPR_Queue_Apps_Script_Secret.txt
5. กลับหน้า Editor เลือกฟังก์ชัน checkMailSetup แล้วกด Run อนุญาตสิทธิ์ส่งเมลด้วยบัญชีผู้ส่ง ฟังก์ชันนี้ตรวจสิทธิ์และโควตาเท่านั้น ยังไม่ส่งอีเมลให้ใคร

MailApp จะส่งจากบัญชีที่ Deploy เว็บแอป ชื่อผู้ส่งแสดง Warwick Institute ไม่ใช่อีเมลผู้รับหรืออีเมลที่ส่งมากับ request

มี appsscript.json แนบไว้สำหรับดูขอบเขตสิทธิ์ ต้องใช้เพียง script.send_mail ไม่ต้องให้สิทธิ์อ่านกล่อง Gmail หากวาง Code.gs อย่างเดียว Apps Script สามารถตรวจพบสิทธิ์ MailApp ที่ต้องใช้เมื่อ Run ได้

## 3. Deploy เป็น Web app

1. กด Deploy → New deployment → เลือก Web app
2. Execute as: Me (บัญชีผู้ส่ง)
3. Who has access: Anyone เพื่อให้ Go บน Railway เรียกได้โดยไม่ต้องล็อกอิน Google ทุกครั้ง คำขอส่งเมลยังต้องมีลายเซ็นที่สร้างด้วยกุญแจเชื่อมต่อ
4. กด Deploy และคัดลอก Web app URL ที่ลงท้าย /exec เช่น https://script.google.com/macros/s/DEPLOYMENT_ID/exec
5. ส่ง URL /exec กลับมาเพื่อเชื่อม Railway ไม่ใช้ /dev หรือ URL ชั่วคราวของ googleusercontent

ถ้าบัญชีองค์กรไม่มีตัวเลือก Anyone ต้องให้ผู้ดูแล Google Workspace อนุญาตก่อน การตั้ง Only myself หรือเฉพาะผู้ล็อกอินจะทำให้ Railway เรียกส่งไม่ได้

เมื่อแก้ Code.gs ภายหลัง ให้ใช้ Deploy → Manage deployments → Edit → New version แล้ว Deploy ใหม่ การ Save โค้ดอย่างเดียวไม่อัปเดตเว็บแอปที่ Deploy ไปแล้ว

## 4. เชื่อม Railway

ตัวแปรของ Consult Web ใน environment production:

| ชื่อ | ค่า |
| --- | --- |
| EMAIL_PROVIDER | apps-script |
| APPS_SCRIPT_URL | Web app URL /exec จากขั้นตอนก่อน |
| APPS_SCRIPT_SECRET | ค่าเดียวกับ Script Property |

ค่ากุญแจอยู่ในไฟล์ส่วนตัว แยกจาก ZIP ที่อัปโหลด GitHub ไม่ใช้ APP_SECRET ของฐานข้อมูลเป็นกุญแจส่งเมล ไม่ต้องเปลี่ยน DATABASE_URL หรือบัญชี Staff

ต้องอัปโหลดโค้ด Go ที่รองรับ apps-script และตั้ง URL/กุญแจให้ครบก่อนเปลี่ยน EMAIL_PROVIDER เว็บยังไม่เปิดส่งจริงจนกว่า Deploy สำเร็จ

## 5. ทดสอบรับจริง

กำหนดกล่องเมลทดสอบที่เข้าถึงได้ก่อน แล้วค่อยทดสอบการลงทะเบียนหรือคำสั่งทดสอบเมล ตรวจ Inbox/Spam เนื้อหา และปุ่มติดตามว่าตรงกับรายการของผู้รับ ไม่สร้างการจองจริงหรือส่งหาใครโดยไม่มีผู้รับทดสอบที่กำหนด

ตัว worker บันทึกสถานะ Pending / Sent / Failed และลองใหม่เมื่อส่งล้มเหลวตามกลไกเดิม การตอบ Sent หมายถึง MailApp รับคำสั่งส่งสำเร็จ ไม่ใช่หลักฐานว่าเข้า Inbox หากลงทะเบียนช่วงปิดอีเมล รายการเก่าจะไม่ถูกส่งย้อนหลังเอง

## การป้องกันการส่งซ้ำและโควตา

- Relay ตรวจ HMAC-SHA256 ของข้อมูลและเวลา ไม่รับคำขอที่ไม่มีลายเซ็น ถูกแก้ไข หรือเก่าเกิน 5 นาที
- ใช้ LockService และบันทึก request ID ของข้อความที่ส่งสำเร็จใน Script Properties เป็นเวลา 7 วัน (สูงสุด 2,000 รายการ) การลองใหม่ปกติของข้อความเดิมไม่ส่งซ้ำ การแก้รายละเอียดจองสร้าง request ID ใหม่
- หาก MailApp ส่งสำเร็จแล้วกระบวนการหยุดก่อนบันทึกผล อาจเกิดเมลซ้ำได้ การส่งระหว่างสองระบบนี้ไม่รับประกัน exactly-once
- ตรวจ getRemainingDailyQuota ก่อนส่ง เมื่อโควตาหมดจะไม่แจ้งว่าส่งสำเร็จ โควตานับร่วมกับ Apps Script อื่นของบัญชีเดียวกัน
- เอกสาร Google ระบุโควตาผู้รับต่อวันทั่วไป 100 สำหรับบัญชีส่วนตัว และ 1,500 สำหรับ Google Workspace; บัญชีทดลองและข้อจำกัดอื่นอาจต่างออกไป ให้ยึดค่าที่ checkMailSetup แสดงและนโยบายบัญชีจริง

## เอกสารทางการ

- https://developers.google.com/apps-script/guides/web
- https://developers.google.com/apps-script/reference/mail/mail-app
- https://developers.google.com/apps-script/guides/content
- https://developers.google.com/apps-script/guides/services/quotas

ตรวจเอกสาร 30 ตุลาคม 2026
