# Validation — PostgreSQL / OpenAI UI update — 13 October 2026

## ตรวจผ่านในสภาพแวดล้อมนี้

- Go compile, `go vet ./...`, private environment setup tests พร้อม race detector
- TypeScript check และ Vite production build พร้อม Tailwind 4 / OpenAI Apps SDK UI
- Backend regression tests 23 cases ใน package consult (22 integration + 1 schedule configuration test) ผ่าน PostgreSQL engine แบบ **PGlite 0.5.9 + pglite-socket 0.2.12** ผ่าน PostgreSQL wire protocol และ pgx พร้อม Go race detector
- 40 simultaneous booking requests จาก Go Service สองชุด → สำเร็จ 4 รายการ, PERIOD_FULL 36 รายการ ใน engine ที่ใช้ตรวจ
- Same idempotency key/payload คืน booking/token เดิม; payload ต่างกันรับ conflict
- Input validation, Cancel คืน capacity, No-show ยังครองที่, revision conflicts และ atomic reschedule
- State transitions, stable queue numbers, single active stream, No-show recovery และ Combined Math → Verbal → Completed
- Reopen Service ยังอ่าน booking/tracking ได้; token ไม่เก็บ plaintext และ key ผิดถูกปฏิเสธ
- Email failure/retry/cancellation/delivery lease ผ่าน fake sender tests
- Staff authorization/CSRF/origin/session revocation และ parent/public projection ผ่าน
- PostgreSQL JSONB/foreign-key/schema SQL ทำงาน; direct SQL insert รายการที่ 5 ถูก capacity trigger ปฏิเสธด้วย 23514 / booking_capacity
- Reopen migration ไม่รันซ้ำ; checksum ที่เปลี่ยนถูกปฏิเสธ
- Concurrent Staff bootstrap ไม่เปลี่ยนรหัสเดิมหรือเพิกถอน session จาก startup ซ้ำ
- Chromium browser ต่อ Vite build ที่ Go เสิร์ฟจริง: independent parent/staff contexts, login/logout, 8 concurrent API requests รับเพียง 4, conflicting Staff save, Combined workflow, public display, dynamic dates และมือถือ 390px
- Browser ใช้ OpenAI UI Button/Input/Badge จริง ไม่พบ JavaScript page errors หรือ horizontal overflow ในหน้าจองมือถือ และ log ไม่บันทึก tracking token/รหัส Staff

## ตรวจเพิ่มหลังปรับรอบจอง

- ตาราง Math 4 รอบ, Verbal 5 รอบ และ Combined 3 รอบตรงตามเวลาที่แจ้ง; รอบปกติ 4 slots และ Combined 15:00–16:30 รับ 6 slots
- 20 concurrent requests ไป Combined 15:00–16:30 จาก Service สองชุด → สำเร็จ 6, PERIOD_FULL 14 บน PGlite; การยกเลิกหนึ่งรายการคืนหนึ่งที่ และการจองแทนยังไม่เกิน 6
- Verbal 16:30–18:00 และ Combined 15:00–16:30 นับโควตาแยกกัน; Combined รอบหนึ่งชั่วโมงรับ 4 เท่านั้น
- รอบ Math เดิม 13:00–14:00 และ Combined ที่ใช้ ID ของรอบเดิม 15:00–16:00 ถูกปฏิเสธ
- แก้ Normalize ให้คัดลอก questions slice ก่อน trim; Input เดียวที่ใช้พร้อมกันหลาย goroutines ผ่าน race detector
- Browser ตรวจจำนวน/เวลาทุกรอบ, label 6 slots, confirmation 15:00–16:30, full state, Staff MV 6/6 และเลือกวัน/วิชาที่มีรอบจริง

## Local-test launcher update — 14 October 2026

- Node initializer tests 3 cases: random independent credentials, separate sample config, preserving repeated initialization/custom edits และ rejecting environment redirected to another database
- Bash launcher syntax ผ่าน; Compose YAML ตรวจ project name, loopback-only app port, unpublished database port, separate volume, database readiness และ app healthcheck
- Go CGO-disabled static build ผ่าน พร้อมฝัง timezone data สำหรับ minimal container; healthcheck tests ตรวจ HTTP 200, unhealthy response และ unreachable endpoint
- Vite/TypeScript build และ browser local-test banner/booking/Staff flow ผ่าน PostgreSQL engine PGlite
- แนบ Dockerfile build Frontend/Go แบบ multi-stage และ CI step สำหรับ docker build แล้ว
- ยังไม่ได้รัน image build/Compose/native PostgreSQL 17/Docker Desktop/PowerShell ในสภาพแวดล้อมนี้; ไม่มี Docker daemon ผลตรวจข้างต้นไม่ใช่การอ้างว่าเปิด containers แล้วบนเครื่องผู้ใช้

## ขอบเขตของการตรวจ PostgreSQL

ยังไม่ได้รัน **native PostgreSQL server / Docker Compose / GitHub Actions** ในสภาพแวดล้อมนี้ PGlite เป็น PostgreSQL engine แบบ WebAssembly และ socket adapter จัดลำดับ transactions ภายใน จึงไม่ยืนยันพฤติกรรม row locks, advisory locks, transaction isolation หรือ throughput ของ PostgreSQL ที่มีหลาย backend processes จริง

Adapter ไม่ใช้ connection startup `search_path` แบบ server ปกติ การตรวจที่นี่จึงแยก PGlite instance ใหม่ต่อ test และใช้ pgx simple protocol ส่วน integration fixtures สำหรับ native PostgreSQL ที่แนบแยก random schema ผ่าน `search_path` และใช้ pgx mode ปกติ

ต้องรัน `TEST_DATABASE_URL=<test database URL> go test -race -count=1 ./...` กับ native PostgreSQL โดยเฉพาะ concurrency tests ก่อน deploy แนบ CI workflow ที่ตั้ง PostgreSQL 17 service และรัน frontend build แล้ว ยังไม่ได้รัน workflow นั้นบน GitHub

## ยังไม่ได้ยืนยัน

- Deployment/load test/backup restore บน server จริง
- ส่งอีเมลถึงผู้รับจริง; SMTP implementation รองรับ STARTTLS แต่ผลตรวจใช้ fake sender
- Safari/Firefox และเครื่องของทีมงาน
- การนำข้อมูล SQLite เดิมเข้า PostgreSQL: ไม่มี import อัตโนมัติ
- SSO/account-management UI; รุ่นนี้มี Admin เริ่มต้นจาก environment
- Google Sheets/student eligibility/AppSheet/Zoom integration
- วันงานจริง และการแบ่งทรัพยากรอาจารย์ร่วมกันระหว่าง booking subjects/levels

ผลนี้ระบุ implementation และ test scenarios ที่ตรวจแล้ว โดยแยกขอบเขต native PostgreSQL ที่ยังต้องตรวจอย่างชัดเจน

## Docker context fix — 14 October 2026

- พบจาก screenshot ผู้ใช้ว่า `**/server` ใน `.dockerignore` กรอง `backend/cmd/server` ออก จึง build error `stat /src/cmd/server: directory not found`
- เปลี่ยน rule เป็น `backend/server` และแก้ `.gitignore` เป็น binary paths ที่ root/backend โดยคง source folder ไว้
- ใช้ Moby patternmatcher v0.6.0 / ignorefile reader จำลอง Docker filtering: reproduce old exclusion; ตรวจ source entrypoint/migrations ถูกส่งเข้า context และ secrets/node_modules/dist ยังถูกกรอง
- Build Go แบบ CGO-disabled จาก backend tree ที่ผ่าน matcher filtering สำเร็จ ขอบเขตนี้ตรวจการกรองและ compilation; ยังไม่มี Docker daemon สำหรับยืนยัน image/Compose เต็มชุด

## Solo slot revision — 14 October 2026

- Math 4 รอบ / Verbal 5 รอบตรงตามคำขอใหม่ ทั้งสองวิชาเปิด 16:30–18:00 ความจุ 6 รอบอื่น 4; Combined config คงเดิม
- 20 concurrent requests ต่อวิชาในรอบ 16:30–18:00 → รับ 6 / PERIOD_FULL 14 ผ่าน PostgreSQL engine PGlite พร้อม race detector และตรวจ Math/Verbal quota แยกกัน
- ปฏิเสธ P1700 เดิม และ Verbal ช่วงบ่าย P1300/P1400/P1500 ที่ถอดออก
- Browser ตรวจ Math 4 / Verbal 5, label/full state ของรอบ 6, Staff M 6/6 และ V 6/6, การจัดคิว 6 รายการ และ Combined handoff เดิม
- Node tests รวม 4: เพิ่มการตรวจ updater เปลี่ยนทั้ง source/local config โดยรักษา Event identity/Combined/credentials, backup และผลรันซ้ำคงเดิม
- Script updater ไม่ย้าย/ลบรายการจองเดิม; ไม่มี Docker daemon สำหรับทดสอบ launcher บน native PostgreSQL จริง ขอบเขต PGlite ยังคงตามที่ระบุข้างต้น

## Staff check-in / flexible Combined update — 14 October 2026

- Consult regression tests รวม 25 cases ผ่านพร้อม race detector บน PGlite; เพิ่มการเลือกวิชาเริ่มต้นทั้งสองลำดับ การรักษา booking/queue/check-in, reset room order, ห้ามสลับหลังเรียกแม้พักคิวแล้ว, จบทั้งสองวิชา, concurrent revision conflict, cross-event/solo/Cancelled restriction และ No-show recovery
- HTTP endpoint `/api/staff/bookings/{id}/start-subject` ตรวจ authentication/CSRF/origin, unknown-field rejection, revision และ actor จาก server session
- Go static build / vet / healthcheck tests ผ่าน; Node initializer/slot-updater tests 4 ผ่าน; TypeScript/Vite production build ผ่าน
- Chromium ต่อ Go backend + PGlite: checkbox visible state + keyboard Space, selected Beginner, all-period/date/level check-in, search Email/Wcode, strict All/Math/Verbal/Combined booked-subject filters, Verbal → Math full workflow, room busy แม้ Combined ถูก filter ซ่อน, stage progress, stable code, parent tracking และ logout ผ่าน
- ตรวจภาพหน้าจอง/Staff desktop และมือถือ 390px; checkbox เห็นชัด, check-in เป็นรายการมีปุ่มในหน้าจอ, period buttons ไม่บีบรวมทุกรอบ, ไม่มี page errors หรือ page horizontal overflow
- Standalone updater ใช้ embedded source patch สำรอง source ก่อนเขียน ไม่รวม env/config/DB; ตรวจด้วย fixture ว่าการอัปเดตและรันซ้ำเก็บ secret/config/data เดิมและ payload checksum ถูกต้อง
- ขอบเขตที่ยังไม่ได้ตรวจ: native PostgreSQL lock/isolation/load, Docker Desktop build/Compose เต็มชุด, Safari/Firefox และ SMTP จริง ยังคงตามรายละเอียดข้างต้น

## Booking edit / cancel archive / restore update — 14 October 2026

- Consult tests รวม 33 cases ผ่านพร้อม race detector บน PGlite; เพิ่มแก้วิชา Math/Verbal/Combined, เปลี่ยนรอบเก็บ queue code/check-in, เปลี่ยนวันล้าง check-in, stage history guard, capacity rollback, archive edit, No-show cancel/restore และ partial Combined resume
- Concurrent restores และ edit แข่งกับ restore สำหรับ slot สุดท้ายรับได้หนึ่งรายการ อีกคำขอได้ PERIOD_FULL บน engine ที่ใช้ตรวจ
- Preserve booking/token, queue prefix ใหม่เมื่อเปลี่ยนวิชา, reset orders, stale revision/cross-event rejection, before/after scope/stage/cancel reason audit ผ่าน
- HTTP operators แก้/คืนได้; anonymous, missing CSRF, forged origin/actor/level ถูกปฏิเสธ; preflight รองรับ PATCH
- Notification busy ป้องกันแก้ระหว่างส่ง; cancelled outbox ถูกข้าม และ restore เข้า Pending ส่งข้อมูลวิชา/รอบล่าสุดผ่าน fake sender
- Go static build, vet, cmd/server race tests; TypeScript/Vite build และ Node initializer/slot tests 4 ผ่าน
- Chromium ใช้งานจริงผ่านหน้า Staff: edit modal/Beginner confirmation, time move, subject changes/new queue prefix, cancellation hides active row, archive/reason, full restore blocked, archived new-period edit+restore, date-change check-in, unchanged tracking token, partial Combined restore/finish และ mobile modal layout ผ่าน
- Browser regression ของ checkbox, all-period check-in, booked-subject filters, either-room handoff, shared room busy และ logout ผ่านหลังเพิ่ม management UI
- Source updater fixture ตรวจ repeated overlay/backup/checksum และรักษา env/config/data เดิม; ไม่ได้รัน Docker Desktop หรือ native PostgreSQL server ในสภาพแวดล้อมนี้ ขอบเขต PGlite/SMTP/browser ตามที่ระบุข้างต้น

## November schedule + clean local-test reset — 14 October 2026

- ตารางจริงใน config ตรวจครบ 7 level/subject/date scopes, 20 periods, รวม 105 slots; 6–7 November 2026 และ seminar Verbal 08:30–09:20 / Math 09:30–10:20 วันที่ 7 พ.ย. ตรงตามคำขอ รอบไม่ระบุจำนวนใช้ 4
- Test fixtures เก็บ catalog เก่าแยกใน `testdata/regression-event.json` เพื่อทดสอบ lifecycle/permission/edit/restore เดิมโดยไม่ผูกกับเวลาใหม่; application อ่าน config November เท่านั้น
- Consult regression tests รวม 38 cases ผ่านพร้อม Go race detector บน PGlite รวมการจองให้เต็มทุกหนึ่งใน 20 รอบ ปฏิเสธคำขอเกินทุกโควตา และปฏิเสธวัน/วิชาที่ไม่ได้เปิด
- ยิงพร้อมกันจาก Service สองชุดในรอบ 2/3/5/7/8 slots รับไม่เกินจำนวนที่ระบุ; ขอบเขต PGlite เรื่อง native locks ยังคงตามคำอธิบายเดิม
- Reset ล้าง bookings/audit/outbox/sequences/periods/sessions ของทุก event ในฐานข้อมูลทดสอบ แล้ว seed catalog ใหม่ 20 rows พร้อมเพิ่ม revision เก็บ users/password hash/encryption key fingerprint/migrations เดิม
- Reset ถูกปฏิเสธใน non-sample/non-local event; ตรวจ CLI boundary ไม่ให้ใช้ production/development, remote host, DB/user อื่น; seed failure ทำให้ transaction rollback และข้อมูลเก่ายังอยู่
- Go build/vet และ cmd/server race tests ผ่าน; Node tests 5 ตรวจ initializer และ November config replacement ที่เก็บ credentials/รันซ้ำได้ พร้อมปฏิเสธ env นอก database ทดสอบ
- Chromium ต่อ Go + PGlite ตรวจ date options/periods/capacity/สัมมนาทุก scope, จอง Combined และ Verbal → Math, full 2-slot, cancel, one-off `-reset-local-test-data` CLI ที่ออกทันที, old token/session ถูกเพิกถอน, Login รหัสเดิม, archive ว่าง, capacity กลับมา, queue เริ่ม B-M-001, แก้วิชาใน catalog ใหม่ และมือถือผ่าน
- Updater fixture ตรวจ source/config backup, checksum, apply ซ้ำ, เก็บ env, เขียน reset/preserving defaults ถูกต้อง และ Docker orchestration แบบ mock: build → stop app → one-off reset → start app เฉพาะ Compose project ทดสอบ
- ไม่มี Docker daemon ในสภาพแวดล้อมนี้ จึงยังไม่ได้รัน native PostgreSQL 17 / Docker Desktop image build และ Compose เต็มชุดจริง; การตรวจ CLI/SQL ใช้ engine PGlite และการตรวจลำดับ Docker ใช้ fixture ไม่อ้างว่าเคลียร์ DB บนเครื่องผู้ใช้แล้ว

## Date cards + seven local Staff accounts — 14 October 2026

- Consult tests 40 cases ผ่านพร้อม Go race detector บน PGlite; เพิ่ม exactly-seven roster, bcrypt hashes, operator role, correct/incorrect login, old staff removal และ session revocation, restart preserves matching sessions/bookings/revision; ปฏิเสธ fixed credentials นอก local sample config
- HTTP Login/check-in ผ่านทั้ง RD1–RD3 และ SM1–SM4 โดยส่ง session/CSRF ของแต่ละบัญชีและตรวจ Staff/audit actor จาก server
- Go static build/vet และ cmd/server race tests ผ่าน; TypeScript/Vite production build และ Node initializer/catalog tests 5 ผ่าน
- Chromium ต่อ Go + PGlite: 6/7 November date cards พร้อม Zoom/On-site, selected aria-pressed/checkmark, keyboard Space, กดวันเดิมเก็บเวลา/เปลี่ยนวันล้างเวลา, High Level Verbal booking วันที่7, Login 7 บัญชีผ่าน UI, RD1 check-in และ old staff rejection; restart ใน DB เดิมเก็บ booking และ tracking token
- ตรวจภาพ desktop/390px mobile ไม่มี horizontal overflow/page errors; preserving updater fixture ตรวจ backup, repeated overlay, checksum และ config/env/data ไม่ถูกเปลี่ยน ไม่มีการเรียก reset ใน Staff-users updater
- Local-test startup sync เฉพาะ 7 บัญชีที่ผู้ใช้ระบุ โหมดอื่นยังใช้ Bootstrap รหัสส่วนตัวตามเดิม; ชุดทดสอบตัวใหม่นี้ไม่ล้าง bookings/queue/audit/outbox
- ไม่มี Docker daemon/native PostgreSQL 17 ในสภาพแวดล้อมนี้ จึงยังไม่ได้รัน Docker Desktop build/Compose จริงบนเครื่องผู้ใช้ ขอบเขต PGlite, SMTP และ browser เดิมยังมีผล

## HTML confirmation / SMTP setup / upcoming display — 14 October 2026

- Consult tests 44 top-level cases ผ่าน Go race detector บน PGlite รวม template/MIME, real SMTP client into isolated TLS receiver (STARTTLS and implicit TLS), correct private token in both MIME parts and outbox Sent; no external mailbox was used
- HTML escapes user text, rejects unsafe tracking URL, includes actual November seminars only for Beginner, both subjects for Combined, exact booked-period range and Check Queue Status; text/plain and HTML are quoted-printable MIME alternatives
- Upcoming display respects saved order, checked-in Waiting state, active period, level/date/room; excludes unordered/no-show/cancelled and unready Combined handoff; public output contains codes and period times only
- Go build/vet and cmd/server race tests passed; TypeScript/Vite production build passed; Node tests 7 cover separate private SMTP overlay, validation, quote/dollar-containing passwords, preserved DB secrets/config/SMTP after startup
- SMTP setup PTY test verified prompts complete, password not echoed, 0600 file saved; Docker launcher/default preserving updates checked with mock fixtures, not actual Docker Desktop
- Chromium: actual Math/Verbal bookings, check-in/order/call, two upcoming codes in both rooms, no names/contact/questions, desktop/390px mobile; actual Go-rendered email button reaches the corresponding booking and responsive layout works
- Headless Chromium lacks Thai system fonts; email visual QA loads the bundled Thai glyph fallback for rendering only. Actual email uses native Arial/Tahoma/sans-serif with inline styles. Gmail/Outlook/mail-client rendering and inbox placement are not verified here
- Staff booking details now expose email delivery status for verification. SMTP is off by default until configured. No automatic backfill for old Not configured bookings, no data reset, no extra queue-count feature added
- Native PostgreSQL/Docker Desktop, real SMTP provider authentication and external delivery still require the user's local environment/account. Localhost email links work only on the machine running the app until deployment with a reachable PUBLIC_URL

## Personal remaining-queue count / Warwick sender shortcut — 14 October 2026

- Consult regression 46 top-level cases checked with race detector on PGlite: 44 passed in the full run, two fixtures required updates (mandatory cancellation reason and privacy projection now allows two count fields) and those two passed on targeted rerun after correction; no application failure remained
- Count test: active1 to target5=4, cancel=>3, No-show=>2, Staff reordering=>1, current Calling/Serving=0, Completed=null; separate tests cover period boundaries, level/date/room, latest called anchor and Combined second-room order
- Public tracking privacy test uses an explicit seven-field allowlist; no student names/contact/questions/period IDs are exposed with the count
- Chromium actual Go API/UI confirms the same transitions, check-in/unassigned placeholders, Completed hides count, desktop/390px mobile no overflow/page errors
- Go static build/vet and cmd/server race tests passed; TypeScript/Vite production build and Node configuration/catalog tests 8 passed
- Warwick sender shortcut presets SMTP host587/STARTTLS/account/From. A PTY test confirms it asks only one hidden App Password, strips display spaces, saves 0600 SMTP overlay and does not echo the test secret
- User's ordinary account password is not placed in source, ZIP, guide or updater. SMTP authentication from the real Mac/account is not verified: the connected device is Offline and a Google App Password is still required. No message was sent from the supplied account during this task
- Existing PostgreSQL, Docker Desktop and real inbox limitations from prior sections still apply. Updater defaults preserve config, bookings, queue codes, tokens and SMTP credentials; fixture checks use mocked Docker, not the user's running services

## Queue-only local test; mail paused — 14 October 2026

- Application queue logic/Frontend unchanged from the verified remaining-count release (46 consult cases and browser transitions). Added only a dedicated launcher and Compose override plus distribution/docs.
- Queue test updater/launcher shell syntax passes. Mock Docker fixture verifies repeated update/start, scoped stop/status/logs, fixed base+mail-disabled override on every Compose call, preserved env/SMTP/catalog/bookings sentinel and backups. No SMTP wizard, test-email, reset or volume removal.
- SMTP_HOST/SMTP_FROM are explicitly empty in the local queue-test overlay even if previous SMTP credentials exist. Credentials are preserved for later setup; the worker is disabled by the existing backend configuration rule.
- Native Docker Desktop and real PostgreSQL are still not executed here; the user starts the provided updater on the Mac. Existing Go/UI runtime validation is reused because application code did not change.

## Completed archive / Staff test-data management — 14 October 2026

- All 52 consult regression cases passed with Go race detector on fresh PGlite databases, including six new cleanup cases: archive/unarchive preserves Completed stages, capacity, queue code and tracking; completed permanent test deletion; mode/event/mail guards; active SMTP lease guard; stale revision checks; bulk clear keeps other event data, seven Staff users and login sessions; queue sequence restarts only for the cleared event; HTTP authorization/CSRF/origin guards.
- Go build/vet and cmd/server race tests passed. TypeScript/Vite production build and eight Node configuration/catalog tests passed. Existing migration contents remain unchanged; new archive fields use the existing Booking JSON.
- Chromium against the actual Go API and PGlite exercised completion, archive/unarchive, parent tracking retained, checkbox-confirmed single deletion, invalidation of deleted tracking tokens, typed whole-event clear, rejection when a booking arrives during confirmation, retained Staff login/accounts and a fresh B-M-001 after clear. Desktop and 390px mobile confirmation layouts were inspected.
- Browser also restarted Go in development mode with a non-sample real-event config: completed archive and parent tracking worked; test deletion controls were absent and a direct authenticated DELETE returned403. This checks the non-local boundary, not a deployed HTTPS production service.
- Preserving updater/launcher fixtures use mock Docker. Updates do not delete existing booking data and retain the mail-disabled queue-test profile. Permanent cleanup requires a separate explicit action inside Staff.
- No Docker daemon/native PostgreSQL is available here. PGlite validation is not native PostgreSQL lock/load certification or a run of Docker Desktop on the user's Mac. Email remains paused; deployment and real inbox delivery are outside this release.

## Staff action button sizing — 15 October 2026

- Removed the obsolete global danger-button 24px width/height that clipped the text action; text actions now size to their labels and do not shrink in their wrapping action group. Font and padding continue to follow neighboring action buttons.
- TypeScript/Vite production build passed. Chromium against the actual Go API/PGlite verified full labels and equal heights/fonts for edit, cancel and delete: 33px height on desktop, 36px on the 390px check-in layout; desktop queue checked too. Both cancel and test-delete buttons still open their confirmation dialogs; no records were deleted by this UI check. Desktop/mobile row screenshots inspected; no mobile page overflow or browser page errors.
- CSS-only application change; booking/queue/storage behavior is unchanged.

## Recover original tracking link from Staff — 15 October 2026

- Two new Go integration cases passed with race detector on PGlite: original link remains identical through booking/cancellation/restore/completion/archive, including non-local/non-sample capability; lookup does not change booking, status, queue, capacity or revisions; other-event/deleted records return404. HTTP tests cover operator access, anonymous/expired-session denial, forged Origin denial, no-store response, one-field projection and generic errors for corrupt stored ciphertext.
- Four related regression cases passed with race detector: HTTP authorization/privacy, persistence/token encryption, completed archive and completed test deletion. Full previous 52-case regression was not rerun for this isolated addition.
- Go build/vet and cmd/server race tests passed; TypeScript/Vite production build passed. Chromium with normal isolated browser contexts (without the single-process launch flag) verified the exact original URL, actual clipboard write/read, opening parent tracking, desktop/mobile modal layout, selection/manual-copy fallback after clipboard rejection, API failure/retry and switching records without retaining the previous URL. Screenshots inspected; no mobile page overflow or page errors.
- Existing token is decrypted only on an authenticated Staff detail request, checked against its stored hash, never rotated or added to the Staff-list/public projections. No email is sent and no application schema migration is needed. Existing native PostgreSQL/Docker Desktop/external email validation limits still apply.

## Production Staff and HTTPS mail — 28 October 2026

- Production roster and Resend HTTPS implementation verified in the prior release; no live inbox delivery or Railway deployment claimed. Database backup/recovery features from that release were removed on29October2026.

## Adaptive frontend polling — 28 October 2026

- Booking/edit availability30s, success stops, parent10s/near2queues5s, final/inactive60s, Staff/display5s. Hidden/offlinepause; stalevisibilityresume; completion-based timers; in-flight dedupe and one mutationfollowup; retry30/60/120s. No database/schema/email changes.
- Seven deterministic fake-clock scheduler cases passed, including slowflight/mutationrace/cleanup/errors/resume/terminalrestore. Node scripts all16passed. TypeScript/Vite buildpassed. No paid usage or live concurrency/cost projection measured.
- Railway project checked: all3services andbucket remain staged; no current deployment or production polling observation.
- Browser test on built frontend with fixture API passed: booking30s and success-stop, parent10s→near5s, hidden pause/stale resume coalescing, Staff/display5s, no runtime errors. This verifies UI scheduling, not live Railway cost/load.

## Remove database backup/recovery — 29 October 2026

Removed backup/restore scripts, images, Compose drill, recovery workflow and generator keys. Production generator creates web settings, seven Staff credentials and private README only. Application booking restore/archive features remain unchanged. Consult Backup staged creation cancelled; bucket object deleted (no provisioned instances); deleted bucket reference still present in pending Railway patch and must be discarded before deployment. Main Postgres service and persistent volume retained. No deployment performed.

## Uniform five-second frontend refresh — 30 October 2026

User requested reverting the cost-saving per-page cadence. Booking/edit availability, booking-success mounted reads, parent tracking (including Completed/Cancelled/No-show and all positions), Staff and display now use the same5000ms policy. No-token tracking page does not request a booking. Hidden/offline suspension, non-overlapping requests, mutation invalidation and error retry remain. No database, booking, queue or email behavior changes. Railway bucket removed from staged changes; main services remain staged.
- Validation: all15Node tests passed; TypeScript/Vite production build passed. Browser against fixture API verified booking/success, far/near parent queue, Completed/Cancelled/No-show, Staff and public display at5s, with hidden pause and coalesced resume retained. No real Railway deployment/load or remote GitHub update claimed.


## Consultation schedule and email wiring — 30 October 2026

- Current sample and production calendars now have six subject/date scopes, 14 periods and 76 slots: High Level6November, Beginner7November; identical subject times with independent quotas. Math13:00–14:00/4,17:00–18:00/4,18:00–20:00/8; Verbal17:00–18:00/4,18:00–20:00/7; Combined14:00–15:30/5,15:30–17:00/6. Seminars7November: Math11:30–13:00, Verbal13:00–14:00. High Verbal7November removed.
- The booking label and accessibility group now say เลือกช่วงเวลา Private Consult. Existing five-second polling and live Staff credentials remain unchanged. No application migration or data reset added.
- TypeScript/Vite production build and15Node tests passed. Go server build and cmd/server race tests passed. Eighteen targeted consult integration cases (20including SMTP TLS subtests) passed on fresh isolated PGlite databases with the race detector: exact sample/production calendars, all capacities/full limits, concurrent bookings across two API instances for4/5/6/7/8places, invalid dates and retired slots, local reset guards, Staff roster/auth/HTTP audit, queue remaining, archive, and email retry/cancellation.
- Added registration-to-HTTPS-email integration: disabled provider creates no outbox work; enabled provider queues confirmation, delivers through an isolated TLS fixture with the correct recipient/private tracking link/new seminar and consult times, marks Sent, and does not redeliver completed work. Existing Resend API/template/idempotency/failure tests passed. No real provider or inbox delivery claimed.
- Railway production inspection found no existing bookings before preparing this update; latest deployed application logs say Email delivery not configured, and service variable names contain no EMAIL_FROM or RESEND_API_KEY. The new schedule source still requires a GitHub upload and successful Railway deployment. Existing actual database/service is not reset or changed by local verification.
- Chromium against the built frontend, actual Go API and isolated PGlite passed all six scopes/14periods/76slots, seminar labels, sole High Verbal6November date card, API rejection of7November, successful registration with explicit mail-off message/private link, Staff editing to the new Verbal period while retaining tracking, and390px mobile layout. No page errors or horizontal overflow.
- Full Go package unit suite passed with race detector after updating cross-period queue/display fixtures to the current calendar. Database integration cases in that command are skipped without TEST_DATABASE_URL; the18targeted integration cases above were executed separately on PGlite.


## Date-specific consultation times and uniform date selector — 30 October 2026

- 6November High Level restored: Math10:00–11:30/7,11:55–13:25/7; Verbal10:00–11:30/7,11:55–13:40/8; Combined14:30–16:10/7,16:10–18:10/8. 7November Beginner retains the newly requested times and capacities; seminars remain Math11:30–13:00 and Verbal13:00–14:00 on7November. Six scopes,13periods,82slots total.
- High Level Verbal now uses the same native date selector and mode indicator as Math/Combined, with only6November available. No special date-card branch remains. Production/sample schedules match. New source still requires uploading to GitHub before Railway can use it.
- Corrected calendar validation passed: TypeScript/Vite build, Go server build, Go unit packages with race detector, and five targeted PostgreSQL-compatible integration cases (all-period quotas/invalid dates, concurrent4–8slot quotas, reset reseeding, and registration-to-HTTPS-mail fixture) on PGlite. Browser assertions passed all six scopes/13periods/82slots, uniform date controls, mobile layout and Staff editing; no browser page errors. Native Railway deployment remains pending source upload.


## Registration copy, seminar emphasis and email activation readiness — 30 October 2026

- Hero now displays Student Performance Report followed by the exact Thai Cycle August - November2026 heading, with matching32px black bold text on desktop (26px on390px screens). Ticket art flows below the title on mobile without overlap.
- Added the exact one-family/10-minute consultation notice below the period heading, connected to the choice group with aria-describedby. Beginner seminar information is larger and bold, with individual highlighted date/time cards for Math, Verbal or both subjects; checked/unchecked feedback remains visible.
- TypeScript/Vite production build passed. Chromium with actual Go API and PGlite verified exact heading text/equal size/color, note text/accessibility, larger bold seminar times, all six scopes/13periods/82slots, uniform date selectors, booking/Staff edit/tracking, desktop/390px layouts and checkbox. Screenshots inspected; no page errors or horizontal overflow.
- Existing confirmation template/MIME, ResendHTTPS idempotency/failure tests and registration-to-HTTPS-confirmation integration passed with race detector using isolated fixtures. Preview is generated from the same Go renderer. No actual email sent to an external recipient.
- Railway latest deployment77382a3f-62ee-48a8-b018-985f9ce595bd was SUCCESS before this UI source update. Variable names still contain no EMAIL_FROM/RESEND_API_KEY; user confirmed no ready Resend account/domain. Sending remains off until the account, verified sender domain and API key are supplied. EMAIL_SETUP_TH.md documents the exact activation steps; no incomplete production variable change was applied.


## Google Apps Script MailApp delivery — 30 October 2026

- Added EMAIL_PROVIDER=apps-script with strict canonical HTTPS /exec URL and an independent64-character hex shared key. Go renders the existing HTML/text template, creates a content-specific stable request ID and HMAC-SHA256 envelope with a short-lived timestamp, then verifies the relay's request-ID-bound sent/duplicate acknowledgement. Google ContentService response redirects allow only GET to script.googleusercontent.com/macros/echo; sign-in/unrelated/body-replaying redirects fail safely.
- Included standalone Code.gs and a minimal script.send_mail manifest. The relay validates signature/time/recipient/body, checks MailApp's remaining recipient quota, locks concurrent sends and retains bounded7-day accepted-message receipts. It does not expose the key, recipient or private link in replies/logs. Rare acceptance-before-receipt crashes are still at-least-once, documented in setup instructions.
- All19Node tests passed, including4executing the actual Code.gs under Apps Script service mocks: UTF-8 signing, retained template, duplicate suppression, malformed/expired/tampered requests, extra-recipient injection, quota/busy/send errors and bounded receipt cleanup.
- Full Go unit packages with race detector, go vet and server build passed. Three targeted PGlite integration cases passed with race detector: real registration→Apps Script adapter→quota failure→retry→Sent, existing mail retries, and cancelled-mail suppression. TLS fixtures verified exact template/private link/current seminar/consult times; ContentService-style redirect and login/error/incorrect-ack cases passed.
- Apps Script has not yet been deployed/authorized by the sending Google account; no /exec URL or real inbox-delivery result exists yet. Production sending remains off pending source upload and the deployed URL. Shared relay key is supplied separately from public source; database APP_SECRET and Staff credentials are untouched.
