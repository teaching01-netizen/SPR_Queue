# วิธีติดตั้งอัปเดต Booking Recovery

1. แตก ZIP นี้ ได้โฟลเดอร์ backend, frontend, docs และ README.md
2. เปิด GitHub teaching01-netizen/SPR_Queue ที่หน้า root ของ repository เลือก Add file > Upload files อัปโหลดโฟลเดอร์/ไฟล์เหล่านี้ทับของเดิม พร้อมไฟล์ใหม่ booking_recovery.go และ booking_recovery_test.go แล้ว Commit changes
3. รอ Railway Consult Web Deploy เป็น Success ใช้ฐานข้อมูลและตัวแปรเดิม ไม่ต้องล้างข้อมูล ไม่ต้องเปลี่ยนตารางหรือ provision บัญชีใหม่
4. รีเฟรชเว็บแล้วเข้า Staff ด้วย RD1 เปิดรายการที่ต้องการ กด คืนสถานะ / คืนที่ว่าง
5. อ่าน docs/BOOKING_RECOVERY_TH.md ก่อนเลือกคืนผู้ปกครองจริงหรือยกเลิกข้อมูลทดลอง

โค้ดนี้ยังไม่ได้ติดตั้งบนเว็บจริง และยังไม่ได้แก้ไขรายการจริงใดๆ
