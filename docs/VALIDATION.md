# Validation — PostgreSQL / OpenAI UI update — 13 October 2026

## Per-row consult timer and slot estimates — 6 November 2026

- The Staff dashboard now shows each booking's consult clock: `Calling` → `เรียก HH:mm`; `Serving` → `เริ่ม HH:mm · ใช้ไป mm:ss · คาดจบ HH:mm` with a live one-second timer anchored to the server clock (`serverTime` in the packet), switching to `เกิน mm:ss` after the configured duration; `Completed` → `HH:mm–HH:mm · N นาที`. The same cell appears in the queue, check-in and completed/cancelled tables and in the detail modal, which also lists per-room stage history. Rows without timing or estimate show `—`.
- Start/End buttons are duplicated in the check-in tab (Start only for Calling, End only for Serving) and call the same status endpoint, so the existing transition, revision, notification and `STREAM_BUSY` guards apply unchanged. A shared label helper keeps the Combined handoff wording identical to the queue tab.
- The server projects each saved slot's start time into `estimates` on `GET /api/staff/bookings`: one chain per level + date + room driven by the real `calledAt`/`startedAt`/`endedAt` stamps, so a 20-minute consult shifts every following slot, an early finish pulls the queue forward, and no estimate is earlier than the slot's own period start or "now". A still-running session keeps sliding, including across the next period of the same room; a projected start past the period end is flagged `late` (`เกินรอบ`). Rows without a saved position, Cancelled/No-show rows and rows whose period left the configuration are skipped and consume no slot; Booked rows that already hold a saved position do get an estimate.
- Admins can change the event-wide minutes per slot (1–60, default 10) from the dashboard. It is stored in the existing `settings` table (`consult_minutes`), returned in the staff packet and in `GET /api/event` (so the booking page shows the real duration), and reaches every Staff device on the next 5-second poll. No migration, no revision bump, no booking/audit/outbox change; parent tracking and the public display keep their existing payloads and the parent projection still exposes no times or ETA.
- Verified locally: `go vet ./...` clean; `go test -race -count=1 ./...` passes against native PostgreSQL 14.23 with `TEST_DATABASE_URL` (115 consult package tests + 7 command tests, including new estimate-chain and settings/HTTP cases); `npm run build` (tsc --noEmit + Vite) passes; `node --test scripts/*.test.cjs` passes 60/60 with Node 26 (6 new consult-time cases).
- A scripted local-server run on a scratch PostgreSQL database performed 35 checks through the real HTTP API: default and changed durations, `/api/event` propagation, estimates at period start with 10- and 15-minute spacing, a live session shifting the next slot to its projected end, late flags on an overrun period, stage timestamps after each action, and unchanged parent/public payloads.
- Headless Chrome against the local server verified the rendered dashboard (evidence in `docs/e2e-2026-11-06-consult-timer/`): admin settings control and modal, live elapsed/expected-end cell, waiting estimate with `เกินรอบ`, completed range and minutes, queue `เวลาคิว` column, detail-modal timing rows, booking-page duration copy and a 390 px layout with no horizontal overflow and zero page errors.
- Not verified in this pass: a Railway deployment and a physical venue dry run — those remain for the operator. No database migration, new environment variable or production data change; the JSON additions are backward compatible and the backend should be deployed before the frontend.

## Atomic booking swap and immediate parent refresh — 5 November 2026

- New authenticated Staff endpoint swaps two existing same-level/same-subject reservations in one shared-lock transaction, retaining each parent's identity, queue code, chosen Combined first room and tracking link. Clears both saved room orders; date changes reset check-in. Both participants need valid revisions, explicit pair confirmation and a reason. Called/serving/completed/cancelled/started-history and same-period swaps are rejected. Each participant has an audit entry and updated mail queued when delivery is configured.
- Full-period swapping temporarily releases exactly the two seats inside the uncommitted transaction, then validates and writes their final destinations under the existing capacity trigger. No migration or real cancellation is committed. An injected error on the second final write verifies rollback of the first write, both notifications, audit and global revision. Concurrent requests using the same revisions yield exactly one swap and one conflict.
- Parent tracking requests fresh data on mount, foreground visibility/focus, pageshow (including cached history restores) and online recovery; normal visible polling stays 5 seconds. Requests bypass HTTP cache. Request epochs prevent older in-flight responses from replacing the new foreground state; queued foreground reads coalesce without overlap, including when the older request fails.
- 23 selected PGlite/pgx PostgreSQL integration cases passed with Go race detector: six new swap cases plus the 17 recovery, reusable-code, cancellation/restoration, Combined, capacity, archive and deletion regressions. New cases cover full-capacity identity/check-in/code/link retention, unchanged third parties, notification/audit behavior, atomic rollback, revision/scope/active/notification guards, Combined first-room and cross-date state, concurrent saves, operator authorization, CSRF/origin and strict pair payload.
- Go test -race ./..., go vet ./..., server build, TypeScript check and Vite production build passed. All Node script tests passed, including 10 polling cases covering forced immediate reads inside five seconds, slow-read coalescing, failed old reads and cleanup/hidden behavior.
- Chromium on an isolated Go server: operator swapped two full one-seat fixture periods through the mobile Staff modal; codes and an unrelated booking remained intact. Parent tracking fetched immediately on entry, kept 5-second polling, refreshed on pageshow/focus/visibility within the normal poll window, displayed new Calling/Serving/Completed states, and hid a deliberately delayed stale response while the newer read was pending. Mobile dialog had no horizontal overflow; zero page errors.
- No production booking, parent record, Railway setting, mail delivery or live Sheet was changed in this work. All browser fixtures had mail/Sheets sync disabled. Deployment is via the supplied source update using the existing database and environment.

## Reusable queue codes and permanent cancelled deletion — 5 November 2026

- Queue allocation uses the lowest free code in the existing event/level/booked-subject namespace inside the shared write transaction. Occupied codes, including completed/archived/no-show rows, are not renumbered. Cancel/release/delete returns a code. Obsolete sequence high-water values do not affect new allocation. Legacy cancelled codes are released atomically before allocation with audit.
- Normal cancellation and Admin cancellation clear code/order/counter; cancelled restoration never reclaims the previous code. Admin check-in reset defaults to releasing its code in the UI, with an option to keep the code for a non-cancelled real parent. Identity/tracking links remain stable until permanent deletion.
- New Admin-only DELETE cancelled endpoint requires CSRF/origin, exact record confirmation, current event/revision, Cancelled state and notification-idle guard. It removes exactly the selected booking, audit and outbox. Tracking becomes unavailable. Active/completed records cannot be deleted through this endpoint.
- 17 selected integration tests passed with race detector on isolated PGlite via pgx/PostgreSQL wire protocol. Five new tests verify interior gaps 002/004, stale sequence 999, restored/released codes, legacy cancellation, separate prefixes, concurrent saves/conflict retry without duplicate codes, production-mode permanent deletion, unrelated parents unchanged, stale/wrong-event/Sending guards and HTTP Admin/CSRF/origin/bulk-field rejection. Updated cancellation/restoration/deletion regression cases and Combined/archive/capacity/recovery cases passed.
- Go test -race ./..., go vet ./..., server build, TypeScript check and Vite production build passed. Chromium against isolated Go server verified default release checkbox, confirmation gating, reset/cancel releases, permanent delete, fresh assignment of B-V-001/B-V-002, operator controls hidden, 390px dialogs without horizontal overflow and zero page errors.
- No production booking, Tar entry, parent data, Railway configuration or live Sheet was modified. No database migration or new variables required. Mail/Sheets disabled for test fixtures. Existing 5-second frontend polling and Sheets sync unchanged.

### Earlier validation history

The sections below record prior versions; the 5 November behavior above supersedes earlier notes about retaining cancelled codes or never reusing deleted numbers.

## Selective booking recovery — 4 November 2026

- Added Admin-only per-booking recovery with CSRF/origin protection, exact booking confirmation, mandatory reason, event/revision checks and full previous-state audit. No database reset, migration or global sequence changes.
- Four new integration tests passed with Go race detector against isolated PGlite through PostgreSQL wire protocol: cancel completed/archived test booking returns exactly one slot; other parents unchanged; unsent mail suppressed; cancelled entry excluded from Excel/Sheets; real B-V-003 reset preserves original tracking URL/code and sent mail, with new bookings continuing at B-V-004; Combined first-subject selection and capacity check; invalid/stale inputs, wrong event, notification Sending guard; anonymous/operator/CSRF/origin and unknown bulk-field rejection.
- Go test -race ./..., go vet ./... and server build passed. TypeScript check and Vite production build passed (existing third-party use-client bundling warnings).
- Chromium against isolated Go server verified Admin modal reset and cancellation, preserved queue code and unrelated record, reason/confirmation gating, 390px mobile dialog without horizontal overflow, hidden controls for operator and zero page errors.
- Browser and integration fixtures used isolated temporary databases with mail and Google Sheets sync disabled. No live registration, Tar record, parent status, production database or Railway configuration was changed.

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


## Staff export and student corrections — 4 November 2026

- Authenticated GET /api/staff/export.xlsx exports a consistent event-scoped snapshot of visible records only: cancelled and archived bookings are excluded from both registration rows and period counts. Completed records that have not been archived remain exportable. No nickname/Wcode blacklist or production deletion is used.
- XLSX has 39 registration columns and a 14-column period summary. Thai, leading-zero Wcodes, formula-like question text, timestamps, numeric summary, frozen headers and filters are verified using openpyxl.
- Added PATCH /api/staff/bookings/{id}/student-details with authentication, CSRF, strict field validation, event isolation, stale revision protection, notification-send guard and staff audit. Wcode, nickname, school and email can be corrected throughout the lifecycle without changing queue/order/status/history/tracking or requeuing an already-sent confirmation. Shared normalization preserves the existing registration validation.
- Go test -race ./..., go vet ./... and server build passed. General tests skip DB cases without TEST_DATABASE_URL; four targeted tests ran separately against isolated PGlite databases with -race: HTTPAuthorizationAndPrivacy, HTTPStaffExportAllRecordsAndAuthorization, StudentDetailsPreserveQueueAcrossLifecycle and HTTPStudentDetailsAuthorizationAndExport. Tested profile corrections before/after check-in, during a call/consultation, after completion/archive, and on cancelled/retired-period records. Invalid fields, forged status, stale revision, forged origin, missing auth/CSRF and concurrent email sending are rejected.
- Production frontend build passed. Browser checks exercise Staff profile edits and a real authenticated XLSX download with filters active, verify a cancelled Tar fixture is excluded, and cover busy state, retryable error, expired login and the 390px mobile header. No production bookings or email sends are created during validation.
- Upload the update to GitHub and wait for a successful Railway deployment to activate the feature. No database migration, variable or Apps Script change is required.


## Google Sheets background synchronization — 4 November 2026

- Optional production-only background worker uses the same registrationSheets builder as Excel (39 columns and identical cancelled/archived exclusions). New records are sent in CreatedAt order; the receiver matches existing rows by booking ID and appends only new IDs. Removals clear A:AM in place without shifting outside columns; historical blank rows are not reused.
- Worker reads a fresh PostgreSQL repeatable-read snapshot, sends on changed revision, reconciles unchanged data every 5 minutes, retries failures with backoff up to 5 minutes and reconciles immediately after process restart. No new DB schema, booking write hooks, Google credentials in the browser, or changes to frontend polling/email delivery.
- Separate Apps Script receiver targets only the supplied spreadsheet and gid 1971459091. Validates HMAC/timestamp, schema, unique booking IDs, event binding and monotonic revision. Blocks non-empty unclaimed tables, changed headers, formulas and data conflicts. Literal rich-text writes preserve Wcode zeros and formula-looking questions. Formats only managed A:AM on first initialization.
- Node VM tests execute the actual Sheets Code.gs and pass append/update/retry ordering, cleared/tail rows, preserved AN cells, stale requests, event mismatch, committed-write/lost-response recovery, unchanged-revision reconciliation, empty-tab expansion, HMAC validation and target safety cases. Existing mail/polling/preparation Node tests pass alongside them (16 total).
- Go test -race ./..., go vet ./... and server build pass. PGlite integration with -race passes latest-snapshot retry/coalescing/restart, column parity, corrected data, removal, and literal control-text sanitization; existing export authentication and student-correction lifecycle integration tests also pass. TLS transport tests verify signed UTF-8 payloads, acknowledgments, stale responses, wrong IDs/revisions, extra JSON, login redirects and cancellation.
- Live Google Drive/Sheets connector reads returned Unknown tool and the supplied page could not be read with web retrieval. No existing target cells were read, overwritten, or visually verified, no production sync variables were enabled, and no live registration data was sent. User must run checkSheetSetup with their Google account, deploy the separate Web app and supply its /exec URL before Railway can be connected.


## Named Google Sheets target — 4 November 2026

- Apps Script selects SPR_Queue Sheets Sync by getSheetByName in the existing spreadsheet, and builds its ownership marker from the actual sheet ID. Setup logs the resolved name/gid before the non-empty check, so destination is visible even when content needs inspection.
- Existing signature, schema, row matching, stale revision and non-empty target checks are preserved. A missing named tab reports TARGET_TAB_NOT_FOUND rather than writing to another tab. No live spreadsheet edits or sync activation were performed.
- Actual Code.gs VM tests verify selection when the actual gid differs from the former fixed gid, and missing-name rejection, alongside the existing append/update/clear/retry/authentication cases. Backend and frontend source unchanged.

## Parent tracking link resilience and public queue board — 5 November 2026

- The emailed tracking link now opens a parent-only surface. `#/track/<token>` renders without the operator sidebar, `#/display` renders as a board/kiosk surface, and `#/book` and `#/staff` keep the existing portal shell. A tab that arrived through a private link keeps booking/staff pages behind a neutral "หน้านี้ไม่พร้อมใช้งาน" card; authorization remains server-side (Staff session + CSRF).
- Rendering no longer blocks on `GET /api/event`. The last good event configuration is cached, so a cold reload of the tracking link can paint the parent ticket while the API is unreachable. `#/track` and `#/display` render without the event API; portal pages still wait for it.
- Each successful tracking read is cached per token (`spr.v1.snap.*`, fetchedAt included). A failed poll no longer replaces the ticket with an error card: the last snapshot stays visible with explicit freshness bands — fresh (≤15s), delayed (>15s), offline (navigator offline), ancient (>2h, "ข้อมูลนี้เก่าเกินไป"). Terminal booking states are never escalated. A `404` clears the cached snapshot for that token and shows an explicit invalid-link card, so a deleted/test-cleared booking cannot survive as a ghost ticket.
- Automatic recovery keeps the existing poller contract: completion-scheduled 5s reads, no overlapping requests, pause when hidden/offline, immediate forced read on `online`, `focus`, `pageshow` and `visibilitychange`. No WebSocket/SSE was added; the 5s cadence is already far tighter than the human-paced queue.
- A production-only service worker (`/sw.js`) caches the app shell: navigation is network-first with cached-shell fallback, `/assets/*` hashed files are cache-first, and `/api/*` is never intercepted or cached. After one successful online visit the tracking URL opens with no network. A new deployment is picked up on the next online load; no DB migration, API shape change or new dependency is involved.
- The public board supports `#/display?level=..&date=..&kiosk=1`: level/date are URL-pinned and remembered, controls hide in kiosk mode, a Wake Lock is requested while visible, and the 6 November High Level/Zoom day shows an Online notice instead of room instructions. The board keeps the last snapshot with a timestamp and offline banner rather than blanking.
- Verified locally: `node --test scripts/*.test.cjs` (34 existing + new resilience tests) pass; `go vet ./...` and `go test -race -count=1 ./...` pass (DB integration cases skip without TEST_DATABASE_URL); `npm run build` (tsc --noEmit + vite build) passes; the built `index.html`, hashed assets and `/sw.js` were served from `frontend/dist` over HTTP and inspected. Not verified in this pass: a real browser cold-open-offline test, a Railway deployment, and a physical venue TV/phone dry run — those remain for the operator before the 6-7 November slots.

## Staff-only portal shell and staff entry — 5 November 2026

- The portal sidebar is now a staff affordance: `shellFor()` renders it only when a staff session is known. Anonymous visitors get public surfaces without portal chrome — `#/track` (parent), `#/display` (board), `#/book` (public booking) — while `#/staff` and the bare site URL render a clean login page. `#/display?...&kiosk=1` stays a board for any viewer.
- The bare URL `/` (and `#`, `#/`) normalizes to `#/staff` through `history.replaceState`, so the browser Back button does not bounce through the login page; a signed-in staff member lands on the dashboard.
- Staff identity is an optimistic cache (`spr.v1.staff`, username/role only) plus an authoritative `GET /api/auth/me` probe. The probe never blocks a public render: a `401` clears the cache and downgrades the shell, while a network/5xx failure keeps the last known state because it is not proof of logout. Logout clears the cache, returns to the login page and broadcasts through `localStorage`, so other tabs drop the sidebar immediately.
- Private-link tabs are guarded only while anonymous. Staff are never blocked, and the guard now offers an escape to the staff login that clears the tab's parent marker, so a signed-out operator cannot get stuck on the guard card.
- Staff opening a tracking link keep the sidebar and see a `STAFF PREVIEW · มุมมองผู้ปกครอง` chip; the page exposes the same parent fields only, with no staff-only data or mutations.
- No backend, schema or API change. Authorization remains the existing session cookie plus CSRF on staff endpoints; the shell decision is UX only and cannot grant access. The probe is one indexed session lookup (`Authenticate`), has no rate limit, returns no data on `401`, and is never cached (`/api/*` is `no-store` and the service worker skips `/api/*`).
- Verified: `npm run build` (tsc --noEmit + vite) passes; `node --test scripts/*.test.cjs` 48/48 including the new shell matrix (anonymous/staff/kiosk), base-entry detection, guard/staff-bypass and staff-cache cases; `go vet ./...` and `go test -race -count=1 ./...` pass with no backend source change. Browser click-through (login → dashboard → track/display with sidebar → logout in a second tab) was not executed in this pass and remains an operator check.

## Staff login input overlap — 5 November 2026

- Defect: on the new staff login page the username and password boxes rendered as two overlapping rectangles. The SDK `Input` renders a wrapper `div` plus a raw `input`; the site's global input rule gives that raw input its own border, padding and `margin-top:8px`, and the existing reset that neutralises it was scoped to `.field-grid` / `.tracking-entry` only. The new `.staff-login` card was not covered, so the inner input overflowed its wrapper and collided with the next field.
- Fix: the shared input reset and field-box rules now also apply to `.staff-login label>div` and `.staff-login label>div input`. No component or API change.
- Evidence: production-style build served locally and rendered in headless Chrome at 916x1156; the two fields render as separate 46px boxes with the wrapper outline and no overlap. Regression guard added in `scripts/ui-styles.test.cjs` (asserts the input reset and field-box rules cover `.staff-login`). Full suite `node --test scripts/*.test.cjs` passes 50/50 and `npm run build` passes.

## Queue board links back to tracking — 5 November 2026

- The normal (non-kiosk) queue board header now shows a `ติดตามคิวของคุณ` button next to `รีเฟรช`, linking back to `#/track`. The redundant per-control refresh button was removed so the board has a single refresh action.
- Kiosk mode (`#/display?...&kiosk=1`) deliberately keeps no back button and no other chrome: a wall screen is not a navigation surface. Verified by DOM dump (no `ติดตามคิวของคุณ` text in kiosk mode).
- Verified in headless Chrome: desktop 1280x900 renders the button in the header; a true 390px viewport (measured inside a 390px iframe) reports `scrollWidth == clientWidth == 390` with the header wrapping correctly, so no horizontal overflow. Earlier 390px window screenshots were clipped by Chrome's minimum headless window width, not by page layout. Full node suite 50/50 and `npm run build` pass.

## Board back-link returns to the parent's own queue — 5 November 2026

- The tracking token is now remembered per device (`spr.v1.track.token`, accepted only at 32-100 characters, matching the server's token bounds) whenever a tracking link is opened. The board's `ติดตามคิวของคุณ` button links to `#/track/<remembered-token>` and falls back to the paste-link entry only when no token is known.
- The token is deliberately never appended to the board URL: a board URL gets copied, projected and screenshotted, and the tracking token must not ride along. The private token stays in localStorage on the parent's own device, next to the snapshot cache that already used it as a key.
- A `404` from `/api/track` clears the remembered token (and the snapshot), so a deleted link cannot leave the board pointing at a dead queue; the next board render degrades to the entry page.
- Integration verified against a mock API in headless Chrome with a persistent profile: opening the parent link then the board yields `href="#/track/<token>"`; a fresh profile yields `href="#/track"`; opening a dead link shows the invalid-link card and the following board render falls back to `#/track`. Unit suite 54/54 and the production build pass.

## Staff operations recheck fixes — 6 November 2026

- Fixed No-show keeping a saved queue position: booking status now clears `orders`/`counter` on No-show, on No-show recovery (including legacy rows), and the queue must be saved again before the parent can be called. Prevents two rows advertising the same position and the parent count disagreeing with the board.
- Cancelling a booking now cancels its unsent confirmation mail in the same transaction, marks EmailStatus `Cancelled`, and is rejected with `NOTIFICATION_BUSY` while the outbox worker is sending. The worker fallback for an already-cancelled booking also reports `Cancelled` instead of `Failed`.
- Restore accepts optional `date` + `periodId` for the same level/subject, so a cancelled Math + Verbal booking that already completed its first room can be restored into another period when the original was retired or refilled. Stage history, tracking token and parent identity are preserved; capacity is rechecked and the queue is rebuilt.
- Admin cancellation of a `Serving` booking is a single confirmed step: no extra acknowledgement checkbox and no typed reason; the server records the fixed audit reason “ยุติการปรึกษาระหว่างให้บริการ”. The dialog warns that the room frees immediately and that a rebooked seat cannot be restored.
- `ASSIGN_QUEUE` audit rows now contain queueNo/orders/counter before and after, and legacy cancelled codes are released only when the exact code is reused instead of rewriting every cancelled row on each save.
- Local-test roster now matches the documentation: RD1 is Admin (recovery/delete tools), RD2–RD3 and SM1–SM4 are Operators.
- Verified at commit state of this work: `go vet ./...` clean; `go test -race -count=1 ./...` passes against native PostgreSQL 14.23 with `TEST_DATABASE_URL` (97 top-level consult tests + 7 command tests; new coverage includes no-show duplicate positions, cancel-vs-Sending, suppressed mail status, restore to new/retired/full period, last-slot concurrency, Serving acknowledgement, audit positions, scoped legacy-code release and HTTP strict decode); `npm run build` (tsc --noEmit + Vite) passes; `node --test scripts/*.test.cjs` passes 54/54 with Node 26 (Node 22.14 in this environment cannot import the `.ts` polling module, so use Node 26+). No database migration, new environment variable or production data change; new API fields are optional and the backend must be deployed before the frontend.
- Not verified in this pass: PostgreSQL 17 / Railway deployment, browser click-through of the new restore picker and Serving checkbox, and real mail delivery. These remain operator checks before relying on the change during the 6–7 November event.
