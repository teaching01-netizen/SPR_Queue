package consult

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func recoveryInput(b Booking, action string) BookingRecovery {
	in := BookingRecovery{Action: action, Reason: "ทดลองระบบ / แก้สถานะผิด", Revision: b.Revision, Confirmation: "RECOVER BOOKING " + b.ID}
	if action == "reset-checkin" && b.Subject == "Combined" {
		in.StartSubject = "Verbal"
	}
	return in
}

func TestRecoveryCancelCompletedReturnsOnlyOneSlot(t *testing.T) {
	s := cleanupService(t)
	input := cleanupInput()
	input.Nickname = "Tar"
	input.Wcode = "W00001"
	c := book(t, s, input)
	done := finishCleanup(t, s, c)
	archived, err := s.ArchiveCompleted(testCtx, done.ID, true, done.Revision, "RD1")
	if err != nil {
		t.Fatal(err)
	}
	others := []Booking{}
	for n := 0; n < 7; n++ {
		others = append(others, book(t, s, cleanupInput()).Booking)
	}
	if _, err = s.Book(testCtx, "full-before-recovery", cleanupInput()); !codeIs(err, "PERIOD_FULL") {
		t.Fatal("period not full", err)
	}
	s.Config.Event.Sample = false
	s.MailEnabled = true
	if s.CanManageTestData() {
		t.Fatal("production destructive tools enabled")
	}
	if _, err = s.DB.ExecContext(testCtx, `INSERT INTO outbox(booking_id,state) VALUES($1,'Pending')`, c.Booking.ID); err != nil {
		t.Fatal(err)
	}
	before := latest(t, s, archived.ID)
	out, err := s.RecoverBooking(testCtx, before.ID, recoveryInput(before, "cancel"), "RD1")
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != "Cancelled" || out.CancelledAt == "" || out.CompletedAt != before.CompletedAt || !reflect.DeepEqual(out.Stages, before.Stages) || out.QueueNo != nil {
		t.Fatal("cancellation lost historical data", out)
	}
	for _, b := range others {
		if !reflect.DeepEqual(b, latest(t, s, b.ID)) {
			t.Fatal("another real booking changed")
		}
	}
	var mail string
	if err = s.DB.QueryRowContext(testCtx, `SELECT state FROM outbox WHERE booking_id=$1`, c.Booking.ID).Scan(&mail); err != nil || mail != "Cancelled" {
		t.Fatal("test email remained pending", err)
	}
	file, err := s.StaffExport(testCtx)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range exportParts(t, file) {
		if strings.Contains(p, c.Booking.ID) {
			t.Fatal("removed record exported")
		}
	}
	snapshot, err := s.sheetSnapshot(testCtx)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range snapshot.Rows {
		if r[0] == c.Booking.ID {
			t.Fatal("removed record sent to Sheets")
		}
	}
	if _, err = s.Book(testCtx, "one-slot-after-recovery", cleanupInput()); err != nil {
		t.Fatal("slot not returned", err)
	}
	if _, err = s.Book(testCtx, "only-one-slot-returned", cleanupInput()); !codeIs(err, "PERIOD_FULL") {
		t.Fatal("more than one slot freed", err)
	}
	audit, err := s.Audit(testCtx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, a := range audit {
		if a.Action == "ADMIN_CANCEL_BOOKING" {
			found = true
			var original Booking
			if json.Unmarshal([]byte(a.Previous), &original) != nil || original.Status != "Completed" || original.CompletedAt == "" || a.Actor != "RD1" {
				t.Fatal("missing original audit", a)
			}
		}
	}
	if !found {
		t.Fatal("missing recovery audit")
	}
}

func TestRecoveryResetPreservesRealCodeTokenAndOtherParents(t *testing.T) {
	s := cleanupService(t)
	input := cleanupInput()
	input.Subject = "Verbal"
	one := book(t, s, input)
	two := book(t, s, input)
	real := book(t, s, input)
	b := transition(t, s, real.Booking.ID, "Waiting")
	scope := Scope{b.Level, b.Date, b.PeriodID, "Verbal"}
	assign(t, s, scope, one.Booking.ID, two.Booking.ID, b.ID)
	transition(t, s, b.ID, "Calling")
	transition(t, s, b.ID, "Serving")
	done := transition(t, s, b.ID, "Completed")
	done, err := s.ArchiveCompleted(testCtx, done.ID, true, done.Revision, "RD1")
	if err != nil {
		t.Fatal(err)
	}
	if *done.QueueNo != "B-V-003" {
		t.Fatal("fixture code", *done.QueueNo)
	}
	beforeOne, beforeTwo := latest(t, s, one.Booking.ID), latest(t, s, two.Booking.ID)
	s.Config.Event.Sample = false
	s.MailEnabled = true
	if _, err = s.DB.ExecContext(testCtx, `INSERT INTO outbox(booking_id,state) VALUES($1,'Sent')`, done.ID); err != nil {
		t.Fatal(err)
	}
	out, err := s.RecoverBooking(testCtx, done.ID, recoveryInput(done, "reset-checkin"), "RD1")
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != "Booked" || out.CheckedInAt != nil || len(out.Stages) != 0 || len(out.Orders) != 0 || out.CompletedAt != "" || out.ArchivedAt != "" || out.CancelledAt != "" || out.Counter != "" || *out.QueueNo != "B-V-003" {
		t.Fatal("not reset to check-in", out)
	}
	if !reflect.DeepEqual(beforeOne, latest(t, s, one.Booking.ID)) || !reflect.DeepEqual(beforeTwo, latest(t, s, two.Booking.ID)) {
		t.Fatal("other parent modified")
	}
	link, err := s.StaffTrackingLink(testCtx, out.ID)
	if err != nil || link.TrackingURL != real.TrackingURL {
		t.Fatal("original tracking changed", err)
	}
	view, err := s.Track(testCtx, real.Token)
	if err != nil || view.Status != "Booked" || view.QueueCountState != "awaiting-check-in" {
		t.Fatal("parent not returned to check-in", err, view)
	}
	var mail string
	s.DB.QueryRowContext(testCtx, `SELECT state FROM outbox WHERE booking_id=$1`, out.ID).Scan(&mail)
	if mail != "Sent" {
		t.Fatal("email resent")
	}
	four := book(t, s, input)
	transition(t, s, out.ID, "Waiting")
	assign(t, s, scope, one.Booking.ID, two.Booking.ID, out.ID, four.Booking.ID)
	if *latest(t, s, out.ID).QueueNo != "B-V-003" || *latest(t, s, four.Booking.ID).QueueNo != "B-V-004" {
		t.Fatal("counter reset or original code replaced")
	}
}

func TestRecoveryCombinedCapacityValidationAndSendingGuard(t *testing.T) {
	s := cleanupService(t)
	i := cleanupInput()
	i.Subject = "Combined"
	i.PeriodID = "P1400_1530"
	c := book(t, s, i)
	assign(t, s, Scope{i.Level, i.Date, i.PeriodID, "Math"}, c.Booking.ID)
	b := latest(t, s, c.Booking.ID)
	cancelled, err := s.Status(testCtx, b.ID, "Cancelled", b.Revision, "RD1", "ทดลอง")
	if err != nil {
		t.Fatal(err)
	}
	full := []Booking{}
	for n := 0; n < 5; n++ {
		full = append(full, book(t, s, i).Booking)
	}
	if _, err = s.RecoverBooking(testCtx, cancelled.ID, recoveryInput(cancelled, "reset-checkin"), "RD1"); !codeIs(err, "PERIOD_FULL") {
		t.Fatal("overbooked", err)
	}
	if !reflect.DeepEqual(cancelled, latest(t, s, cancelled.ID)) {
		t.Fatal("failed recovery changed record")
	}
	if _, err = s.Status(testCtx, full[0].ID, "Cancelled", full[0].Revision, "RD1", "คืนที่ว่าง"); err != nil {
		t.Fatal(err)
	}
	bad := recoveryInput(cancelled, "reset-checkin")
	bad.StartSubject = ""
	if _, err = s.RecoverBooking(testCtx, cancelled.ID, bad, "RD1"); !codeIs(err, "VALIDATION_ERROR") {
		t.Fatal(err)
	}
	out, err := s.RecoverBooking(testCtx, cancelled.ID, recoveryInput(cancelled, "reset-checkin"), "RD1")
	if err != nil || out.CurrentSubject != "Verbal" || out.QueueNo != nil {
		t.Fatal("combined recovery", err)
	}
	for _, change := range []func(*BookingRecovery){func(i *BookingRecovery) { i.Confirmation = "" }, func(i *BookingRecovery) { i.Reason = "" }, func(i *BookingRecovery) { i.Action = "clear-all" }, func(i *BookingRecovery) { i.Revision-- }} {
		in := recoveryInput(out, "cancel")
		change(&in)
		if _, err = s.RecoverBooking(testCtx, out.ID, in, "RD1"); err == nil {
			t.Fatal("invalid recovery accepted")
		}
	}
	if _, err = s.DB.ExecContext(testCtx, `INSERT INTO outbox(booking_id,state) VALUES($1,'Sending')`, out.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.RecoverBooking(testCtx, out.ID, recoveryInput(out, "cancel"), "RD1"); !codeIs(err, "NOTIFICATION_BUSY") {
		t.Fatal("sending race", err)
	}
	s.Config.Event.ID = "another-event"
	if _, err = s.RecoverBooking(testCtx, out.ID, recoveryInput(out, "cancel"), "RD1"); !codeIs(err, "BOOKING_NOT_FOUND") {
		t.Fatal("wrong event", err)
	}
}

func TestHTTPRecoveryRequiresAdminCSRFAndSingleRecordConfirmation(t *testing.T) {
	s := cleanupService(t)
	c := book(t, s, cleanupInput())
	a, err := NewAuth(s, false)
	if err != nil {
		t.Fatal(err)
	}
	a.Cost = bcrypt.MinCost
	if err = a.SyncLocalStaff(testCtx); err != nil {
		t.Fatal(err)
	}
	if _, err = s.DB.ExecContext(testCtx, `UPDATE users SET role='admin' WHERE username='RD1'`); err != nil {
		t.Fatal(err)
	}
	admin, err := a.Login(testCtx, "RD1", "RD1")
	if err != nil {
		t.Fatal(err)
	}
	operator, err := a.Login(testCtx, "SM1", "SM1")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []Identity{admin, operator} {
		if err = a.StoreSession(testCtx, id, ""); err != nil {
			t.Fatal(err)
		}
	}
	h := NewHTTP(s, a, []string{"http://localhost:8080"}, "").Handler()
	post := func(token, csrf, origin string, in any) *httptest.ResponseRecorder {
		body, _ := json.Marshal(in)
		r := httptest.NewRequest("POST", "/api/staff/bookings/"+c.Booking.ID+"/recovery", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if token != "" {
			r.AddCookie(&http.Cookie{Name: "warwick_session", Value: token})
		}
		if csrf != "" {
			r.Header.Set("X-CSRF-Token", csrf)
		}
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	in := recoveryInput(c.Booking, "cancel")
	if w := post("", "", "", in); w.Code != 401 {
		t.Fatal("anonymous", w.Code)
	}
	if w := post(operator.Token, a.csrf(operator.Token), "http://localhost:8080", in); w.Code != 403 {
		t.Fatal("operator", w.Code)
	}
	if w := post(admin.Token, "", "http://localhost:8080", in); w.Code != 403 {
		t.Fatal("csrf", w.Code)
	}
	if w := post(admin.Token, a.csrf(admin.Token), "https://forged.example", in); w.Code != 403 {
		t.Fatal("origin", w.Code)
	}
	if w := post(admin.Token, a.csrf(admin.Token), "http://localhost:8080", map[string]any{"action": "cancel", "all": true}); w.Code != 400 {
		t.Fatal("bulk action accepted", w.Code)
	}
	if w := post(admin.Token, a.csrf(admin.Token), "http://localhost:8080", in); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := post(admin.Token, a.csrf(admin.Token), "http://localhost:8080", in); w.Code != 409 {
		t.Fatal("stale action accepted", w.Code)
	}
}
