package consult

import (
	"bytes"
	"encoding/json"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func cleanupService(t *testing.T) *Service {
	s := novemberService(t)
	s.Config.Event.ID = "local-test-" + s.Config.Event.ID
	s.LocalTestDataManagement = true
	if err := s.migrate(testCtx); err != nil {
		t.Fatal(err)
	}
	return s
}
func cleanupInput() Input {
	return Input{Email: "parent@example.com", Nickname: "Test Cleanup", School: "School", Level: "Beginner", Subject: "Math", Date: "2026-11-07", PeriodID: "P1800_2000", Questions: []string{"one", "two"}, SeminarConfirmed: true}
}
func finishCleanup(t *testing.T, s *Service, c Confirmation) Booking {
	b := transition(t, s, c.Booking.ID, "Waiting")
	assign(t, s, Scope{b.Level, b.Date, b.PeriodID, b.CurrentSubject}, b.ID)
	transition(t, s, b.ID, "Calling")
	transition(t, s, b.ID, "Serving")
	return transition(t, s, b.ID, "Completed")
}
func TestArchiveCompletedPreservesHistoryCapacityAndToken(t *testing.T) {
	s := novemberService(t)
	c := book(t, s, cleanupInput())
	done := finishCleanup(t, s, c)
	s.LocalTestDataManagement = false
	archived, err := s.ArchiveCompleted(testCtx, done.ID, true, done.Revision, "RD1")
	if err != nil {
		t.Fatal(err)
	}
	if archived.Status != "Completed" || archived.ArchivedAt == "" || archived.ArchivedBy != "RD1" || !reflect.DeepEqual(archived.Stages, done.Stages) || archived.CompletedAt != done.CompletedAt || *archived.QueueNo != *done.QueueNo {
		t.Fatal("lost completed history")
	}
	if v, err := s.Track(testCtx, c.Token); err != nil || v.Status != "Completed" {
		t.Fatal("lost tracking", err)
	}
	for n := 0; n < 7; n++ {
		book(t, s, cleanupInput())
	}
	if _, err = s.Book(testCtx, "full-after-archive", cleanupInput()); !codeIs(err, "PERIOD_FULL") {
		t.Fatal("archive freed used capacity", err)
	}
	if _, err = s.ArchiveCompleted(testCtx, done.ID, false, done.Revision, "RD2"); !codeIs(err, "REVISION_CONFLICT") {
		t.Fatal("stale archive", err)
	}
	unarchived, err := s.ArchiveCompleted(testCtx, done.ID, false, archived.Revision, "RD2")
	if err != nil || unarchived.ArchivedAt != "" || unarchived.Status != "Completed" {
		t.Fatal("unarchive reopened case", err)
	}
	audit, err := s.Audit(testCtx)
	if err != nil {
		t.Fatal(err)
	}
	if audit[0].Actor != "RD2" || audit[0].Action != "UNARCHIVE_COMPLETED" {
		t.Fatal("archive actor missing")
	}
	if _, err = s.RestoreBooking(testCtx, done.ID, BookingRestore{Revision: unarchived.Revision}, "RD2"); !codeIs(err, "RESTORE_NOT_ALLOWED") {
		t.Fatal("completed reopened", err)
	}
}
func TestArchiveRejectsUnfinishedOrWrongEvent(t *testing.T) {
	s := cleanupService(t)
	c := book(t, s, cleanupInput())
	if _, err := s.ArchiveCompleted(testCtx, c.Booking.ID, true, c.Booking.Revision, "RD1"); !codeIs(err, "ARCHIVE_NOT_ALLOWED") {
		t.Fatal(err)
	}
	s.Config.Event.ID = "another-event"
	if _, err := s.ArchiveCompleted(testCtx, c.Booking.ID, true, c.Booking.Revision, "RD1"); !codeIs(err, "BOOKING_NOT_FOUND") {
		t.Fatal(err)
	}
}
func TestDeleteCompletedTestBookingAndKeepOtherCodes(t *testing.T) {
	s := cleanupService(t)
	c := book(t, s, cleanupInput())
	done := finishCleanup(t, s, c)
	other := book(t, s, cleanupInput())
	assign(t, s, Scope{done.Level, done.Date, done.PeriodID, "Math"}, other.Booking.ID)
	before := latest(t, s, other.Booking.ID)
	if err := s.DeleteTestBooking(testCtx, done.ID, done.Revision, "wrong"); !codeIs(err, "CONFIRMATION_REQUIRED") {
		t.Fatal(err)
	}
	if err := s.DeleteTestBooking(testCtx, done.ID, done.Revision-1, "DELETE TEST BOOKING"); !codeIs(err, "REVISION_CONFLICT") {
		t.Fatal(err)
	}
	if err := s.DeleteTestBooking(testCtx, done.ID, done.Revision, "DELETE TEST BOOKING"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Track(testCtx, c.Token); !codeIs(err, "BOOKING_NOT_FOUND") {
		t.Fatal("deleted token works", err)
	}
	if !reflect.DeepEqual(before, latest(t, s, other.Booking.ID)) {
		t.Fatal("other booking changed")
	}
	fresh := book(t, s, cleanupInput())
	assign(t, s, Scope{done.Level, done.Date, done.PeriodID, "Math"}, other.Booking.ID, fresh.Booking.ID)
	if *latest(t, s, fresh.Booking.ID).QueueNo != "B-M-001" {
		t.Fatal("single deletion did not reuse free code")
	}
	var count int
	s.DB.QueryRow(`SELECT count(*) FROM audit WHERE booking_id=$1`, done.ID).Scan(&count)
	if count != 0 {
		t.Fatal("deleted test history remains")
	}
}
func TestTestDeleteGatesAndNotificationBusy(t *testing.T) {
	s := cleanupService(t)
	c := book(t, s, cleanupInput())
	for _, change := range []func(){func() { s.LocalTestDataManagement = false }, func() { s.Config.Event.Sample = false }, func() { s.MailEnabled = true }, func() { s.Config.Event.ID = "real-event" }} {
		id := s.Config.Event.ID
		change()
		if err := s.DeleteTestBooking(testCtx, c.Booking.ID, c.Booking.Revision, "DELETE TEST BOOKING"); !codeIs(err, "TEST_DELETE_NOT_ALLOWED") {
			t.Fatal(err)
		}
		if _, err := s.ClearCurrentTestEvent(testCtx, 0, "CLEAR TEST DATA"); !codeIs(err, "TEST_DELETE_NOT_ALLOWED") {
			t.Fatal(err)
		}
		s.LocalTestDataManagement = true
		s.Config.Event.Sample = true
		s.MailEnabled = false
		s.Config.Event.ID = id
	}
	if _, err := s.DB.Exec(`INSERT INTO outbox(booking_id,state,lease_at) VALUES($1,'Sending',$2)`, c.Booking.ID, time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteTestBooking(testCtx, c.Booking.ID, c.Booking.Revision, "DELETE TEST BOOKING"); !codeIs(err, "NOTIFICATION_BUSY") {
		t.Fatal(err)
	}
	p, _ := s.StaffList(testCtx)
	if _, err := s.ClearCurrentTestEvent(testCtx, p.Revision, "CLEAR TEST DATA"); !codeIs(err, "NOTIFICATION_BUSY") {
		t.Fatal(err)
	}
	s.DB.Exec(`UPDATE outbox SET lease_at=0 WHERE booking_id=$1`, c.Booking.ID)
	if err := s.DeleteTestBooking(testCtx, c.Booking.ID, c.Booking.Revision, "DELETE TEST BOOKING"); err != nil {
		t.Fatal(err)
	}
}
func TestClearCurrentTestEventPreservesOtherEventUsersAndSession(t *testing.T) {
	s := cleanupService(t)
	c := book(t, s, cleanupInput())
	done := finishCleanup(t, s, c)
	if _, err := s.ArchiveCompleted(testCtx, done.ID, true, done.Revision, "RD1"); err != nil {
		t.Fatal(err)
	}
	auth, err := NewAuth(s, false)
	if err != nil {
		t.Fatal(err)
	}
	auth.Cost = bcrypt.MinCost
	if err = auth.SyncLocalStaff(testCtx); err != nil {
		t.Fatal(err)
	}
	id, err := auth.Login(testCtx, "RD1", "RD1")
	if err != nil {
		t.Fatal(err)
	}
	if err = auth.StoreSession(testCtx, id, ""); err != nil {
		t.Fatal(err)
	}
	currentID := s.Config.Event.ID
	s.Config.Event.ID = "local-test-another"
	if err = s.migrate(testCtx); err != nil {
		t.Fatal(err)
	}
	other := book(t, s, cleanupInput())
	s.Config.Event.ID = currentID
	packet, _ := s.StaffList(testCtx)
	if _, err = s.ClearCurrentTestEvent(testCtx, packet.Revision-1, "CLEAR TEST DATA"); !codeIs(err, "REVISION_CONFLICT") {
		t.Fatal(err)
	}
	if _, err = s.ClearCurrentTestEvent(testCtx, packet.Revision, "wrong"); !codeIs(err, "CONFIRMATION_REQUIRED") {
		t.Fatal(err)
	}
	count, err := s.ClearCurrentTestEvent(testCtx, packet.Revision, "CLEAR TEST DATA")
	if err != nil || count != 1 {
		t.Fatal(count, err)
	}
	packet, _ = s.StaffList(testCtx)
	if len(packet.Bookings) != 0 {
		t.Fatal("not cleared")
	}
	if _, err = s.Track(testCtx, c.Token); !codeIs(err, "BOOKING_NOT_FOUND") {
		t.Fatal(err)
	}
	var users, sessions int
	s.DB.QueryRow(`SELECT count(*) FROM users`).Scan(&users)
	s.DB.QueryRow(`SELECT count(*) FROM sessions`).Scan(&sessions)
	if users != 7 || sessions != 1 {
		t.Fatal("removed accounts/session", users, sessions)
	}
	s.Config.Event.ID = "local-test-another"
	if _, err = s.Track(testCtx, other.Token); err != nil {
		t.Fatal("removed other event", err)
	}
	s.Config.Event.ID = currentID
	fresh := book(t, s, cleanupInput())
	assign(t, s, Scope{fresh.Booking.Level, fresh.Booking.Date, fresh.Booking.PeriodID, "Math"}, fresh.Booking.ID)
	if *latest(t, s, fresh.Booking.ID).QueueNo != "B-M-001" {
		t.Fatal("bulk clear did not restart sequence")
	}
}
func TestHTTPCleanupAuthorizationAndServerMode(t *testing.T) {
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
	identity, err := a.Login(testCtx, "RD1", "RD1")
	if err != nil {
		t.Fatal(err)
	}
	a.StoreSession(testCtx, identity, "")
	cookie := &http.Cookie{Name: "warwick_session", Value: identity.Token}
	csrf := a.csrf(identity.Token)
	handler := NewHTTP(s, a, []string{"http://localhost:8080"}, "").Handler()
	send := func(method, path string, payload any, authenticated bool, token, origin string) *httptest.ResponseRecorder {
		var b bytes.Buffer
		json.NewEncoder(&b).Encode(payload)
		r := httptest.NewRequest(method, path, &b)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", origin)
		r.Header.Set("X-CSRF-Token", token)
		if authenticated {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	payload := map[string]any{"expectedRevision": c.Booking.Revision, "confirmation": "DELETE TEST BOOKING"}
	path := "/api/staff/bookings/" + c.Booking.ID
	if w := send("DELETE", path, payload, false, csrf, "http://localhost:8080"); w.Code != 401 {
		t.Fatal(w.Code)
	}
	if w := send("DELETE", path, payload, true, "", "http://localhost:8080"); w.Code != 403 {
		t.Fatal(w.Code)
	}
	if w := send("DELETE", path, payload, true, csrf, "https://forged.example"); w.Code != 403 {
		t.Fatal(w.Code)
	}
	s.LocalTestDataManagement = false
	if w := send("DELETE", path, payload, true, csrf, "http://localhost:8080"); w.Code != 403 {
		t.Fatal("allowed production delete", w.Code)
	}
	s.LocalTestDataManagement = true
	if w := send("DELETE", path, payload, true, csrf, "http://localhost:8080"); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	packet, _ := s.StaffList(testCtx)
	clear := map[string]any{"expectedRevision": packet.Revision, "confirmation": "CLEAR TEST DATA", "actor": "forged"}
	if w := send("POST", "/api/staff/test-data/clear", clear, true, csrf, "http://localhost:8080"); w.Code != 400 {
		t.Fatal("forged field", w.Code)
	}
	delete(clear, "actor")
	if w := send("POST", "/api/staff/test-data/clear", clear, true, csrf, "http://localhost:8080"); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
}
