package consult

import (
	"bytes"
	"encoding/json"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func swapInput(a, b Booking) BookingSwap {
	return BookingSwap{OtherID: b.ID, Revision: a.Revision, OtherRevision: b.Revision, Reason: "ทั้งสองครอบครัวยืนยันสลับ", Confirmation: "SWAP BOOKINGS " + a.ID + " " + b.ID}
}
func swapPair(t *testing.T, s *Service) (Confirmation, Confirmation) {
	t.Helper()
	i := cleanupInput()
	i.PeriodID = "P1300_1400"
	i.Nickname = "Parent A"
	a := book(t, s, i)
	i.PeriodID = "P1700_1800"
	i.Nickname = "Parent B"
	return a, book(t, s, i)
}

func TestSwapFullPeriodsAtomicIdentityMailAndAudit(t *testing.T) {
	s := cleanupService(t)
	a, b := swapPair(t, s)
	ids := [][]string{{a.Booking.ID}, {b.Booking.ID}}
	others := []Booking{}
	for n, p := range []string{"P1300_1400", "P1700_1800"} {
		for j := 0; j < 3; j++ {
			i := cleanupInput()
			i.PeriodID = p
			c := book(t, s, i)
			ids[n] = append(ids[n], c.Booking.ID)
		}
	}
	for n, p := range []string{"P1300_1400", "P1700_1800"} {
		assign(t, s, Scope{"Beginner", "2026-11-07", p, "Math"}, ids[n]...)
		i := cleanupInput()
		i.PeriodID = p
		if _, e := s.Book(testCtx, "still-full-"+p, i); !codeIs(e, "PERIOD_FULL") {
			t.Fatal(e)
		}
	}
	aa := transition(t, s, a.Booking.ID, "Waiting")
	bb := transition(t, s, b.Booking.ID, "Waiting")
	for _, group := range ids {
		for _, id := range group[1:] {
			others = append(others, latest(t, s, id))
		}
	}
	before, e := s.snapshot(testCtx)
	if e != nil {
		t.Fatal(e)
	}
	s.Config.Event.Sample = false
	s.MailEnabled = true
	for _, id := range []string{aa.ID, bb.ID} {
		if _, e = s.DB.Exec(`INSERT INTO outbox(booking_id,state) VALUES($1,'Sent')`, id); e != nil {
			t.Fatal(e)
		}
	}
	out, e := s.SwapBookings(testCtx, aa.ID, swapInput(aa, bb), "SM1")
	if e != nil {
		t.Fatal(e)
	}
	for n, original := range []Booking{aa, bb} {
		x := out[n]
		destination := []Booking{bb, aa}[n]
		if x.ID != original.ID || x.Nickname != original.Nickname || x.Email != original.Email || x.Wcode != original.Wcode || !reflect.DeepEqual(x.Questions, original.Questions) || *x.QueueNo != *original.QueueNo || x.PeriodID != destination.PeriodID || x.Status != "Waiting" || *x.CheckedInAt != *original.CheckedInAt || len(x.Orders) != 0 || len(x.Stages) != 0 || x.Counter != "" || x.Revision != original.Revision+1 {
			t.Fatal("wrong identity/state", x)
		}
		var state string
		if e = s.DB.QueryRow(`SELECT state FROM outbox WHERE booking_id=$1`, x.ID).Scan(&state); e != nil || state != "Pending" {
			t.Fatal("updated email missing", e, state)
		}
	}
	for _, c := range others {
		if !reflect.DeepEqual(c, latest(t, s, c.ID)) {
			t.Fatal("other parent changed")
		}
	}
	for _, c := range []Confirmation{a, b} {
		link, e := s.StaffTrackingLink(testCtx, c.Booking.ID)
		if e != nil || link.TrackingURL != c.TrackingURL {
			t.Fatal("link changed", e)
		}
		if _, e = s.Track(testCtx, c.Token); e != nil {
			t.Fatal(e)
		}
	}
	after, e := s.snapshot(testCtx)
	if e != nil || after.Revision != before.Revision+1 {
		t.Fatal("not one atomic mutation", e)
	}
	for _, p := range []string{"P1300_1400", "P1700_1800"} {
		var count int
		if e = s.DB.QueryRow(`SELECT count(*) FROM bookings WHERE event_id=$1 AND period_id=$2 AND status!='Cancelled'`, s.Config.Event.ID, p).Scan(&count); e != nil || count != 4 {
			t.Fatal("capacity changed", count, e)
		}
	}
	audit, e := s.Audit(testCtx)
	if e != nil {
		t.Fatal(e)
	}
	count := 0
	for _, x := range audit {
		if x.Action == "SWAP_BOOKING_PERIOD" {
			count++
			if x.Actor != "SM1" || !strings.Contains(x.Next, "ทั้งสองครอบครัว") || !strings.Contains(x.Next, "otherBookingId") {
				t.Fatal("bad swap audit")
			}
		}
	}
	if count != 2 {
		t.Fatal("missing pair audit", count)
	}
	snap, e := s.sheetSnapshot(testCtx)
	if e != nil || len(snap.Rows) != 8 {
		t.Fatal("sheets snapshot", e)
	}
}

func TestSwapFailureOnSecondWriteRollsBackBothAndNotifications(t *testing.T) {
	s := cleanupService(t)
	a, b := swapPair(t, s)
	aa, bb := a.Booking, b.Booking
	s.MailEnabled = true
	for _, id := range []string{aa.ID, bb.ID} {
		s.DB.Exec(`INSERT INTO outbox(booking_id,state) VALUES($1,'Sent')`, id)
	}
	// Inject a database error after A has already been updated and its outbox queued.
	if _, e := s.DB.Exec(`CREATE FUNCTION reject_second_swap() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.data->>'nickname'='Parent B' AND NEW.period_id='P1300_1400' THEN RAISE EXCEPTION 'fixture second write failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER fixture_swap_failure BEFORE UPDATE ON bookings FOR EACH ROW EXECUTE FUNCTION reject_second_swap()`); e != nil {
		t.Fatal(e)
	}
	v, _ := s.snapshot(testCtx)
	if _, e := s.SwapBookings(testCtx, aa.ID, swapInput(aa, bb), "SM1"); e == nil {
		t.Fatal("injected failure ignored")
	}
	if !reflect.DeepEqual(aa, latest(t, s, aa.ID)) || !reflect.DeepEqual(bb, latest(t, s, bb.ID)) {
		t.Fatal("partial swap")
	}
	after, _ := s.snapshot(testCtx)
	if after.Revision != v.Revision {
		t.Fatal("failed swap bumped revision")
	}
	for _, id := range []string{aa.ID, bb.ID} {
		var state string
		s.DB.QueryRow(`SELECT state FROM outbox WHERE booking_id=$1`, id).Scan(&state)
		if state != "Sent" {
			t.Fatal("failed swap queued mail")
		}
	}
	var audits int
	s.DB.QueryRow(`SELECT count(*) FROM audit WHERE action='SWAP_BOOKING_PERIOD'`).Scan(&audits)
	if audits != 0 {
		t.Fatal("failed swap audit committed")
	}
}

func TestSwapValidationRevisionBusyAndActiveGuards(t *testing.T) {
	s := cleanupService(t)
	a, b := swapPair(t, s)
	aa, bb := a.Booking, b.Booking
	for _, change := range []func(*BookingSwap){func(i *BookingSwap) { i.OtherID = aa.ID }, func(i *BookingSwap) { i.Reason = "" }, func(i *BookingSwap) { i.Confirmation = "wrong" }, func(i *BookingSwap) { i.Revision-- }, func(i *BookingSwap) { i.OtherRevision-- }} {
		in := swapInput(aa, bb)
		change(&in)
		if _, e := s.SwapBookings(testCtx, aa.ID, in, "SM1"); e == nil {
			t.Fatal("bad input accepted")
		}
	}
	i := cleanupInput()
	i.PeriodID = aa.PeriodID
	same := book(t, s, i).Booking
	if _, e := s.SwapBookings(testCtx, aa.ID, swapInput(aa, same), "SM1"); !codeIs(e, "SWAP_SAME_PERIOD") {
		t.Fatal(e)
	}
	i.Subject = "Verbal"
	i.PeriodID = "P1700_1800"
	otherSubject := book(t, s, i).Booking
	if _, e := s.SwapBookings(testCtx, aa.ID, swapInput(aa, otherSubject), "SM1"); !codeIs(e, "SWAP_SCOPE_MISMATCH") {
		t.Fatal(e)
	}
	original := s.Config.Event.ID
	s.Config.Event.ID = "wrong-event"
	if _, e := s.SwapBookings(testCtx, aa.ID, swapInput(aa, bb), "SM1"); !codeIs(e, "BOOKING_NOT_FOUND") {
		t.Fatal(e)
	}
	s.Config.Event.ID = original
	if _, e := s.DB.Exec(`INSERT INTO outbox(booking_id,state) VALUES($1,'Sending')`, bb.ID); e != nil {
		t.Fatal(e)
	}
	if _, e := s.SwapBookings(testCtx, aa.ID, swapInput(aa, bb), "SM1"); !codeIs(e, "NOTIFICATION_BUSY") {
		t.Fatal(e)
	}
	s.DB.Exec(`DELETE FROM outbox WHERE booking_id=$1`, bb.ID)
	if !reflect.DeepEqual(aa, latest(t, s, aa.ID)) || !reflect.DeepEqual(bb, latest(t, s, bb.ID)) {
		t.Fatal("failed validation changed data")
	}
	bb = cancelBooking(t, s, bb)
	if _, e := s.SwapBookings(testCtx, aa.ID, swapInput(aa, bb), "SM1"); !codeIs(e, "SWAP_NOT_ALLOWED") {
		t.Fatal(e)
	}
	bb, e := s.RestoreBooking(testCtx, bb.ID, bb.Revision, "SM1")
	if e != nil {
		t.Fatal(e)
	}
	aa = transition(t, s, aa.ID, "Waiting")
	assign(t, s, Scope{aa.Level, aa.Date, aa.PeriodID, "Math"}, aa.ID, same.ID)
	aa = transition(t, s, aa.ID, "Calling")
	if _, e = s.SwapBookings(testCtx, aa.ID, swapInput(aa, bb), "SM1"); !codeIs(e, "SWAP_NOT_ALLOWED") {
		t.Fatal("called booking moved", e)
	}
	aa = transition(t, s, aa.ID, "Serving")
	if _, e = s.SwapBookings(testCtx, aa.ID, swapInput(aa, bb), "SM1"); !codeIs(e, "SWAP_NOT_ALLOWED") {
		t.Fatal("serving moved", e)
	}
	aa = transition(t, s, aa.ID, "Completed")
	if _, e = s.SwapBookings(testCtx, aa.ID, swapInput(aa, bb), "SM1"); !codeIs(e, "SWAP_NOT_ALLOWED") {
		t.Fatal("completed moved", e)
	}
}

func TestSwapCombinedKeepsChosenFirstRoomAndDateChangeResetsCheckin(t *testing.T) {
	s := cleanupService(t)
	i := cleanupInput()
	i.Subject = "Combined"
	i.PeriodID = "P1400_1530"
	a := book(t, s, i).Booking
	i.PeriodID = "P1530_1700"
	b := book(t, s, i).Booking
	a, e := s.ChooseStartSubject(testCtx, a.ID, "Verbal", a.Revision, "SM1")
	if e != nil {
		t.Fatal(e)
	}
	a = transition(t, s, a.ID, "Waiting")
	b = transition(t, s, b.ID, "Waiting")
	out, e := s.SwapBookings(testCtx, a.ID, swapInput(a, b), "SM1")
	if e != nil || out[0].CurrentSubject != "Verbal" || out[1].CurrentSubject != "Math" || out[0].CheckedInAt == nil {
		t.Fatal("combined first room", e)
	}
	// Add an isolated second-day rule to exercise the general date-change path.
	rule := s.Config.Rules[0]
	rule.Date = "2026-11-08"
	s.Config.Rules = append(s.Config.Rules, rule)
	if e = s.migrate(testCtx); e != nil {
		t.Fatal(e)
	}
	i = cleanupInput()
	i.PeriodID = "P1300_1400"
	c := book(t, s, i).Booking
	i.Date = "2026-11-08"
	d := book(t, s, i).Booking
	c = transition(t, s, c.ID, "Waiting")
	d = transition(t, s, d.ID, "Waiting")
	out, e = s.SwapBookings(testCtx, c.ID, swapInput(c, d), "SM1")
	if e != nil {
		t.Fatal(e)
	}
	for _, x := range out {
		if x.Status != "Booked" || x.CheckedInAt != nil {
			t.Fatal("old-date checkin survived")
		}
	}
}

func TestSwapConcurrentRequestsChangePairOnlyOnce(t *testing.T) {
	s := cleanupService(t)
	a, b := swapPair(t, s)
	in := swapInput(a.Booking, b.Booking)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for n := range errs {
		wg.Add(1)
		go func(n int) { defer wg.Done(); _, errs[n] = s.SwapBookings(testCtx, a.Booking.ID, in, "SM1") }(n)
	}
	wg.Wait()
	success, conflict := 0, 0
	for _, e := range errs {
		if e == nil {
			success++
		} else if codeIs(e, "REVISION_CONFLICT") {
			conflict++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal(success, conflict)
	}
	if latest(t, s, a.Booking.ID).PeriodID != b.Booking.PeriodID || latest(t, s, b.Booking.ID).PeriodID != a.Booking.PeriodID {
		t.Fatal("double swap")
	}
}

func TestHTTPSwapStaffCSRFAndPairConfirmation(t *testing.T) {
	s := cleanupService(t)
	a, b := swapPair(t, s)
	auth, e := NewAuth(s, false)
	if e != nil {
		t.Fatal(e)
	}
	auth.Cost = bcrypt.MinCost
	if e = auth.SyncLocalStaff(testCtx); e != nil {
		t.Fatal(e)
	}
	op, e := auth.Login(testCtx, "SM1", "SM1")
	if e != nil {
		t.Fatal(e)
	}
	if e = auth.StoreSession(testCtx, op, ""); e != nil {
		t.Fatal(e)
	}
	h := NewHTTP(s, auth, []string{"http://localhost:8080"}, "").Handler()
	in := swapInput(a.Booking, b.Booking)
	post := func(login, csrf bool, origin string, payload any) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(payload)
		r := httptest.NewRequest("POST", "/api/staff/bookings/"+a.Booking.ID+"/swap", bytes.NewReader(raw))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", origin)
		if login {
			r.AddCookie(&http.Cookie{Name: "warwick_session", Value: op.Token})
		}
		if csrf {
			r.Header.Set("X-CSRF-Token", auth.csrf(op.Token))
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	for _, x := range []struct {
		login, csrf bool
		origin      string
		status      int
	}{{false, false, "", 401}, {true, false, "http://localhost:8080", 403}, {true, true, "https://forged.example", 403}} {
		if w := post(x.login, x.csrf, x.origin, in); w.Code != x.status {
			t.Fatal(w.Code, x.status)
		}
	}
	if w := post(true, true, "http://localhost:8080", map[string]any{"all": true}); w.Code != 400 {
		t.Fatal("bulk accepted", w.Code)
	}
	if w := post(true, true, "http://localhost:8080", in); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := post(true, true, "http://localhost:8080", in); w.Code != 409 {
		t.Fatal("stale pair accepted", w.Code)
	}
}
