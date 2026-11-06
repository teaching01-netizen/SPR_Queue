# Parallel Math / Verbal consultation audit — 6 November 2026

## 1. Executive Assessment

Overall business-logic confidence: high for two independent subject rooms in the current local code. Critical/high/medium findings: 0 in this scope. No confirmed parallel-room defect. This is not a whole-system audit or deployed-service verification.

Most important invariant: one Calling/Serving booking per event, level, date and subject room, while Math and Verbal may both be active. Most fragile workflow: a Combined student's handoff requires a new saved position in the second room. Specification gaps: no blocker for the requested two-room operation; more than one teacher per subject would require a different room identity.

## 2. Domain Model

Event → booking rules (level, subject, date, period, capacity) → bookings. Each booking has one current subject and overall status, with separate Math/Verbal stage timestamps and queue orders. A Combined booking visits both rooms sequentially; separate students can occupy different rooms concurrently. Staff commands update bookings and audit records atomically. Parent tracking and public display project the current subject's room; staff timing estimates use separate room chains.

Production configuration includes simultaneous Math and Verbal periods: High Level on 6 November at 10:00 and 11:55; Beginner on 7 November at 17:00 and 18:00. Combined periods also support different students starting in opposite rooms.

## 3. Critical Invariants

| ID | Invariant | Enforcement | Status |
| -- | --------- | ----------- | ------ |
| INV-001 | Math activity must not block Verbal activity | `Status` scopes busy checks by current subject; PostgreSQL `active_stream` index includes current subject; parallel integration test | PROTECTED |
| INV-002 | At most one active booking per room | Busy validation, serialized transaction, partial unique database index; existing concurrent same-room test | PROTECTED |
| INV-003 | A Combined student occupies one room at a time | Single current-subject field; first completion hands off to Waiting; second room requires assignment | PROTECTED |
| INV-004 | Finishing Math must not mutate another student's Verbal stage | Booking-specific transactional update; new integration assertions | PROTECTED |
| INV-005 | Display/tracking must use the correct subject room | Subject-scoped projections; assertions with both rooms Calling and Serving | PROTECTED |
| INV-006 | Timing estimates must not share a Math/Verbal chain | `roomKey{level,date,stream}`; existing estimate isolation tests | PROTECTED |

## 4. State-Machine Findings

Normal route: Booked → Waiting → Calling → Serving → Completed. Calling requires check-in, queue code, saved room order and a free room. Completing the first Combined stage instead produces Waiting in the other subject, clears that subject's order, and requires staff to save its position. If the destination is occupied, calling returns `STREAM_BUSY`. Existing tests exercise either subject first and reject changing the starting subject after calling. No parallel-operation transition defect found.

## 5. Confirmed Bugs

None demonstrated in the requested workflow. No production implementation change required.

## 6. Edge Cases

| Dimension | Assessment |
| --------- | ---------- |
| Boundary / state | Separate streams can both be Calling or Serving; single current subject prevents one Combined booking occupying both rooms. |
| Concurrency | New synchronized requests pass for single-subject bookings and two Combined students starting in opposite rooms; existing same-room race accepts only one call. |
| Time | Estimates use event timezone and separate room chains; existing tests cover overruns, early finishes, exact period end and past estimates. Starting consultations is a staff command, not an automatic clock-triggered action. |
| Retry | Stale status updates return revision conflict; invalid repeated transitions are rejected. A client should refresh after a timeout before retrying. |
| Authorization | Both rooms use the same staff-authenticated status endpoint; choosing a room is queue scope, not a separate staff permission. No room-specific role requirement was supplied. |
| Data integrity | The active-room unique index protects stored state independently of UI validation. |
| Integration / partial failure | Status, stage timestamps, audit and revision updates share a transaction. These calls do not require external messaging to start a room. |
| Historical / missing data | Estimate tests cover missing order and removed periods. Arbitrarily corrupted legacy records were not reproduced in this focused audit. |

## 7. Concurrency and Idempotency

`beginWrite` locks the event metadata row until commit. Database writes serialize briefly; consultations remain active concurrently after commit. `active_stream` is unique over `(event_id,level,date,current_subject)` for Calling/Serving, so Math and Verbal use different keys. Booking revisions reject lost status updates. Queue-order saving uses a shared event revision: two staff saving from the same snapshot may cause one `REVISION_CONFLICT`, requiring refresh and another save; this is deliberate stale-write protection and does not block ongoing consultations.

## 8. Data Integrity Risks

No unprotected active-room uniqueness rule found. The schema checks booking columns against the JSON payload and enforces booking-period references. Room identity includes level and date; it supports one active teacher per subject within that scope. Multiple simultaneous teachers for the same subject are outside this model.

## 9. Cross-Feature Inconsistencies

No contradictory Math/Verbal busy rule found in staff actions, backend status changes, tracking, display or estimates. Staff chooses the active room using the room selector. The online-day public display intentionally shows Zoom guidance instead of live room numbers; parent tracking still projects queue state. Consult minutes are shared event configuration, while stage start times and running timers belong to each booking.

## 10. Missing Regression Tests

Added `backend/internal/consult/parallel_rooms_test.go`:

- Given two checked-in, ordered students in opposite rooms, when call requests and then start requests are released concurrently, then both succeed and both display/tracking projections identify the correct student and state.
- Given the same scenario with two Combined students, then opposite first rooms run concurrently.
- Given both rooms serving, when Math completes, then the Verbal student's stage and revision remain unchanged.
- Given a Combined student handed off to occupied Verbal and assigned an order, when called, then `STREAM_BUSY` preserves the destination room's exclusivity.

Verification passed against native local PostgreSQL with Go race detection (integration cases executed, not skipped):

```sh
TEST_DATABASE_URL='postgresql:///postgres?host=/tmp' go test -race -count=1 ./internal/consult -run 'Test(MathAndVerbalRunConcurrently|SingleActiveStreamConcurrent|CombinedEitherSubjectFirst|Estimates|Display)' -v
```

Run from `backend/`. Test helpers create isolated temporary schemas and remove them afterward. No live browser session or deployed backend was exercised.

## 11. Recommended Fix Order

No P0–P4 implementation fixes identified for this requirement. Retain the new regression test. Operationally, check in students, save each room's queue, and use separate staff tabs/devices scoped to Math and Verbal. Starting each room remains a manual staff action.
