# นำโค้ด Warwick Consult เข้า GitHub

อัปเดต29ตุลาคม2026 · ชุดนี้ไม่มีระบบสำรอง/กู้คืนฐานข้อมูล

## ขั้นตอนแรก: สร้าง repository ว่าง

1. เปิด https://github.com/new ด้วยบัญชีที่ต้องการเป็นเจ้าของแอป
2. ตั้งชื่อแนะนำ warwick-consult และเลือก Private
3. ไม่เลือกเพิ่ม README, .gitignore หรือ License เพราะชุดโค้ดมีไฟล์เตรียมแล้ว
4. กด Create repository แล้วส่งลิงก์รูปแบบ https://github.com/OWNER/warwick-consult เพื่อเชื่อมและอัปโหลดโค้ดต่อ

ตัวเชื่อมGitHubที่ตรวจในงานนี้ลงชื่อเป็น warwickchatgpt1-bot ไม่ควรสมมติว่าเป็นบัญชีเดียวกับRailway เจ้าของrepositoryต้องให้GitHub Appที่เชื่อมต่อเข้าถึงrepoนั้น และRailwayต้องเข้าถึงrepoนั้นด้วย

## อัปโหลดด้วย Git ในเครื่อง

แตก Warwick_Railway_Deploy.zip หรือ Warwick_GitHub_Ready.zip แล้วเปิดTerminalในโฟลเดอร์ที่มีDockerfile, backend/, frontend/:

```bash
git init -b main
git add .
git status --short
git commit -m "Initial Warwick Consult web application"
git remote add origin https://github.com/OWNER/warwick-consult.git
git push -u origin main
```

แทนOWNERด้วยบัญชีจริง หากโฟลเดอร์เป็นGitอยู่แล้วให้ตรวจstatus/remoteก่อน ไม่รันเพิ่มremoteซ้ำ ไม่force pushทับrepositoryที่มีงานอื่น หากใช้งานผ่านGitHub Desktopให้Add local repositoryแล้วPublish repositoryแบบPrivateได้

ก่อนcommitต้องไม่มี .private/, .env*, staff-credentials.csv, token, database dump, data/, node_modules/ หรือ frontend/dist/ อยู่ในรายการstage ชุดZIPสำหรับGitHubมีเฉพาะsourceและไม่มีค่าลับจริง สร้างค่าลับหลังนำโค้ดเข้าrepoด้วย scripts/prepare-railway.sh ได้

## ตรวจหลัง push

- rootต้องมีDockerfile, README.md, backend/, frontend/, scripts/, .github/workflows/ci.yml ไม่ใช่ZIPไฟล์เดียว
- GitHub ActionsตรวจNode tests, frontend build, Go testsกับPostgreSQL17, go vet และDocker build ต้องตรวจผลจริงในrepo
- ไม่มีworkflow restore-drill และไม่มี ops/backup หรือ ops/drill
- ใช้repositoryนี้เป็นsourceของ Consult Web ในRailwayตาม RAILWAY_DEPLOY_TH.md ขั้นตอนถัดไป

ขณะจัดเตรียมชุดนี้ยังไม่ได้สร้างrepositoryหรือpushขึ้นGitHub เพราะยังไม่มีrepositoryปลายทางที่ผู้ใช้ยืนยัน
