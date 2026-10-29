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
| GET | `/api/staff/bookings` | Staff; event records + global revision |
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
| Completed / Cancelled | ปิดรายการ ไม่มี restore endpoint รุ่นนี้ |

Combined Math + Completed จะเป็น Verbal/Waiting แทน global Completed; เริ่ม Calling Verbal ต้องจัด Verbal order ก่อน

`reason` บังคับเมื่อ Cancelled Actor ได้จาก session identity ไม่รับ `staffName` จาก payload

## Reschedule

```json
{ "date": "2026-08-15", "periodId": "P1400", "expectedRevision": 1 }
```

Admin เท่านั้น และรายการต้องเป็น Booked หากรอบใหม่เต็ม การจองเก่าจะไม่เปลี่ยน Booking ID/queue code คงเดิม แต่ลำดับรอถูกล้างเพื่อจัดในรอบใหม่ หากกำลังส่งอีเมลให้รอจน delivery attempt จบ

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
