# Go API — endpoints ที่ implement แล้ว

Go Server เป็นผู้ตัดสินกฎจริง Frontend ไม่กำหนด capacity, queue number, actor หรือ timestamps เอง

## Routes

| Method | Path | Access |
| --- | --- | --- |
| GET | `/api/health` | Public health |
| GET | `/api/event` | Public event config |
| GET | `/api/periods?level=...&subject=...&date=...` | Public availability; ไม่คืนจำนวนคนจอง |
| POST | `/api/bookings` | Public booking + Idempotency-Key |
| GET | `/api/track/:token` | Participant token |
| GET | `/api/display?level=...&date=...` | Public current queue codes/state/counter |
| POST | `/api/auth/login` | Staff credentials |
| GET | `/api/auth/me` | Staff cookie; คืน user + CSRF token |
| POST | `/api/auth/logout` | Staff cookie + CSRF |
| GET | `/api/staff/bookings` | Staff; event records + global revision + consult timing projection |
| PUT | `/api/staff/settings/consult-minutes` | Admin cookie + CSRF; minutes per consult slot (1–60) |
| PUT | `/api/staff/queue-order` | Staff cookie + CSRF + expectedRevision |
| POST | `/api/staff/bookings/:id/actions` | Staff cookie + CSRF + booking expectedRevision |
| POST | `/api/staff/bookings/:id/reschedule` | Admin cookie + CSRF + booking expectedRevision |
| GET | `/api/staff/audit` | Admin; latest 200 audit entries for event |

Staff filter ใน UI ทำจาก authorized event list รุ่นนี้ ยังไม่มี server pagination/streaming events

## Booking

ส่ง `Content-Type: application/json`, `Idempotency-Key: <unique-client-request-id>`

```json
{
  "email": "parent@example.com",
  "wcode": "W1234",
  "nickname": "Korn",
  "school": "Example School",
  "level": "Beginner",
  "subject": "Combined",
  "date": "2026-08-15",
  "periodId": "P1300",
  "questions": ["Question one", "Question two", ""],
  "seminarConfirmed": true
}
```

ผลลัพธ์: `{ booking, token, trackingUrl }` Token คืนเฉพาะ confirmation ของคำขอนั้น รวม retry ที่ใช้ key/payload เดิม Staff list ไม่คืน token

Unknown fields, unsupported subject/level/date/period และ payload เกิน body limit จะถูกปฏิเสธ

## Staff queue order

ใช้ `X-CSRF-Token` ที่ได้รับจาก login/me โดย session cookie เป็น HttpOnly

```json
{
  "scope": { "level": "Beginner", "date": "2026-08-15", "periodId": "P1300", "stream": "Math" },
  "bookingIds": ["immutable-id-1", "immutable-id-2"],
  "expectedRevision": 7
}
```

ต้องส่งรายการ Booked/Waiting ของ scope ให้ครบ ไม่ซ้ำและไม่มี ID ต่าง scope ค่า revision มาจาก `/api/staff/bookings` ถ้าเปลี่ยนแล้วรับ 409 ต้อง reload และตรวจลำดับใหม่

Revision เป็น event-store revision แบบ conservative: mutation หรือ email status อื่นอาจทำให้ draft เก่าถูกปฏิเสธด้วย ระบบไม่ rebase draft ให้อัตโนมัติ

## Staff status action

```json
{ "status": "Calling", "expectedRevision": 3, "reason": "" }
```

| Current | Allowed next |
| --- | --- |
| Booked | Waiting, Cancelled |
| Waiting | Calling, No-show, Cancelled |
| Calling | Serving, Waiting, No-show, Cancelled |
| Serving | Completed |
| No-show | Waiting ผ่าน Staff recovery action |
| Cancelled | คืนการจองผ่าน `POST /api/staff/bookings/:id/restore` |
| Completed | ปิดรายการ ไม่มี restore endpoint |

Combined Math + Completed จะเป็น Verbal/Waiting แทน global Completed; เริ่ม Calling Verbal ต้องจัด Verbal order ก่อน

`reason` บังคับเมื่อ Cancelled Actor ได้จาก session identity ไม่รับ `staffName` จาก payload

## Staff consult timing and slot estimates

`GET /api/staff/bookings` adds three display-only fields on top of the existing snapshot. None of them are stored, and none appear on parent or public surfaces:

| Field | Meaning |
| --- | --- |
| `consultMinutes` | Minutes per consult slot for this event (default 10, range 1–60) |
| `serverTime` | Server clock when the snapshot was built (RFC3339Nano UTC); the dashboard anchors live elapsed timers to it so a skewed staff device still shows true durations |
| `estimates` | `{ "<bookingId>": { "startAt": "<RFC3339Nano UTC>", "late": true? } }`; a missing id means no estimate for that row |

`startAt` is the projected consult start of a saved queue position. Each room (level + date + Math/Verbal) is one chain: a slot starts when the previous slot starts plus `consultMinutes`. Real `calledAt`/`startedAt`/`endedAt` move the chain, so a slot that runs 20 minutes instead of 10 shifts every following slot and finishing early pulls the queue forward. A slot never starts before its own period start or before "now"; a session that is still running keeps sliding with the clock, including across the next period start of the same room. A projected start past its period end is flagged `late`. Rows without a saved position, Cancelled/No-show rows, rows whose period left the event configuration, and rows whose current room already ended are skipped and consume no slot; Booked rows that already hold a saved position do receive an estimate. `GET /api/track/:token` and `GET /api/display` are unchanged.

`GET /api/event` adds the same `consultMinutes` value so the booking page can show the real consultation duration. The rest of the event payload is unchanged.

`PUT /api/staff/settings/consult-minutes` (Admin cookie + CSRF, strict JSON) takes `{ "minutes": 1..60 }` and returns `{ "consultMinutes": n }`. It writes the existing `settings` row; there is no migration, no booking/audit/outbox change and no revision bump, so open queue drafts stay valid. Every Staff device sees the new duration on its next 5-second poll; the booking page reads it when the page loads. Invalid or out-of-range values return 400, non-Admin 403, unauthenticated 401.

## Reschedule

```json
{ "date": "2026-08-15", "periodId": "P1400", "expectedRevision": 1 }
```

Admin เท่านั้น และรายการต้องเป็น Booked หากรอบใหม่เต็ม การจองเก่าจะไม่เปลี่ยน Booking ID/queue code คงเดิม แต่ลำดับรอถูกล้างเพื่อจัดในรอบใหม่ หากกำลังส่งอีเมลให้รอจน delivery attempt จบ

## Restore cancelled booking

```json
{ "expectedRevision": 7, "date": "2026-11-07", "periodId": "P1400" }
```

`date` และ `periodId` เป็น optional: ไม่ส่งทั้งคู่ = คืนรอบเดิม; ส่งทั้งคู่ = ย้ายไปรอบใหม่ของระดับและวิชาเดิม (ห้ามเปลี่ยนวิชา/ระดับ) ระบบตรวจ config/capacity และล้างรหัสคิว/ลำดับเดิม แล้วตั้งสถานะ Waiting หากเคยเช็กอิน หรือ Booked หากยังไม่เช็กอิน เก็บประวัติ stage ที่จบแล้วและ token เดิม ใช้ได้กับรายการ Cancelled ที่ยังไม่ Completed; ผิดรูปแบบได้ 400, revision เก่า/รอบเต็ม/อีเมลกำลังส่งได้ 409

## Admin recovery

```json
{ "action": "cancel", "reason": "…", "expectedRevision": 7, "confirmation": "RECOVER BOOKING <id>" }
```

`action` เป็น `cancel` หรือ `reset-checkin` เท่านั้น ฝั่ง cancel ของรายการ `Serving` ไม่ต้องส่ง `reason` (ระบบบันทึกเหตุผลกลาง “ยุติการปรึกษาระหว่างให้บริการ”) และไม่ต้องมี field ยืนยันเพิ่มเติม นอกเหนือจาก `confirmation` ของรายการ ส่วน cancel สถานะอื่นและ reset-checkin ยังต้องมี `reason` การยกเลิกจะยกเลิกอีเมลที่ยังไม่ส่งให้ด้วย และปฏิเสธด้วย 409 `NOTIFICATION_BUSY` หากกำลังส่งอีเมลอยู่

## Parent projection

```json
{
  "queueNo": "B-MV-001",
  "currentSubject": "Math",
  "status": "Waiting",
  "counter": "",
  "currentQueues": [{ "queueNo": "B-M-002", "state": "Serving" }]
}
```

ไม่มีชื่อ โรงเรียน email Wcode questions Booking IDs ของคนอื่น หรือเวลา/ETA Tokens ที่ไม่ถูกต้องรับ 404

## Errors and safety

```json
{ "error": { "code": "PERIOD_FULL", "message": "ช่วงเวลานี้เต็มแล้ว กรุณาเลือกช่วงอื่น", "correlationId": "request-id" } }
```

400 validation/config, 401 unauthenticated, 403 role/CSRF/origin, 404 booking/token, 409 capacity/revision/state, 429 rate limit, 500 generic internal failure

API responses เป็น `Cache-Control: no-store` ส่วน Cookie Secure เปิดเมื่อ production/HTTPS SameSite=Strict ทุก mode

Server logs ใช้ route pattern เช่น `/api/track/{token}` ไม่บันทึก token จริงหรือ request body จากผู้ปกครอง

Capacity และ sequence บันทึกใน PostgreSQL transaction ที่ล็อก guard row ก่อนเขียน มี database constraints เป็น backstop ไม่ใช้จำนวนแถวเป็น queue sequence

## Display upcoming queues

GET /api/display keeps Math/Verbal arrays for active queues, and adds MathNext/VerbalNext arrays (0–2 entries) with queueNo, state=Waiting, periodStart and periodEnd. Only checked-in students with saved order in the current room are included; with an active student, restrict to that period. No names/contact/questions/tokens are exposed. The preview follows Staff order and may change.

## Personal queue count

GET /api/track/{token} adds queuesUntilTurn (integer or null) and queueCountState (ready, current, inactive, awaiting-check-in, awaiting-order, staff-review). For a ready student the integer counts future calls through their own turn, so next=1 and current=0. Counts use saved order in the remaining periods of the current room/level/date, anchored at the active or last called period. Codes are not subtracted. Only a number and status are exposed, not the bookings used to calculate it.

## Completed archive and local test cleanup

StaffPacket includes canManageTestData from server configuration; Booking may include archivedAt/archivedBy. POST /api/staff/bookings/{id}/archive takes archived + expectedRevision, preserves Completed/capacity/history/tracking and audits the authenticated actor.

DELETE /api/staff/bookings/{id} requires expectedRevision + confirmation=DELETE TEST BOOKING. POST /api/staff/test-data/clear requires expectedRevision of the whole snapshot + confirmation=CLEAR TEST DATA. Both require authenticated CSRF-protected requests and a server-granted local sample test database capability with mail disabled. Single deletion preserves other sequences; whole-event clear restarts only its sequence prefix and preserves users/sessions/settings/catalog/other events. Active notification leases block cleanup. These are not production hard-delete endpoints.

## Recover an existing tracking link (Staff)

`GET /api/staff/bookings/{id}/tracking-link` requires a valid Staff session and an allowed Origin when supplied. Returns only `{ "trackingUrl": "<PUBLIC_URL>/#/track/<original-token>" }` with `Cache-Control: no-store`. Returns404 for missing, deleted or other-event bookings;401 for unauthenticated/expired sessions. Decrypts and verifies the stored token on demand. No token rotation, email, booking/status/queue/revision changes; the general Staff list and public projections remain unchanged. Supported for active, cancelled and completed/archived records.
