# Warwick Frontend — Vite + React + OpenAI Apps SDK UI

UI นี้ใช้ข้อมูลจาก Go Backend ไม่ใช้ localStorage สำหรับ bookings และไม่มีปุ่ม reset ข้อมูลตัวอย่าง

```bash
npm ci
npm run build
```

รัน Go Server ตาม README ที่ root แล้วเปิด `http://127.0.0.1:8080`

พัฒนาแบบ HMR: เปิด Go Server ที่ port 8080 แล้ว `npm run dev` เปิด `http://127.0.0.1:5173` Vite proxy `/api` ให้

- Staff ต้อง Login ก่อน fetch รายชื่อนักเรียน และ session/CSRF เก็บตามกลไก HTTP
- วัน รอบ และ attendance mode โหลดจาก `/api/event`
- Parent/Display ใช้ public projections แยกจาก Staff
- ไม่มีเวลา Estimated consult หรือ ETA
- ถ้า Backend ไม่เปิด UI จะแจ้งให้ลองเชื่อมต่อใหม่

ใช้ source ในชุดนี้ร่วมกับ Go Backend ไม่ใช่ standalone HTML preview จากรุ่นก่อน

ใช้ `@openai/apps-sdk-ui` Button, Input และ Badge จริง พร้อม Tailwind CSS 4 ผ่าน `@tailwindcss/vite` ไม่ต้องตั้ง OpenAI API key เพื่อใช้ UI components ดู `src/App.tsx`, `src/styles.css` และ `vite.config.ts`

สำหรับทดลองทั้งระบบในเครื่องด้วย Docker Desktop ใช้ `bash scripts/local-test.sh` ที่ root อ่าน `../docs/LOCAL_TEST_TH.md` จะเปิด Go API และ PostgreSQL ของ Local Test พร้อม frontend build ให้
