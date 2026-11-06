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

func TestStudentDetailsPreserveQueueAcrossLifecycle(t *testing.T) {
	s := cleanupService(t)
	c := book(t, s, cleanupInput())
	s.MailEnabled = true
	if _, err := s.DB.ExecContext(testCtx, `INSERT INTO outbox(booking_id,state) VALUES($1,'Sent')`, c.Booking.ID); err != nil {
		t.Fatal(err)
	}
	edit := func(id string) Booking {
		t.Helper()
		before := latest(t, s, id)
		in := StudentDetailsEdit{Wcode: " 000012 ", Nickname: before.Nickname + " แก้", School: " โรงเรียนใหม่ ", Email: " NEW@EXAMPLE.COM ", Revision: before.Revision}
		out, err := s.EditStudentDetails(testCtx, id, in, "SM2")
		if err != nil {
			t.Fatal(err)
		}
		if out.Wcode != "000012" || out.Nickname != strings.TrimSpace(in.Nickname) || out.School != "โรงเรียนใหม่" || out.Email != "new@example.com" || out.Revision != before.Revision+1 {
			t.Fatal("profile not updated", out)
		}
		// Nothing except the four fields and record revision/time may change.
		compare := out
		compare.Wcode = before.Wcode
		compare.Nickname = before.Nickname
		compare.School = before.School
		compare.Email = before.Email
		compare.Revision = before.Revision
		compare.UpdatedAt = before.UpdatedAt
		if !reflect.DeepEqual(compare, before) {
			t.Fatal("profile edit changed operational data")
		}
		if id == c.Booking.ID {
			link, err := s.StaffTrackingLink(testCtx, id)
			if err != nil || link.TrackingURL != c.TrackingURL {
				t.Fatal("tracking changed", err)
			}
		}
		return out
	}
	edit(c.Booking.ID)
	b := transition(t, s, c.Booking.ID, "Waiting")
	assign(t, s, Scope{b.Level, b.Date, b.PeriodID, b.CurrentSubject}, b.ID)
	edit(b.ID)
	transition(t, s, b.ID, "Calling")
	edit(b.ID)
	transition(t, s, b.ID, "Serving")
	edit(b.ID)
	done := transition(t, s, b.ID, "Completed")
	done = edit(done.ID)
	if _, err := s.ArchiveCompleted(testCtx, done.ID, true, done.Revision, "RD1"); err != nil {
		t.Fatal(err)
	}
	edit(done.ID)
	other := book(t, s, cleanupInput())
	cancelled, err := s.Status(testCtx, other.Booking.ID, "Cancelled", other.Booking.Revision, "RD1", "Test cancellation")
	if err != nil {
		t.Fatal(err)
	}
	edit(cancelled.ID)
	var mailState string
	if err = s.DB.QueryRowContext(testCtx, `SELECT state FROM outbox WHERE booking_id=$1`, c.Booking.ID).Scan(&mailState); err != nil || mailState != "Sent" {
		t.Fatal("sent email requeued", err, mailState)
	}
	s.Config.Rules = nil
	s.Config.Event.Sample = false
	edit(done.ID) // retired periods cannot block a contact correction
	current := latest(t, s, done.ID)
	v, err := s.StaffList(testCtx)
	if err != nil {
		t.Fatal(err)
	}
	input := StudentDetailsEdit{current.Wcode, current.Nickname, current.School, current.Email, current.Revision}
	if _, err = s.EditStudentDetails(testCtx, current.ID, input, "SM2"); err != nil {
		t.Fatal(err)
	}
	after, err := s.StaffList(testCtx)
	if err != nil || !sameStaffState(v, after) {
		t.Fatal("no-op changed snapshot", err)
	}
	for _, bad := range []StudentDetailsEdit{{"", "", current.School, current.Email, current.Revision}, {"", current.Nickname, "", current.Email, current.Revision}, {"", current.Nickname, current.School, "bad\r\n@example.com", current.Revision}, {strings.Repeat("W", 41), current.Nickname, current.School, current.Email, current.Revision}, {"", strings.Repeat("ก", 81), current.School, current.Email, current.Revision}} {
		if _, err = s.EditStudentDetails(testCtx, current.ID, bad, "SM2"); !codeIs(err, "VALIDATION_ERROR") {
			t.Fatal("invalid details accepted", err)
		}
	}
	input.Revision--
	if _, err = s.EditStudentDetails(testCtx, current.ID, input, "SM2"); !codeIs(err, "REVISION_CONFLICT") {
		t.Fatal(err)
	}
	input.Revision++
	input.Nickname += "ใหม่"
	if _, err = s.DB.ExecContext(testCtx, `UPDATE outbox SET state='Sending' WHERE booking_id=$1`, c.Booking.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.EditStudentDetails(testCtx, current.ID, input, "SM2"); !codeIs(err, "NOTIFICATION_BUSY") {
		t.Fatal("email race not blocked", err)
	}
	s.Config.Event.ID = "another-event"
	if _, err = s.EditStudentDetails(testCtx, current.ID, input, "SM2"); !codeIs(err, "BOOKING_NOT_FOUND") {
		t.Fatal("other event edited", err)
	}
	s.Config.Event.ID = current.EventID
	audit, err := s.Audit(testCtx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, a := range audit {
		if a.Action == "EDIT_STUDENT_DETAILS" {
			found = true
			if a.Actor != "SM2" || !strings.Contains(a.Next, "000012") || a.Previous == a.Next {
				t.Fatal("missing audit", a)
			}
		}
	}
	if !found {
		t.Fatal("no profile audit")
	}
}

func TestHTTPStudentDetailsAuthorizationAndExport(t *testing.T) {
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
	id, err := a.Login(testCtx, "SM1", "SM1")
	if err != nil {
		t.Fatal(err)
	}
	if err = a.StoreSession(testCtx, id, ""); err != nil {
		t.Fatal(err)
	}
	h := NewHTTP(s, a, []string{"http://localhost:8080"}, "").Handler()
	payload := map[string]any{"wcode": "W12345", "nickname": "ชื่อแก้แล้ว", "school": "โรงเรียนใหม่", "email": "correct@example.com", "expectedRevision": c.Booking.Revision}
	patch := func(login, csrf bool, origin string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(payload)
		r := httptest.NewRequest("PATCH", "/api/staff/bookings/"+c.Booking.ID+"/student-details", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if login {
			r.AddCookie(&http.Cookie{Name: "warwick_session", Value: id.Token})
		}
		if csrf {
			r.Header.Set("X-CSRF-Token", a.csrf(id.Token))
		}
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := patch(false, false, ""); w.Code != 401 {
		t.Fatal("anonymous edit", w.Code)
	}
	if w := patch(true, false, "http://localhost:8080"); w.Code != 403 {
		t.Fatal("csrf edit", w.Code)
	}
	if w := patch(true, true, "https://forged.example"); w.Code != 403 {
		t.Fatal("forged origin", w.Code)
	}
	payload["status"] = "Completed"
	if w := patch(true, true, "http://localhost:8080"); w.Code != 400 {
		t.Fatal("forged status", w.Code)
	}
	delete(payload, "status")
	w := patch(true, true, "http://localhost:8080")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = patch(true, true, "http://localhost:8080"); w.Code != 409 {
		t.Fatal("stale editor overwrote data", w.Code)
	}
	file, err := s.StaffExport(testCtx)
	if err != nil {
		t.Fatal(err)
	}
	rows := exportRows(t, exportParts(t, file)["xl/worksheets/sheet1.xml"])
	if len(rows) != 2 || rows[1][2] != "W12345" || rows[1][3] != "ชื่อแก้แล้ว" || rows[1][5] != "correct@example.com" {
		t.Fatal("corrected data missing in export", rows)
	}
}
