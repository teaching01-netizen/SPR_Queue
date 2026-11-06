package consult

import (
	"encoding/json"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestStaffTrackingLinkPreservesOriginalAcrossStates(t *testing.T) {
	s := cleanupService(t)
	c := book(t, s, cleanupInput())
	check := func() {
		t.Helper()
		before, err := s.StaffList(testCtx)
		if err != nil {
			t.Fatal(err)
		}
		link, err := s.StaffTrackingLink(testCtx, c.Booking.ID)
		if err != nil || link.TrackingURL != c.TrackingURL {
			t.Fatal("did not recover original link", err)
		}
		after, err := s.StaffList(testCtx)
		if err != nil || !reflect.DeepEqual(before, after) {
			t.Fatal("recovery changed booking/queue/revision", err)
		}
		if _, err = s.Track(testCtx, c.Token); err != nil {
			t.Fatal("old link stopped working", err)
		}
		raw, _ := json.Marshal(after)
		if strings.Contains(string(raw), c.Token) || strings.Contains(string(raw), "trackingUrl") {
			t.Fatal("link leaked into staff list")
		}
	}
	check()
	b, err := s.Status(testCtx, c.Booking.ID, "Cancelled", c.Booking.Revision, "RD1", "Test cancellation")
	if err != nil {
		t.Fatal(err)
	}
	check()
	if _, err := s.RestoreBooking(testCtx, b.ID, BookingRestore{Revision: b.Revision}, "RD1"); err != nil {
		t.Fatal(err)
	}
	check()
	done := finishCleanup(t, s, c)
	if _, err := s.ArchiveCompleted(testCtx, done.ID, true, done.Revision, "RD1"); err != nil {
		t.Fatal(err)
	}
	check()
	s.LocalTestDataManagement = false
	s.Config.Event.Sample = false
	check()
	s.LocalTestDataManagement = true
	s.Config.Event.Sample = true
	originalEvent := s.Config.Event.ID
	s.Config.Event.ID = "another-event"
	if _, err := s.StaffTrackingLink(testCtx, c.Booking.ID); !codeIs(err, "BOOKING_NOT_FOUND") {
		t.Fatal("other-event link exposed", err)
	}
	s.Config.Event.ID = originalEvent
	current := latest(t, s, c.Booking.ID)
	if err := s.DeleteTestBooking(testCtx, current.ID, current.Revision, "DELETE TEST BOOKING"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.StaffTrackingLink(testCtx, current.ID); !codeIs(err, "BOOKING_NOT_FOUND") {
		t.Fatal("deleted link recovered", err)
	}
}

func TestHTTPStaffTrackingLinkAuthenticationAndPrivacy(t *testing.T) {
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
	id, err := a.Login(testCtx, "RD1", "RD1")
	if err != nil {
		t.Fatal(err)
	}
	if err = a.StoreSession(testCtx, id, ""); err != nil {
		t.Fatal(err)
	}
	h := NewHTTP(s, a, []string{"http://localhost:8080"}, "").Handler()
	path := "/api/staff/bookings/" + c.Booking.ID + "/tracking-link"
	get := func(path, origin string, login bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if login {
			r.AddCookie(&http.Cookie{Name: "warwick_session", Value: id.Token})
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := get(path, "", false); w.Code != 401 || strings.Contains(w.Body.String(), c.Token) {
		t.Fatal("anonymous disclosure", w.Code)
	}
	if w := get(path, "https://forged.example", true); w.Code != 403 {
		t.Fatal("forged origin", w.Code)
	}
	w := get(path, "http://localhost:8080", true)
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(w.Code, w.Body.String())
	}
	var out map[string]string
	if err = json.Unmarshal(w.Body.Bytes(), &out); err != nil || len(out) != 1 || out["trackingUrl"] != c.TrackingURL {
		t.Fatal("wrong private response", err)
	}
	s.Config.Event.ID = "another-event"
	if w = get(path, "", true); w.Code != 404 || strings.Contains(w.Body.String(), c.Token) {
		t.Fatal("wrong-event disclosure", w.Code)
	}
	s.Config.Event.ID = c.Booking.EventID
	if _, err = s.DB.Exec(`UPDATE bookings SET token_cipher='invalid' WHERE id=$1`, c.Booking.ID); err != nil {
		t.Fatal(err)
	}
	if w = get(path, "", true); w.Code != 500 || strings.Contains(w.Body.String(), "invalid") || strings.Contains(w.Body.String(), c.Token) {
		t.Fatal("corruption disclosure", w.Code)
	}
	if _, err = s.DB.Exec(`DELETE FROM sessions`); err != nil {
		t.Fatal(err)
	}
	if w = get(path, "", true); w.Code != 401 {
		t.Fatal("expired session allowed", w.Code)
	}
}
