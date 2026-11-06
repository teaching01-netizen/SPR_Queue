package consult

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// T1: a no-show must not keep a saved room position; the queue has to be saved
// again before the recovered row can be called, and positions stay unique.
func TestNoShowClearsPositionAndRecoveryForcesResave(t *testing.T) {
	s := testService(t)
	i := testInput()
	ids := []string{}
	for n := 0; n < 3; n++ {
		ids = append(ids, book(t, s, i).Booking.ID)
	}
	assign(t, s, defaultScope(), ids...)
	for _, id := range ids {
		transition(t, s, id, "Waiting")
	}
	noShow, err := s.Status(testCtx, ids[1], "No-show", latest(t, s, ids[1]).Revision, "staff", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(noShow.Orders) != 0 || noShow.Counter != "" || noShow.QueueNo == nil {
		t.Fatalf("no-show kept queue state: %+v", noShow)
	}
	assign(t, s, defaultScope(), ids[0], ids[2])
	if orders := map[int]bool{}; true {
		for _, id := range []string{ids[0], ids[2]} {
			o := latest(t, s, id).Orders["Math"]
			if o == 0 || orders[o] {
				t.Fatalf("duplicate position after no-show: %v", o)
			}
			orders[o] = true
		}
	}
	recovered, err := s.Status(testCtx, ids[1], "Waiting", latest(t, s, ids[1]).Revision, "staff", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(recovered.Orders) != 0 {
		t.Fatalf("recovered no-show kept old position: %+v", recovered.Orders)
	}
	assign(t, s, defaultScope(), ids[0], ids[1], ids[2])
	seen := map[int]bool{}
	for _, id := range ids {
		o := latest(t, s, id).Orders["Math"]
		if o == 0 || seen[o] {
			t.Fatalf("duplicate position after resave: %v", o)
		}
		seen[o] = true
	}
}

// T2: rows created before this fix can still be in No-show with an old order.
// Recovering them must clear that order too.
func TestNoShowRecoveryClearsLegacySavedPosition(t *testing.T) {
	s := testService(t)
	c := book(t, s, testInput())
	assign(t, s, defaultScope(), c.Booking.ID)
	tx, err := s.beginWrite(testCtx)
	if err != nil {
		t.Fatal(err)
	}
	b, err := getBooking(testCtx, tx, c.Booking.ID)
	if err != nil {
		t.Fatal(err)
	}
	b.Status = "No-show"
	b.Orders = map[string]int{"Math": 2}
	if err = updateBooking(testCtx, tx, &b); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	out, err := s.Status(testCtx, c.Booking.ID, "Waiting", latest(t, s, c.Booking.ID).Revision, "staff", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Orders) != 0 || out.Counter != "" {
		t.Fatalf("legacy position survived recovery: %+v", out)
	}
}

// T3: cancelling while the outbox worker is sending must be rejected, not
// silently race a confirmation email.
func TestCancelRejectedWhileConfirmationMailInFlight(t *testing.T) {
	s := testService(t)
	s.MailEnabled = true
	c := book(t, s, testInput())
	b := latest(t, s, c.Booking.ID)
	if _, err := s.DB.ExecContext(testCtx, `UPDATE outbox SET state='Sending',lease_at=$2 WHERE booking_id=$1`, b.ID, time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Status(testCtx, b.ID, "Cancelled", b.Revision, "staff", "race"); !codeIs(err, "NOTIFICATION_BUSY") {
		t.Fatal(err)
	}
	after := latest(t, s, b.ID)
	if after.Status != "Booked" || after.Revision != b.Revision {
		t.Fatalf("booking changed while mail in flight: %+v", after)
	}
	var state string
	if err := s.DB.QueryRowContext(testCtx, `SELECT state FROM outbox WHERE booking_id=$1`, b.ID).Scan(&state); err != nil || state != "Sending" {
		t.Fatalf("outbox changed: %s %v", state, err)
	}
}

// T4: a pending confirmation is cancelled with the booking and is never sent.
// Already delivered mail keeps its Sent state.
func TestCancelSuppressesPendingConfirmationMail(t *testing.T) {
	s := testService(t)
	s.MailEnabled = true
	pending := book(t, s, testInput())
	delivered := book(t, s, testInput())
	tx, err := s.beginWrite(testCtx)
	if err != nil {
		t.Fatal(err)
	}
	deliveredBooking, err := getBooking(testCtx, tx, delivered.Booking.ID)
	if err != nil {
		t.Fatal(err)
	}
	deliveredBooking.EmailStatus = "Sent"
	if err = updateBooking(testCtx, tx, &deliveredBooking); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.ExecContext(testCtx, `UPDATE outbox SET state='Sent' WHERE booking_id=$1`, delivered.Booking.ID); err != nil {
		t.Fatal(err)
	}

	b := latest(t, s, pending.Booking.ID)
	out, err := s.Status(testCtx, b.ID, "Cancelled", b.Revision, "staff", "ผู้ปกครองยกเลิก")
	if err != nil {
		t.Fatal(err)
	}
	if out.EmailStatus != "Cancelled" {
		t.Fatalf("email status not cancelled: %q", out.EmailStatus)
	}
	var state string
	if err := s.DB.QueryRowContext(testCtx, `SELECT state FROM outbox WHERE booking_id=$1`, b.ID).Scan(&state); err != nil || state != "Cancelled" {
		t.Fatalf("pending outbox not cancelled: %s %v", state, err)
	}
	sent := latest(t, s, delivered.Booking.ID)
	if _, err := s.Status(testCtx, sent.ID, "Cancelled", sent.Revision, "staff", "ผู้ปกครองยกเลิก"); err != nil {
		t.Fatal(err)
	}
	m := &fakeMailer{}
	if _, err := s.ProcessMail(testCtx, m); err != nil || m.sent != 0 {
		t.Fatalf("cancelled confirmation was sent: sent=%d err=%v", m.sent, err)
	}
	if got := latest(t, s, delivered.Booking.ID); got.EmailStatus != "Sent" || got.Status != "Cancelled" {
		t.Fatalf("delivered mail state changed: %+v", got)
	}
}

// T5: the worker's fallback path for a booking cancelled before it could send
// must report Cancelled, not Failed.
func TestWorkerReportsSuppressedCancelledMail(t *testing.T) {
	s := testService(t)
	s.MailEnabled = true
	c := book(t, s, testInput())
	tx, err := s.beginWrite(testCtx)
	if err != nil {
		t.Fatal(err)
	}
	b, err := getBooking(testCtx, tx, c.Booking.ID)
	if err != nil {
		t.Fatal(err)
	}
	b.Status = "Cancelled"
	b.CancelledAt = stamp()
	if err = updateBooking(testCtx, tx, &b); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	m := &fakeMailer{}
	done, err := s.ProcessMail(testCtx, m)
	if err != nil || !done || m.sent != 0 {
		t.Fatalf("worker sent cancelled mail: done=%v sent=%d err=%v", done, m.sent, err)
	}
	got := latest(t, s, c.Booking.ID)
	if got.EmailStatus != "Cancelled" {
		t.Fatalf("suppressed mail reported as %q", got.EmailStatus)
	}
	var state string
	if err := s.DB.QueryRowContext(testCtx, `SELECT state FROM outbox WHERE booking_id=$1`, got.ID).Scan(&state); err != nil || state != "Cancelled" {
		t.Fatalf("outbox not cancelled: %s %v", state, err)
	}
}

// T6: a partially served Math + Verbal booking cancelled after the first room
// can be restored into a different period when the original period was retired.
func TestRestoreWithTargetMovesPartiallyServedCombined(t *testing.T) {
	s := testService(t)
	s.MailEnabled = true
	i := testInput()
	i.Subject = "Combined"
	i.PeriodID = "P1300"
	c := book(t, s, i)
	scope := Scope{i.Level, i.Date, i.PeriodID, "Math"}
	assign(t, s, scope, c.Booking.ID)
	b := transition(t, s, c.Booking.ID, "Waiting")
	b = transition(t, s, b.ID, "Calling")
	b = transition(t, s, b.ID, "Serving")
	b = transition(t, s, b.ID, "Completed")
	if b.CurrentSubject != "Verbal" || b.Status != "Waiting" || b.Stages["Math"].EndedAt == "" {
		t.Fatalf("fixture did not finish first room: %+v", b)
	}
	cancelled, err := s.Status(testCtx, b.ID, "Cancelled", b.Revision, "staff", "ผู้ปกครองขอเลื่อน")
	if err != nil {
		t.Fatal(err)
	}
	// Retire the original period from the live config, as an operator would.
	for n := range s.Config.Rules {
		r := &s.Config.Rules[n]
		if r.Level == i.Level && r.Subject == "Combined" && r.Date == i.Date {
			kept := r.Periods[:0]
			for _, p := range r.Periods {
				if p.ID != "P1300" {
					kept = append(kept, p)
				}
			}
			r.Periods = kept
		}
	}
	if _, err := s.RestoreBooking(testCtx, cancelled.ID, BookingRestore{Revision: cancelled.Revision}, "staff"); !codeIs(err, "INVALID_PERIOD") {
		t.Fatal("retired period restore was allowed", err)
	}
	restored, err := s.RestoreBooking(testCtx, cancelled.ID, BookingRestore{Revision: cancelled.Revision, Date: i.Date, PeriodID: "P1400"}, "staff")
	if err != nil {
		t.Fatal(err)
	}
	if restored.PeriodID != "P1400" || restored.Status != "Waiting" || restored.CurrentSubject != "Verbal" {
		t.Fatalf("wrong restored state: %+v", restored)
	}
	if restored.Stages["Math"].EndedAt == "" || restored.Stages["Verbal"].StartedAt != "" {
		t.Fatalf("stage history lost: %+v", restored.Stages)
	}
	if restored.QueueNo != nil || len(restored.Orders) != 0 || restored.EmailStatus != "Pending" {
		t.Fatalf("queue state not rebuilt: %+v", restored)
	}
	if _, err := s.Track(testCtx, c.Token); err != nil {
		t.Fatal("tracking identity lost", err)
	}
}

// T7: when the original period is full, a target period lets staff restore the
// booking instead of leaving it permanently cancelled.
func TestRestoreTargetWhenOriginalPeriodIsFull(t *testing.T) {
	s := testService(t)
	i := testInput()
	i.Subject = "Combined"
	i.PeriodID = "P1300"
	c := book(t, s, i)
	b := latest(t, s, c.Booking.ID)
	cancelled, err := s.Status(testCtx, b.ID, "Cancelled", b.Revision, "staff", "test")
	if err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 4; n++ {
		book(t, s, i)
	}
	if _, err := s.RestoreBooking(testCtx, cancelled.ID, BookingRestore{Revision: cancelled.Revision}, "staff"); !codeIs(err, "PERIOD_FULL") {
		t.Fatal("full period restore was allowed", err)
	}
	restored, err := s.RestoreBooking(testCtx, cancelled.ID, BookingRestore{Revision: cancelled.Revision, Date: i.Date, PeriodID: "P1400"}, "staff")
	if err != nil || restored.PeriodID != "P1400" {
		t.Fatalf("target period restore failed: %+v %v", restored, err)
	}
}

// T8: target validation is strict and keeps the original record on failure.
func TestRestoreTargetValidation(t *testing.T) {
	s := testService(t)
	c := book(t, s, testInput())
	b := latest(t, s, c.Booking.ID)
	cancelled, err := s.Status(testCtx, b.ID, "Cancelled", b.Revision, "staff", "test")
	if err != nil {
		t.Fatal(err)
	}
	for _, in := range []BookingRestore{
		{Revision: cancelled.Revision, Date: b.Date},
		{Revision: cancelled.Revision, PeriodID: "P1630_1800"},
		{Revision: cancelled.Revision, Date: b.Date, PeriodID: "NOPE"},
		{Revision: cancelled.Revision, Date: b.Date, PeriodID: "P1400"},
	} {
		if _, err := s.RestoreBooking(testCtx, cancelled.ID, in, "staff"); err == nil {
			t.Fatalf("invalid target accepted: %+v", in)
		}
	}
	if got := latest(t, s, cancelled.ID); got.Status != "Cancelled" || got.PeriodID != b.PeriodID {
		t.Fatalf("failed restore changed record: %+v", got)
	}
	if _, err := s.RestoreBooking(testCtx, cancelled.ID, BookingRestore{Revision: cancelled.Revision - 1}, "staff"); !codeIs(err, "REVISION_CONFLICT") {
		t.Fatal("stale revision accepted", err)
	}
}

// T9: two restores competing for the last target slot cannot both succeed.
func TestRestoreTargetLastSlotIsSerialized(t *testing.T) {
	s := testService(t)
	i := testInput()
	i.Subject = "Combined"
	i.PeriodID = "P1300"
	type target struct {
		id string
		in BookingRestore
	}
	targets := []target{}
	for n := 0; n < 2; n++ {
		c := book(t, s, i)
		b := latest(t, s, c.Booking.ID)
		cancelled, err := s.Status(testCtx, b.ID, "Cancelled", b.Revision, "staff", "test")
		if err != nil {
			t.Fatal(err)
		}
		targets = append(targets, target{cancelled.ID, BookingRestore{Revision: cancelled.Revision, Date: i.Date, PeriodID: "P1400"}})
	}
	for n := range s.Config.Rules {
		r := &s.Config.Rules[n]
		if r.Level != i.Level || r.Subject != "Combined" || r.Date != i.Date {
			continue
		}
		for p := range r.Periods {
			if r.Periods[p].ID == "P1400" {
				r.Periods[p].Capacity = 1
			}
		}
	}
	var wg sync.WaitGroup
	errs := make([]error, len(targets))
	for n := range targets {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_, errs[n] = s.RestoreBooking(testCtx, targets[n].id, targets[n].in, "staff")
		}(n)
	}
	wg.Wait()
	success, full := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			success++
		case codeIs(err, "PERIOD_FULL"):
			full++
		default:
			t.Fatal(err)
		}
	}
	if success != 1 || full != 1 {
		t.Fatalf("last slot not serialized: success=%d full=%d", success, full)
	}
}

// T10: an Admin can cancel a Serving room in one confirmed step, without a
// separate acknowledgement checkbox or a typed reason. The server keeps a
// fixed audit reason; other cancellations still require a reason.
func TestRecoveryServingCancelNeedsNoReason(t *testing.T) {
	s := testService(t)
	c := book(t, s, testInput())
	assign(t, s, defaultScope(), c.Booking.ID)
	b := transition(t, s, c.Booking.ID, "Waiting")
	b = transition(t, s, b.ID, "Calling")
	b = transition(t, s, b.ID, "Serving")
	in := recoveryInput(b, "cancel")
	in.Reason = ""
	out, err := s.RecoverBooking(testCtx, b.ID, in, "RD1")
	if err != nil || out.Status != "Cancelled" {
		t.Fatalf("serving cancel without reason failed: %+v %v", out, err)
	}
	if out.CancellationReason != "ยุติการปรึกษาระหว่างให้บริการ" {
		t.Fatalf("audit reason not defaulted: %q", out.CancellationReason)
	}
	// A non-Serving cancellation still requires a reason.
	other := book(t, s, testInput())
	if other.Booking.ID == "" {
		t.Fatal("second booking missing")
	}
	in = recoveryInput(latest(t, s, other.Booking.ID), "cancel")
	in.Reason = ""
	if _, err = s.RecoverBooking(testCtx, other.Booking.ID, in, "RD1"); !codeIs(err, "VALIDATION_ERROR") {
		t.Fatal("reasonless non-serving cancel accepted", err)
	}
}

// T12: queue saves record real positions in the audit trail.
func TestAssignAuditRecordsPositions(t *testing.T) {
	s := testService(t)
	ids := []string{book(t, s, testInput()).Booking.ID, book(t, s, testInput()).Booking.ID}
	assign(t, s, defaultScope(), ids...)
	audit, err := s.Audit(testCtx)
	if err != nil {
		t.Fatal(err)
	}
	type position struct {
		Status  string         `json:"status"`
		QueueNo *string        `json:"queueNo"`
		Orders  map[string]int `json:"orders"`
		Counter string         `json:"counter"`
	}
	seen := map[int]bool{}
	for _, a := range audit {
		if a.Action != "ASSIGN_QUEUE" {
			continue
		}
		var previous, next position
		if err := json.Unmarshal([]byte(a.Previous), &previous); err != nil {
			t.Fatalf("previous audit: %v %q", err, a.Previous)
		}
		if err := json.Unmarshal([]byte(a.Next), &next); err != nil {
			t.Fatalf("next audit: %v %q", err, a.Next)
		}
		seen[next.Orders["Math"]] = true
	}
	if !seen[1] || !seen[2] {
		t.Fatalf("audit positions missing: %v", seen)
	}
}

// T13: reusing a legacy cancelled code must not rewrite other cancelled rows.
func TestLegacyCodeReleaseIsScopedToReusedCode(t *testing.T) {
	s := testService(t)
	i := testInput()
	c1 := book(t, s, i)
	c2 := book(t, s, i)
	assign(t, s, defaultScope(), c1.Booking.ID, c2.Booking.ID)
	if *latest(t, s, c1.Booking.ID).QueueNo != "B-M-001" || *latest(t, s, c2.Booking.ID).QueueNo != "B-M-002" {
		t.Fatal("fixture codes")
	}
	legacy := []string{c1.Booking.ID, c2.Booking.ID}
	for _, id := range legacy {
		tx, err := s.beginWrite(testCtx)
		if err != nil {
			t.Fatal(err)
		}
		b, err := getBooking(testCtx, tx, id)
		if err != nil {
			t.Fatal(err)
		}
		b.Status = "Cancelled"
		b.CancelledAt = stamp()
		if err = updateBooking(testCtx, tx, &b); err != nil {
			t.Fatal(err)
		}
		if err = tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	untouched := latest(t, s, legacy[1])
	fresh := book(t, s, i)
	assign(t, s, defaultScope(), fresh.Booking.ID)
	if got := latest(t, s, fresh.Booking.ID); got.QueueNo == nil || *got.QueueNo != "B-M-001" {
		t.Fatalf("legacy code not reused: %+v", got)
	}
	if got := latest(t, s, legacy[0]); got.QueueNo != nil {
		t.Fatalf("reused row still holds code: %+v", got)
	}
	if got := latest(t, s, legacy[1]); !reflect.DeepEqual(untouched, got) {
		t.Fatalf("unrelated cancelled row changed: %+v -> %+v", untouched, got)
	}
}

// The HTTP restore endpoint accepts the optional target fields and still
// rejects unknown fields (strict JSON decoding).
func TestHTTPRestoreAcceptsOptionalTarget(t *testing.T) {
	s := cleanupService(t)
	c := book(t, s, cleanupInput())
	cancelled, err := s.Status(testCtx, c.Booking.ID, "Cancelled", c.Booking.Revision, "RD1", "ทดลอง")
	if err != nil {
		t.Fatal(err)
	}
	a, err := NewAuth(s, false)
	if err != nil {
		t.Fatal(err)
	}
	a.Cost = bcrypt.MinCost
	if err = a.SyncLocalStaff(testCtx); err != nil {
		t.Fatal(err)
	}
	admin, err := a.Login(testCtx, "RD1", "RD1")
	if err != nil {
		t.Fatal(err)
	}
	if err = a.StoreSession(testCtx, admin, ""); err != nil {
		t.Fatal(err)
	}
	h := NewHTTP(s, a, []string{"http://localhost:8080"}, "").Handler()
	post := func(payload map[string]any) *httptest.ResponseRecorder {
		body, _ := json.Marshal(payload)
		r := httptest.NewRequest("POST", "/api/staff/bookings/"+cancelled.ID+"/restore", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-CSRF-Token", a.csrf(admin.Token))
		r.Header.Set("Origin", "http://localhost:8080")
		r.AddCookie(&http.Cookie{Name: "warwick_session", Value: admin.Token})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	w := post(map[string]any{"expectedRevision": cancelled.Revision, "date": cancelled.Date, "periodId": cancelled.PeriodID})
	if w.Code != 200 {
		t.Fatalf("restore target HTTP %d %s", w.Code, w.Body.String())
	}
	var out Booking
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || out.Status != "Booked" {
		t.Fatalf("restore response: %+v %v", out, err)
	}
	if w := post(map[string]any{"expectedRevision": out.Revision, "date": out.Date, "periodId": out.PeriodID, "extra": true}); w.Code != 400 {
		t.Fatalf("unknown field accepted: %d %s", w.Code, w.Body.String())
	}
}
