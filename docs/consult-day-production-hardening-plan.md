# Implementation plan: reliable multi-user Consult Day

## 0. Context and current behavior

Goal: staff can operate concurrently, understand whether their screen is current, and recover safely from lost connections or ambiguous action responses.

Verified repository evidence:
- `frontend/src/polling.ts:createPoller`: one in-flight read, five seconds after read completion, hidden/offline suspension, retries at 30/60/120 seconds. Invalidations queue a read but do not invalidate publication of the preceding result.
- `frontend/src/hooks.ts:useLive`: publishes results and successful-fetch timestamps; epoch protection is currently for forced resume only. Cached writes occur before active/epoch checks. Refresh resolves even on fetch failure because the poller catches errors.
- `frontend/src/App.tsx:StaffPage/run`: disables writes while busy/loading or after a read error. No offline or snapshot-age gate. Refresh after mutation is attempted; completion of refresh does not prove successful reconciliation. Other mutation entry points include saveSwap, saveCancelledDelete, saveRecovery, saveStudentDetails, saveBookingEdit and saveRestore; inventory these before implementation.
- `frontend/src/api.ts:request`: 15-second client timeout; status and queue mutations carry expected revisions. Transport errors do not distinguish an uncertain write outcome from a failed read.
- `backend/internal/consult/database.go:beginWrite/snapshot`: database write guard, consistent REPEATABLE READ booking/revision snapshot.
- `backend/internal/consult/service.go:Status/Assign`: booking revision checks and event revision checks, transactional audit and commit; repeated Waiting on Waiting is harmless.
- `backend/internal/consult/migrations/001_initial.sql:active_stream`: unique active consultation per event/level/date/subject.
- `backend/internal/consult/http.go`: authentication, authorization, CSRF, request IDs, request duration/status logs and DB-backed health check already exist.

Unknown: deployed revision, peak screen count, measured request latency, production conflict frequency. These are not prerequisites to fixing client recovery.

Scope: multi-user queue freshness and staff action recovery. No queue protocol redesign, database backup changes, room ownership feature, or new service.

## 1. Requirements

| ID | Observable requirement | Acceptance |
| --- | --- | --- |
| FR1 P0 | Staff sees last successful sync, offline state and stale state | Last-sync time advances only on an accepted successful read; stale warning appears without requiring another response |
| FR2 P0 | Staff cannot submit operational changes from an unusable screen | Offline, failed fetch, loading, stale snapshot or unresolved write outcome disables mutation controls and is also checked at the submission boundary |
| FR3 P0 | A read superseded by a local mutation cannot republish old data as current | Controlled pre-mutation response is discarded; exactly one coalesced follow-up fetch obtains authoritative state |
| FR4 P0 | A timed-out write has an explicit uncertain outcome | Staff sees “result not confirmed”; no automatic replay; successful authoritative read permits review before another action |
| FR5 P0 | Concurrent actions retain database protection | Same-booking stale write rejected; same-room different-student calls yield one winner; different rooms can progress together |
| FR6 P1 | Reconnect recovers promptly | One forced/coalesced read when staff/display returns online or visible; no overlapping reads or focus-event request storm |

Proposed freshness cutoff: reuse `frontend/src/freshness.ts:FRESH_MS` (15 seconds). Validate on the actual event network; this is a proposed operational threshold, not a measured production SLO. Keep visible healthy polling at five seconds after completion. Keep staff PII out of persistent browser snapshots.

## 2. Core entities and invariants

- PostgreSQL booking state is authoritative; booking revision protects individual changes, event revision protects queue-order saves.
- INV1: one Calling/Serving student per existing active_stream key; Math and Verbal remain independent.
- INV2: stale writes never overwrite a newer booking or queue snapshot.
- INV3: superseded or disposed reads never update visible data, cache, or last-success time.
- INV4: a network failure never triggers automatic replay of a staff command.
- INV5: a successful command acknowledgement and successful reconciliation are distinct states; a failed reconciliation must not invite repeating an acknowledged command.

## 3. API / interface changes

Keep HTTP endpoints, authorization and revision payloads compatible. Retain revision validation at the server even when the client disables buttons.

Extend `useLive` with an explicit successful-read outcome and a way to require a post-invalidation read. Preserve convenient fire-and-forget refresh usage; automatic poll failures must not become unhandled rejections. Keep internal cancellation/supersession distinct from actual transport failure.

At `api.ts:request`, retain typed server errors and correlation IDs. Classify write transport/abort failures as outcome-unknown. Do not label every HTTP error “not saved”: a server failure around commit can also require reconciliation. No mutation auto-retries or operation ledger in this phase; existing expected revisions remain the write safeguard. Reconciliation shows current state, not proof of which actor performed a transition.

## 4. Critical data flow

Click -> shared staff submission guard -> capture ID/action/expected revision -> existing authenticated API -> database guard/revision validation/transaction -> command acknowledgement -> invalidate older read generation -> authoritative fetch -> publish snapshot and sync time -> release reconciliation gate.

If the write response is lost: record local uncertain outcome -> fetch current state -> present current state for review. If that fetch also fails: preserve uncertainty and keep mutation submission disabled; allow manual refresh. After browser reload, require a successful initial fetch before enabling writes.

## 5. Recommended design

1. Fix shared read lifecycle in `polling.ts` and `hooks.ts`: invalidate publication immediately on local mutation, queue a fresh fetch, and report whether that follow-up succeeded. Guard cache writes with the same lifetime/generation check as UI publication. Ensure a superseded result does not count as last successful sync.
2. Reuse `freshness.ts`, `useOnline` and `useNow` in StaffPage. Show Thai last-sync and connection messages through existing Notice/status styles. Centralize eligibility at the staff mutation boundary used by all save handlers; controls consume the same eligibility. Keep read/search/export behavior available where appropriate.
3. Separate command pending, acknowledged-awaiting-sync and outcome-unknown states. A command that saved but failed to refresh shows “saved; syncing failed,” rather than a generic invitation to repeat. Conflict feedback says data changed and requires review. Preserve revision-bound modal drafts or require reopening after conflict; never silently update a draft revision and resubmit old intent.
4. Enable forced/coalesced resume for staff and public display after verifying the shared hook behavior. Never restore staff snapshots from local storage.

Alternatives: server push would reduce notification delay but still needs all recovery safeguards above; defer until a measured sub-five-second need. Manual-refresh-only reduces reads but imposes avoidable operator work. Existing polling is the minimal complete choice.

Capacity model: normal read rate is approximately V / (5 seconds + mean fetch duration), where V is visible online screens. Illustrative only: 50 screens at negligible fetch duration produce about 10 reads/second. Measure snapshot latency, response size, DB pool wait and write-lock wait before changing polling frequency or the database guard.

## 6. Failure deep dives

| Failure | Protection / owner | Verification | Residual risk |
| --- | --- | --- | --- |
| Old fetch resolves after local mutation | Generation rejection in shared read lifecycle | Deferred response released after mutation; no old publication/cache write | Another user's update remains bounded by polling |
| Write commits but response is lost | Uncertain outcome UI; no replay; successful authoritative fetch before enabling actions | Drop response after actual DB commit | Latest state cannot always establish actor attribution |
| Write acknowledged but reconciliation fails | Separate saved/sync-failed state; keep mutation gate | Success response followed by failed GET | Operator waits for connectivity |
| Offline or hung request | Age/connectivity gate at UI and submission boundary | Fake clock exceeds cutoff before error arrives | Server remains necessary against forged/stale clients |
| Concurrent calls / stale saves | Existing transactions, revisions and unique index | Real PostgreSQL concurrent tests, including separate service connections | Event-wide order revision can reject unrelated room edits |
| Session expiry | Existing auth expiry handling; no replay on re-login | Expire session during poll and mutation | Operator must authenticate again |

## 7. Ordered implementation phases

| Phase | Files | Specific work | Done when |
| --- | --- | --- | --- |
| 1 | scripts/polling.test.cjs; frontend/src/polling.ts; frontend/src/hooks.ts | Write controlled response-order tests, implement generation-aware publication and explicit reconciliation outcome | FR3 / INV3 tests pass; all existing poll tests remain green |
| 2 | frontend/src/App.tsx; frontend/src/api.ts; frontend/src/freshness.ts; existing UI styles if needed | Shared staff submission eligibility and action recovery state; typed uncertainty; accessible Thai sync messages | FR1/2/4 exercised across normal actions and every modal save handler |
| 3 | frontend/src/pages/DisplayPage.tsx; frontend/src/App.tsx | Coalesced resume/reconnect using corrected hook | FR6 passes, hidden/offline still suspends |
| 4 | scripts/polling.test.cjs; scripts/resilience.test.cjs; backend/internal/consult/service_test.go; backend/internal/consult/parallel_rooms_test.go; browser test location chosen from existing harness | Full verification below; record actual results and remaining gaps | No DB test skips reported as passes; two-browser scenarios recorded |

## 8. Verification matrix

| Case | Stimulus | Expected | Level / coverage |
| --- | --- | --- | --- |
| T1 | Hold read A, commit mutation, invalidate, release A then read B | A never advances data/cache/sync time; B publishes | Controlled async hook integration; FR3 / INV3 |
| T2 | Unmount or change scope while old read is pending | No disposed publication or cache write | Hook integration; INV3 |
| T3 | Fetch hangs; advance clock beyond cutoff; attempt UI and direct save handler | Visible stale warning; no write request | Component/fake-clock; FR1/2 |
| T4 | Server commits action, drop acknowledgement, then GET succeeds | Unknown-outcome message; no repeat POST; current state shown for review | Browser + real API/DB; FR4 / INV4 |
| T5 | Mutation returns success but GET fails | Saved-but-not-synced state; further writes blocked until successful fetch | Component/API integration; INV5 |
| T6 | Two browsers edit same booking from same revision | One succeeds; stale request conflicts; review required | PostgreSQL + browser; FR5 / INV2 |
| T7 | Concurrent calls for two students in same room | One active student; loser gets conflict/busy | PostgreSQL; INV1 |
| T8 | Simultaneous Math and Verbal progression | Both active; finishing one does not alter the other | PostgreSQL; FR5 / INV1 |
| T9 | Concurrent queue saves from one event revision | One succeeds; rejected draft is not automatically replayed | PostgreSQL + browser; INV2/4 |
| T10 | Hide/offline, restore, emit repeated focus/online events | One coalesced fresh fetch, no overlap; stale controls recover only after success | Poller/hook/browser; FR2/6 |
| T11 | Expire session; resume; re-login | Login handling; no unauthorized submission or command replay | Browser/API; FR2 / INV4 |

Run `node --test scripts/polling.test.cjs scripts/resilience.test.cjs`, frontend `npm run build`, and backend `go test -race -count=1 ./...` with a dedicated TEST_DATABASE_URL. Add actual component/browser coverage for hook and UI behavior; poller-only tests do not prove React publication correctness.

## 9. Rollout and recovery

No database migration or new infrastructure required. Deploy compatible client changes against current API after local integration/browser verification. Canary with two staff devices and a display, then check all event devices have the intended frontend version. Observe sync age, recovery time, 409 counts and request latency using existing logs plus minimal added categories if needed; never log booking bodies, emails, tracking tokens or credentials.

Proposed abort criteria: stale data marked fresh, commands replayed automatically, cross-room interference, or controls remaining blocked after successful reconciliation. Revert the frontend bundle if necessary; database protections remain. Client rollback cannot undo committed staff actions; inspect current state/audit before correction.

## 10. Open decisions and final gap review

Defaults: retain five-second polling, reuse proposed 15-second freshness cutoff, disable writes while uncertain, no automatic replay, no WebSockets, no lock partitioning. Peak device count and network latency inform load verification, not correctness rules. Separate scope-specific queue versions only if measured conflict frequency justifies the migration.

Traceability: FR1/2 -> phase 2 -> T3/T10/T11; FR3 -> phase 1 -> T1/T2; FR4 -> phase 2 -> T4/T5; FR5 -> phase 4 -> T6/T7/T8/T9; FR6 -> phase 3 -> T10. INV1-5 each have explicit tests above.

Plan status: source inspected; implementation and listed new tests are not yet performed. Preserve pre-existing untracked parallel_rooms_test.go and parallel-consult-audit-2026-11-06.md.
