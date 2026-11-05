package consult

import (
	"bytes"
	"encoding/json"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
)

func TestQueueCodeReuseLowestGapsRestoreAndRelease(t *testing.T) {
	s := cleanupService(t)
	i := cleanupInput()
	i.Subject = "Verbal"
	bs := []Booking{}
	ids := []string{}
	for n := 0; n < 5; n++ {
		b := book(t, s, i).Booking
		bs = append(bs, b)
		ids = append(ids, b.ID)
	}
	scope := Scope{i.Level, i.Date, i.PeriodID, "Verbal"}
	assign(t, s, scope, ids...)
	for _, n := range []int{1, 3} {
		b := latest(t, s, bs[n].ID)
		c, e := s.Status(testCtx, b.ID, "Cancelled", b.Revision, "RD1", "test")
		if e != nil || c.QueueNo != nil {
			t.Fatal("cancel did not release", e)
		}
	}
	// An obsolete high-water counter must not prevent reuse of an actual free code.
	if _, e := s.DB.Exec(`INSERT INTO sequences(scope,value) VALUES($1,999) ON CONFLICT(scope) DO UPDATE SET value=999`, s.Config.Event.ID+"/B-V"); e != nil {
		t.Fatal(e)
	}
	a, b := book(t, s, i).Booking, book(t, s, i).Booking
	active := []string{bs[0].ID, bs[2].ID, bs[4].ID, a.ID, b.ID}
	assign(t, s, scope, active...)
	for id, code := range map[string]string{bs[0].ID: "B-V-001", bs[2].ID: "B-V-003", bs[4].ID: "B-V-005", a.ID: "B-V-002", b.ID: "B-V-004"} {
		q := latest(t, s, id).QueueNo
		if q == nil || *q != code {
			t.Fatal("wrong code", id, q, code)
		}
	}
	cancelled := latest(t, s, bs[1].ID)
	restored, e := s.RestoreBooking(testCtx, cancelled.ID, cancelled.Revision, "RD1")
	if e != nil || restored.QueueNo != nil {
		t.Fatal("restore reclaimed occupied 002", e)
	}
	active = append(active, restored.ID)
	assign(t, s, scope, active...)
	if *latest(t, s, restored.ID).QueueNo != "B-V-006" {
		t.Fatal("restored booking collided")
	}
	old := latest(t, s, bs[2].ID)
	in := recoveryInput(old, "reset-checkin")
	in.ReleaseQueueCode = true
	out, e := s.RecoverBooking(testCtx, old.ID, in, "RD1")
	if e != nil || out.QueueNo != nil {
		t.Fatal("reset did not release", e)
	}
	newcomer := book(t, s, i).Booking
	assign(t, s, scope, newcomer.ID, bs[0].ID, bs[4].ID, a.ID, b.ID, restored.ID, out.ID)
	if *latest(t, s, newcomer.ID).QueueNo != "B-V-003" || *latest(t, s, out.ID).QueueNo != "B-V-007" {
		t.Fatal("released reset code not reused")
	}
}

func TestQueueCodeLegacyCancelledCleanupAndPrefixes(t *testing.T) {
	s := cleanupService(t)
	i := cleanupInput()
	one := book(t, s, i)
	assign(t, s, Scope{i.Level, i.Date, i.PeriodID, "Math"}, one.Booking.ID)
	// Fixture from the previous version: a cancelled row still holds its code.
	tx, e := s.beginWrite(testCtx)
	if e != nil {
		t.Fatal(e)
	}
	old, e := getBooking(testCtx, tx, one.Booking.ID)
	if e != nil {
		t.Fatal(e)
	}
	old.Status = "Cancelled"
	old.CancelledAt = stamp()
	if e = updateBooking(testCtx, tx, &old); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	otherInput := i
	otherInput.Subject = "Verbal"
	other := book(t, s, otherInput)
	assign(t, s, Scope{i.Level, i.Date, i.PeriodID, "Verbal"}, other.Booking.ID)
	before := latest(t, s, other.Booking.ID)
	fresh := book(t, s, i)
	assign(t, s, Scope{i.Level, i.Date, i.PeriodID, "Math"}, fresh.Booking.ID)
	if latest(t, s, old.ID).QueueNo != nil || *latest(t, s, fresh.Booking.ID).QueueNo != "B-M-001" {
		t.Fatal("legacy code not released")
	}
	if !reflect.DeepEqual(before, latest(t, s, before.ID)) {
		t.Fatal("another prefix changed")
	}
	if _, e = s.Track(testCtx, one.Token); e != nil {
		t.Fatal("cancelled link lost", e)
	}
}

func TestQueueCodeConcurrentSavesCannotDuplicate(t *testing.T) {
	s := cleanupService(t)
	i := cleanupInput()
	i.PeriodID = "P1300_1400"
	a := book(t, s, i).Booking
	i.PeriodID = "P1700_1800"
	b := book(t, s, i).Booking
	view, e := s.snapshot(testCtx)
	if e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	errs := make([]error, 2)
	bookings := []Booking{a, b}
	for n, x := range bookings {
		wg.Add(1)
		go func(n int, x Booking) {
			defer wg.Done()
			errs[n] = s.Assign(testCtx, Scope{x.Level, x.Date, x.PeriodID, "Math"}, []string{x.ID}, view.Revision, "RD1")
		}(n, x)
	}
	wg.Wait()
	failed := 0
	for n, e := range errs {
		if e != nil {
			if !codeIs(e, "REVISION_CONFLICT") {
				t.Fatal(e)
			}
			failed++
			v, e := s.snapshot(testCtx)
			if e != nil {
				t.Fatal(e)
			}
			x := bookings[n]
			if e = s.Assign(testCtx, Scope{x.Level, x.Date, x.PeriodID, "Math"}, []string{x.ID}, v.Revision, "RD1"); e != nil {
				t.Fatal(e)
			}
		}
	}
	if failed != 1 {
		t.Fatal("expected one stale save", failed)
	}
	codes := map[string]bool{}
	for _, x := range bookings {
		q := latest(t, s, x.ID).QueueNo
		if q == nil || codes[*q] {
			t.Fatal("duplicate code", q)
		}
		codes[*q] = true
	}
	if !codes["B-M-001"] || !codes["B-M-002"] {
		t.Fatal(codes)
	}
}

func TestCancelledPermanentDeleteProductionIsolationAndReuse(t *testing.T) {
	s := cleanupService(t)
	one := book(t, s, cleanupInput())
	done := finishCleanup(t, s, one)
	other := book(t, s, cleanupInput())
	assign(t, s, Scope{done.Level, done.Date, done.PeriodID, "Math"}, other.Booking.ID)
	before := latest(t, s, other.Booking.ID)
	s.Config.Event.Sample = false
	s.MailEnabled = true
	if s.CanManageTestData() {
		t.Fatal("production test tools enabled")
	}
	if e := s.DeleteCancelledBooking(testCtx, done.ID, done.Revision, "DELETE CANCELLED BOOKING "+done.ID); !codeIs(e, "CANCELLED_DELETE_NOT_ALLOWED") {
		t.Fatal("deleted completed", e)
	}
	c, e := s.RecoverBooking(testCtx, done.ID, recoveryInput(done, "cancel"), "RD1")
	if e != nil {
		t.Fatal(e)
	}
	confirm := "DELETE CANCELLED BOOKING " + c.ID
	if e = s.DeleteCancelledBooking(testCtx, c.ID, c.Revision, "DELETE CANCELLED BOOKING another"); !codeIs(e, "CONFIRMATION_REQUIRED") {
		t.Fatal(e)
	}
	if e = s.DeleteCancelledBooking(testCtx, c.ID, c.Revision-1, confirm); !codeIs(e, "REVISION_CONFLICT") {
		t.Fatal(e)
	}
	originalEvent := s.Config.Event.ID
	s.Config.Event.ID = "another-event"
	if e = s.DeleteCancelledBooking(testCtx, c.ID, c.Revision, confirm); !codeIs(e, "BOOKING_NOT_FOUND") {
		t.Fatal(e)
	}
	s.Config.Event.ID = originalEvent
	if _, e = s.DB.Exec(`INSERT INTO outbox(booking_id,state) VALUES($1,'Sending')`, c.ID); e != nil {
		t.Fatal(e)
	}
	if e = s.DeleteCancelledBooking(testCtx, c.ID, c.Revision, confirm); !codeIs(e, "NOTIFICATION_BUSY") {
		t.Fatal(e)
	}
	s.DB.Exec(`UPDATE outbox SET state='Sent' WHERE booking_id=$1`, c.ID)
	if e = s.DeleteCancelledBooking(testCtx, c.ID, c.Revision, confirm); e != nil {
		t.Fatal(e)
	}
	if e = s.DeleteCancelledBooking(testCtx, c.ID, c.Revision, confirm); !codeIs(e, "BOOKING_NOT_FOUND") {
		t.Fatal("deleted twice", e)
	}
	if _, e = s.Track(testCtx, one.Token); !codeIs(e, "BOOKING_NOT_FOUND") {
		t.Fatal("deleted link works", e)
	}
	for _, table := range []string{"bookings", "audit", "outbox"} {
		col := "booking_id"
		if table == "bookings" {
			col = "id"
		}
		var n int
		if e = s.DB.QueryRow(`SELECT count(*) FROM `+table+` WHERE `+col+`=$1`, c.ID).Scan(&n); e != nil || n != 0 {
			t.Fatal("data retained", table, n, e)
		}
	}
	if !reflect.DeepEqual(before, latest(t, s, before.ID)) {
		t.Fatal("real parent modified")
	}
	fresh := book(t, s, cleanupInput())
	assign(t, s, Scope{done.Level, done.Date, done.PeriodID, "Math"}, other.Booking.ID, fresh.Booking.ID)
	if *latest(t, s, fresh.Booking.ID).QueueNo != "B-M-001" || *latest(t, s, other.Booking.ID).QueueNo != "B-M-002" {
		t.Fatal("delete reuse wrong")
	}
}

func TestHTTPCancelledDeleteAdminAndCSRF(t *testing.T) {
	s := cleanupService(t)
	c := book(t, s, cleanupInput())
	b, e := s.Status(testCtx, c.Booking.ID, "Cancelled", c.Booking.Revision, "RD1", "test")
	if e != nil {
		t.Fatal(e)
	}
	a, e := NewAuth(s, false)
	if e != nil {
		t.Fatal(e)
	}
	a.Cost = bcrypt.MinCost
	if e = a.SyncLocalStaff(testCtx); e != nil {
		t.Fatal(e)
	}
	s.DB.Exec(`UPDATE users SET role='admin' WHERE username='RD1'`)
	admin, e := a.Login(testCtx, "RD1", "RD1")
	if e != nil {
		t.Fatal(e)
	}
	op, e := a.Login(testCtx, "SM1", "SM1")
	if e != nil {
		t.Fatal(e)
	}
	for _, x := range []Identity{admin, op} {
		if e = a.StoreSession(testCtx, x, ""); e != nil {
			t.Fatal(e)
		}
	}
	h := NewHTTP(s, a, []string{"http://localhost:8080"}, "").Handler()
	post := func(token, csrf, origin string, in any) *httptest.ResponseRecorder {
		data, _ := json.Marshal(in)
		r := httptest.NewRequest("DELETE", "/api/staff/bookings/"+b.ID+"/cancelled", bytes.NewReader(data))
		r.Header.Set("Content-Type", "application/json")
		if token != "" {
			r.AddCookie(&http.Cookie{Name: "warwick_session", Value: token})
		}
		r.Header.Set("X-CSRF-Token", csrf)
		r.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	in := map[string]any{"expectedRevision": b.Revision, "confirmation": "DELETE CANCELLED BOOKING " + b.ID}
	for _, tt := range []struct {
		token, csrf, origin string
		status              int
	}{{"", "", "", 401}, {op.Token, a.csrf(op.Token), "http://localhost:8080", 403}, {admin.Token, "", "http://localhost:8080", 403}, {admin.Token, a.csrf(admin.Token), "https://forged.example", 403}} {
		if w := post(tt.token, tt.csrf, tt.origin, in); w.Code != tt.status {
			t.Fatal(w.Code, tt.status)
		}
	}
	in["all"] = true
	if w := post(admin.Token, a.csrf(admin.Token), "http://localhost:8080", in); w.Code != 400 {
		t.Fatal("bulk field", w.Code)
	}
	delete(in, "all")
	if w := post(admin.Token, a.csrf(admin.Token), "http://localhost:8080", in); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
}
