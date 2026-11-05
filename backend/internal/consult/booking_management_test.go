package consult

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func editValues(b Booking) BookingEdit {
	return BookingEdit{Subject: b.Subject, Date: b.Date, PeriodID: b.PeriodID, SeminarConfirmed: b.SeminarConfirmed, Revision: b.Revision}
}
func cancelBooking(t *testing.T, s *Service, b Booking) Booking {
	t.Helper()
	out, err := s.Status(testCtx, b.ID, "Cancelled", b.Revision, "staff", "ไม่มา / ไม่แจ้งยกเลิก")
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func TestEditBookingSubjectAndIdentity(t *testing.T) {
	s := testService(t)
	c := book(t, s, testInput())
	assign(t, s, defaultScope(), c.Booking.ID)
	b := transition(t, s, c.Booking.ID, "Waiting")
	oldCode := *b.QueueNo
	oldCheck := *b.CheckedInAt
	in := editValues(b)
	in.Subject = "Verbal"
	in.PeriodID = "P1000"
	b, err := s.EditBooking(testCtx, b.ID, in, "operator")
	if err != nil {
		t.Fatal(err)
	}
	if b.ID != c.Booking.ID || b.Status != "Waiting" || b.Subject != "Verbal" || b.CurrentSubject != "Verbal" || b.QueueNo != nil || len(b.Orders) != 0 || len(b.Stages) != 0 || *b.CheckedInAt != oldCheck {
		t.Fatalf("bad edit: %+v", b)
	}
	if _, err = s.Status(testCtx, b.ID, "Calling", b.Revision, "staff", ""); !codeIs(err, "QUEUE_NOT_READY") {
		t.Fatal("called without assign", err)
	}
	scope := defaultScope()
	scope.Stream = "Verbal"
	scope.PeriodID = "P1000"
	assign(t, s, scope, b.ID)
	b = latest(t, s, b.ID)
	if !strings.HasPrefix(*b.QueueNo, "B-V-") || *b.QueueNo == oldCode {
		t.Fatal("wrong code after subject edit", b.QueueNo)
	}
	tracked, err := s.Track(testCtx, c.Token)
	if err != nil || *tracked.QueueNo != *b.QueueNo || tracked.CurrentSubject != "Verbal" {
		t.Fatal("old tracking URL", tracked, err)
	}
	in = editValues(b)
	in.Subject = "Combined"
	in.PeriodID = "P1300"
	in.StartSubject = "Verbal"
	b, err = s.EditBooking(testCtx, b.ID, in, "staff")
	if err != nil || b.Subject != "Combined" || b.CurrentSubject != "Verbal" {
		t.Fatal("combined edit", b, err)
	}
	scope.PeriodID = "P1300"
	assign(t, s, scope, b.ID)
	b = latest(t, s, b.ID)
	if !strings.HasPrefix(*b.QueueNo, "B-MV-") {
		t.Fatal(*b.QueueNo)
	}
	in = editValues(b)
	in.Subject = "Math"
	in.PeriodID = "P1800"
	b, err = s.EditBooking(testCtx, b.ID, in, "staff")
	if err != nil || b.CurrentSubject != "Math" {
		t.Fatal("solo edit", b, err)
	}
	audit, err := s.Audit(testCtx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, a := range audit {
		if a.Action == "EDIT_BOOKING" && a.Actor == "operator" && strings.Contains(a.Previous, "Math") && strings.Contains(a.Next, "Verbal") {
			found = true
		}
	}
	if !found {
		t.Fatal("edit audit missing before/after")
	}
}
func TestEditFullPeriodRollbackAndRules(t *testing.T) {
	s := testService(t)
	b := book(t, s, testInput()).Booking
	target := testInput()
	target.Subject = "Verbal"
	target.PeriodID = "P1000"
	for n := 0; n < 4; n++ {
		book(t, s, target)
	}
	in := editValues(b)
	in.Subject = "Verbal"
	in.PeriodID = "P1000"
	if _, err := s.EditBooking(testCtx, b.ID, in, "staff"); !codeIs(err, "PERIOD_FULL") {
		t.Fatal(err)
	}
	unchanged := latest(t, s, b.ID)
	if unchanged.Subject != b.Subject || unchanged.PeriodID != b.PeriodID || unchanged.Revision != b.Revision {
		t.Fatal("lost original reservation", unchanged)
	}
	in = editValues(b)
	in.Subject = "Verbal"
	in.PeriodID = "P1800"
	in.SeminarConfirmed = false
	if _, err := s.EditBooking(testCtx, b.ID, in, "staff"); !codeIs(err, "VALIDATION_ERROR") {
		t.Fatal("seminar", err)
	}
	in = editValues(b)
	in.PeriodID = "P1700"
	if _, err := s.EditBooking(testCtx, b.ID, in, "staff"); !codeIs(err, "INVALID_PERIOD") {
		t.Fatal("retired period", err)
	}
	assign(t, s, defaultScope(), b.ID)
	b = transition(t, s, b.ID, "Waiting")
	in = editValues(b)
	in.PeriodID = "P1800"
	moved, err := s.EditBooking(testCtx, b.ID, in, "staff")
	if err != nil || *moved.QueueNo != *b.QueueNo || moved.CheckedInAt == nil {
		t.Fatal("time-only edit", moved, err)
	}
	if _, err = s.EditBooking(testCtx, b.ID, in, "staff"); !codeIs(err, "REVISION_CONFLICT") {
		t.Fatal("stale edit", err)
	}
	in = editValues(moved)
	same, err := s.EditBooking(testCtx, b.ID, in, "staff")
	if err != nil || same.Revision != moved.Revision {
		t.Fatal("no-op edit", err)
	}
	scope := defaultScope()
	scope.PeriodID = "P1800"
	assign(t, s, scope, b.ID)
	b = transition(t, s, b.ID, "Calling")
	in = editValues(b)
	in.PeriodID = "P1900"
	if _, err = s.EditBooking(testCtx, b.ID, in, "staff"); !codeIs(err, "BOOKING_EDIT_LOCKED") {
		t.Fatal("editing active call", err)
	}
	b = transition(t, s, b.ID, "Waiting")
	in = editValues(b)
	in.PeriodID = "P1900"
	b, err = s.EditBooking(testCtx, b.ID, in, "staff")
	if err != nil {
		t.Fatal("editing paused call", err)
	}
	scope.PeriodID = "P1900"
	assign(t, s, scope, b.ID)
	transition(t, s, b.ID, "Calling")
	b = transition(t, s, b.ID, "Serving")
	in = editValues(b)
	in.PeriodID = "P1800"
	if _, err = s.EditBooking(testCtx, b.ID, in, "staff"); !codeIs(err, "BOOKING_EDIT_LOCKED") {
		t.Fatal("editing consultation", err)
	}
	b = transition(t, s, b.ID, "Completed")
	in = editValues(b)
	if _, err = s.EditBooking(testCtx, b.ID, in, "staff"); !codeIs(err, "BOOKING_EDIT_LOCKED") {
		t.Fatal("editing completed", err)
	}
}
func TestEditDateResetsCheckInAndMail(t *testing.T) {
	s := testService(t)
	s.MailEnabled = true
	i := testInput()
	i.Level = "High Level"
	i.Subject = "Verbal"
	i.Date = "2026-08-14"
	i.PeriodID = "P1000"
	c := book(t, s, i)
	scope := Scope{i.Level, i.Date, i.PeriodID, "Verbal"}
	assign(t, s, scope, c.Booking.ID)
	b := transition(t, s, c.Booking.ID, "Waiting")
	in := editValues(b)
	in.Date = "2026-08-15"
	s.DB.Exec(`UPDATE outbox SET state='Sending' WHERE booking_id=$1`, b.ID)
	if _, err := s.EditBooking(testCtx, b.ID, in, "staff"); !codeIs(err, "NOTIFICATION_BUSY") {
		t.Fatal("mail race", err)
	}
	s.DB.Exec(`UPDATE outbox SET state='Sent' WHERE booking_id=$1`, b.ID)
	changed, err := s.EditBooking(testCtx, b.ID, in, "staff")
	if err != nil {
		t.Fatal(err)
	}
	if changed.Status != "Booked" || changed.CheckedInAt != nil || changed.Mode == b.Mode || *changed.QueueNo != *b.QueueNo || changed.EmailStatus != "Pending" {
		t.Fatal("date edit", changed)
	}
	var state string
	s.DB.QueryRow(`SELECT state FROM outbox WHERE booking_id=$1`, b.ID).Scan(&state)
	if state != "Pending" {
		t.Fatal("notification not queued", state)
	}
}
func TestCancelRestoreCapacityAndArchivedEdit(t *testing.T) {
	s := testService(t)
	c := book(t, s, testInput())
	b := cancelBooking(t, s, c.Booking)
	if tracked, err := s.Track(testCtx, c.Token); err != nil || tracked.Status != "Cancelled" {
		t.Fatal(tracked, err)
	}
	for n := 0; n < 4; n++ {
		book(t, s, testInput())
	}
	if _, err := s.RestoreBooking(testCtx, b.ID, b.Revision, "staff"); !codeIs(err, "PERIOD_FULL") {
		t.Fatal("restore into full", err)
	}
	if latest(t, s, b.ID).Status != "Cancelled" {
		t.Fatal("partial restore")
	}
	in := editValues(b)
	in.Subject = "Verbal"
	in.PeriodID = "P1000"
	b, err := s.EditBooking(testCtx, b.ID, in, "staff")
	if err != nil || b.Status != "Cancelled" {
		t.Fatal("archived edit", b, err)
	}
	restored, err := s.RestoreBooking(testCtx, b.ID, b.Revision, "staff")
	if err != nil || restored.Status != "Booked" || restored.ID != c.Booking.ID || restored.CancelledAt != "" || restored.CancellationReason != "" || len(restored.Orders) != 0 {
		t.Fatal(restored, err)
	}
	if _, err = s.RestoreBooking(testCtx, b.ID, b.Revision, "staff"); !codeIs(err, "REVISION_CONFLICT") {
		t.Fatal("duplicate restore", err)
	}
	// Archived records may point at a full target but do not reserve it until restore.
	another := book(t, s, testInputForPeriod("P1800")).Booking
	another = cancelBooking(t, s, another)
	in = editValues(another)
	in.PeriodID = "P1100"
	another, err = s.EditBooking(testCtx, another.ID, in, "staff")
	if err != nil {
		t.Fatal("edit full target while cancelled", err)
	}
	if _, err = s.RestoreBooking(testCtx, another.ID, another.Revision, "staff"); !codeIs(err, "PERIOD_FULL") {
		t.Fatal(err)
	}
	audit, _ := s.Audit(testCtx)
	found := false
	for _, a := range audit {
		if a.BookingID == restored.ID && a.Action == "RESTORE_BOOKING" && strings.Contains(a.Previous, "ไม่มา") {
			found = true
		}
	}
	if !found {
		t.Fatal("restoration audit lacks cancel reason")
	}
}
func testInputForPeriod(id string) Input { i := testInput(); i.PeriodID = id; return i }
func TestRestoreCheckedInNoShowAndPartialCombined(t *testing.T) {
	s := testService(t)
	c := book(t, s, testInput())
	assign(t, s, defaultScope(), c.Booking.ID)
	b := transition(t, s, c.Booking.ID, "Waiting")
	check := *b.CheckedInAt
	b = transition(t, s, b.ID, "No-show")
	b = cancelBooking(t, s, b)
	b, err := s.RestoreBooking(testCtx, b.ID, b.Revision, "staff")
	if err != nil || b.Status != "Waiting" || b.QueueNo != nil || *b.CheckedInAt != check || len(b.Orders) != 0 {
		t.Fatal("no-show restore", b, err)
	}
	i := testInput()
	i.Subject = "Combined"
	i.PeriodID = "P1300"
	c = book(t, s, i)
	scope := Scope{i.Level, i.Date, i.PeriodID, "Math"}
	assign(t, s, scope, c.Booking.ID)
	transition(t, s, c.Booking.ID, "Waiting")
	transition(t, s, c.Booking.ID, "Calling")
	transition(t, s, c.Booking.ID, "Serving")
	b = transition(t, s, c.Booking.ID, "Completed")
	ended := b.Stages["Math"].EndedAt
	b = cancelBooking(t, s, b)
	in := editValues(b)
	in.Subject = "Verbal"
	in.PeriodID = "P1800"
	if _, err = s.EditBooking(testCtx, b.ID, in, "staff"); !codeIs(err, "BOOKING_EDIT_LOCKED") {
		t.Fatal("partial consult history reassigned", err)
	}
	b, err = s.RestoreBooking(testCtx, b.ID, b.Revision, "staff")
	if err != nil || b.Status != "Waiting" || b.CurrentSubject != "Verbal" || b.Stages["Math"].EndedAt != ended || b.QueueNo != nil {
		t.Fatal("partial restore", b, err)
	}
	scope.Stream = "Verbal"
	assign(t, s, scope, b.ID)
	transition(t, s, b.ID, "Calling")
	transition(t, s, b.ID, "Serving")
	b = transition(t, s, b.ID, "Completed")
	if b.Status != "Completed" {
		t.Fatal(b)
	}
}
func TestConcurrentRestoreLastSlot(t *testing.T) {
	s := testService(t)
	i := testInput()
	i.PeriodID = "P1900"
	cancelled := []Booking{}
	for n := 0; n < 2; n++ {
		cancelled = append(cancelled, cancelBooking(t, s, book(t, s, i).Booking))
	}
	for n := 0; n < 3; n++ {
		book(t, s, i)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, b := range cancelled {
		wg.Add(1)
		go func(b Booking) {
			defer wg.Done()
			_, err := s.RestoreBooking(testCtx, b.ID, b.Revision, "staff")
			results <- err
		}(b)
	}
	wg.Wait()
	close(results)
	ok, full := 0, 0
	for err := range results {
		if err == nil {
			ok++
		} else if codeIs(err, "PERIOD_FULL") {
			full++
		} else {
			t.Fatal(err)
		}
	}
	if ok != 1 || full != 1 {
		t.Fatal(fmt.Sprint(ok, full))
	}
}

func TestEditCompetesWithRestoreForLastSlot(t *testing.T) {
	s := testService(t)
	target := testInputForPeriod("P1900")
	archived := cancelBooking(t, s, book(t, s, target).Booking)
	for n := 0; n < 3; n++ {
		book(t, s, target)
	}
	source := book(t, s, testInput()).Booking
	in := editValues(source)
	in.PeriodID = target.PeriodID
	var wg sync.WaitGroup
	results := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := s.RestoreBooking(testCtx, archived.ID, archived.Revision, "staff")
		results <- err
	}()
	go func() { defer wg.Done(); _, err := s.EditBooking(testCtx, source.ID, in, "staff"); results <- err }()
	wg.Wait()
	close(results)
	accepted, full := 0, 0
	for err := range results {
		if err == nil {
			accepted++
		} else if codeIs(err, "PERIOD_FULL") {
			full++
		} else {
			t.Fatal(err)
		}
	}
	if accepted != 1 || full != 1 {
		t.Fatal(accepted, full)
	}
	var count int
	s.DB.QueryRow(`SELECT COUNT(*) FROM bookings WHERE period_id='P1900' AND status!='Cancelled'`).Scan(&count)
	if count != 4 {
		t.Fatal(count)
	}
	s.Config.Event.ID = "another-event"
	if _, err := s.EditBooking(testCtx, source.ID, in, "staff"); !codeIs(err, "BOOKING_NOT_FOUND") {
		t.Fatal("cross event edit", err)
	}
	if _, err := s.RestoreBooking(testCtx, archived.ID, archived.Revision, "staff"); !codeIs(err, "BOOKING_NOT_FOUND") {
		t.Fatal("cross event restore", err)
	}
}

type captureConfirmation struct{ sent []Booking }

func (m *captureConfirmation) Send(_ context.Context, b Booking, _ string) error {
	m.sent = append(m.sent, b)
	return nil
}
func TestRestoreQueuesLatestConfirmation(t *testing.T) {
	s := testService(t)
	s.MailEnabled = true
	c := book(t, s, testInput())
	b := cancelBooking(t, s, c.Booking)
	m := &captureConfirmation{}
	if _, err := s.ProcessMail(testCtx, m); err != nil || len(m.sent) != 0 {
		t.Fatal("cancelled mail", err)
	}
	b = latest(t, s, b.ID)
	in := editValues(b)
	in.Subject = "Verbal"
	in.PeriodID = "P1900"
	b, err := s.EditBooking(testCtx, b.ID, in, "staff")
	if err != nil {
		t.Fatal(err)
	}
	b, err = s.RestoreBooking(testCtx, b.ID, b.Revision, "staff")
	if err != nil || b.EmailStatus != "Pending" {
		t.Fatal(b, err)
	}
	if _, err = s.ProcessMail(testCtx, m); err != nil || len(m.sent) != 1 {
		t.Fatal("restore email", err)
	}
	if m.sent[0].Subject != "Verbal" || m.sent[0].PeriodID != "P1900" || m.sent[0].ID != c.Booking.ID {
		t.Fatal("sent old reservation", m.sent[0])
	}
}
