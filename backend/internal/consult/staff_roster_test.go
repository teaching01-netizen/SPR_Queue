package consult

import (
	"bytes"
	"encoding/json"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func rosterService(t *testing.T) *Service {
	t.Helper()
	s := novemberService(t)
	s.Config.Event.ID = "local-test-" + s.Config.Event.ID
	if err := s.migrate(testCtx); err != nil {
		t.Fatal(err)
	}
	return s
}
func TestSevenLocalStaffAndSessionPreservation(t *testing.T) {
	s := rosterService(t)
	a, err := NewAuth(s, false)
	if err != nil {
		t.Fatal(err)
	}
	a.Cost = bcrypt.MinCost
	if err = a.Bootstrap(testCtx, "staff", "old-test-password-123", false); err != nil {
		t.Fatal(err)
	}
	old, err := a.Login(testCtx, "staff", "old-test-password-123")
	if err != nil {
		t.Fatal(err)
	}
	if err = a.StoreSession(testCtx, old, ""); err != nil {
		t.Fatal(err)
	}
	i := testInput()
	i.Date = "2026-11-07"
	i.PeriodID = "P1800_2000"
	booking := book(t, s, i).Booking
	if err = a.SyncLocalStaff(testCtx); err != nil {
		t.Fatal(err)
	}
	var count int
	s.DB.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	if count != 7 {
		t.Fatal("not exactly seven", count)
	}
	s.DB.QueryRow(`SELECT COUNT(*) FROM sessions WHERE username='staff'`).Scan(&count)
	if count != 0 {
		t.Fatal("old staff session remains")
	}
	if _, err = a.Login(testCtx, "staff", "old-test-password-123"); !codeIs(err, "INVALID_CREDENTIALS") {
		t.Fatal("old account active", err)
	}
	for _, name := range localStaffNames {
		id, err := a.Login(testCtx, name, name)
		expectedRole := "operator"
		if name == "RD1" {
			expectedRole = "admin"
		}
		if err != nil || id.User.Username != name || id.User.Role != expectedRole {
			t.Fatal(name, err)
		}
		var hash string
		s.DB.QueryRow(`SELECT password_hash FROM users WHERE username=$1`, name).Scan(&hash)
		if hash == name || !strings.HasPrefix(hash, "$2") {
			t.Fatal("plaintext password")
		}
		if _, err = a.Login(testCtx, name, "wrong"); !codeIs(err, "INVALID_CREDENTIALS") {
			t.Fatal("wrong password accepted", err)
		}
	}
	id, err := a.Login(testCtx, "RD1", "RD1")
	if err != nil {
		t.Fatal(err)
	}
	if err = a.StoreSession(testCtx, id, ""); err != nil {
		t.Fatal(err)
	}
	if err = a.SyncLocalStaff(testCtx); err != nil {
		t.Fatal(err)
	}
	s.DB.QueryRow(`SELECT COUNT(*) FROM sessions WHERE token_hash=$1`, digest(id.Token)).Scan(&count)
	if count != 1 {
		t.Fatal("repeat startup revoked correct session")
	}
	if latest(t, s, booking.ID).Revision != booking.Revision {
		t.Fatal("roster changed booking")
	}
	s.Config.Event.Sample = false
	if _, err = a.Login(testCtx, "RD1", "RD1"); !codeIs(err, "INVALID_CREDENTIALS") {
		t.Fatal("fixed password accepted outside local sample", err)
	}
	if err = a.SyncLocalStaff(testCtx); !codeIs(err, "LOCAL_STAFF_NOT_ALLOWED") {
		t.Fatal("non-sample roster allowed", err)
	}
	s.Config.Event.Sample = true
	s.Config.Event.ID = "production-nov"
	if err = a.SyncLocalStaff(testCtx); !codeIs(err, "LOCAL_STAFF_NOT_ALLOWED") {
		t.Fatal("non-local roster allowed", err)
	}
}
func TestAllSevenStaffHTTPCheckInAndAudit(t *testing.T) {
	s := rosterService(t)
	a, err := NewAuth(s, false)
	if err != nil {
		t.Fatal(err)
	}
	a.Cost = bcrypt.MinCost
	if err = a.SyncLocalStaff(testCtx); err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for range localStaffNames {
		i := testInput()
		i.Date = "2026-11-07"
		i.PeriodID = "P1800_2000"
		ids = append(ids, book(t, s, i).Booking.ID)
	}
	assign(t, s, Scope{"Beginner", "2026-11-07", "P1800_2000", "Math"}, ids...)
	handler := NewHTTP(s, a, []string{"http://localhost:8080"}, "").Handler()
	send := func(path string, in any, cookie *http.Cookie, csrf string) *httptest.ResponseRecorder {
		var buf bytes.Buffer
		json.NewEncoder(&buf).Encode(in)
		r := httptest.NewRequest("POST", path, &buf)
		r.RemoteAddr = "127.0.0.1:1234"
		r.Header.Set("Origin", "http://localhost:8080")
		r.Header.Set("Content-Type", "application/json")
		if cookie != nil {
			r.AddCookie(cookie)
		}
		if csrf != "" {
			r.Header.Set("X-CSRF-Token", csrf)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	for n, name := range localStaffNames {
		login := send("/api/auth/login", map[string]string{"username": name, "password": name}, nil, "")
		if login.Code != 200 {
			t.Fatal(name, login.Body.String())
		}
		var session struct {
			User User   `json:"user"`
			CSRF string `json:"csrfToken"`
		}
		if err = json.Unmarshal(login.Body.Bytes(), &session); err != nil {
			t.Fatal(err)
		}
		b := latest(t, s, ids[n])
		w := send("/api/staff/bookings/"+b.ID+"/actions", map[string]any{"status": "Waiting", "expectedRevision": b.Revision}, login.Result().Cookies()[0], session.CSRF)
		if w.Code != 200 {
			t.Fatal(name, w.Body.String())
		}
		if latest(t, s, b.ID).Staff != name {
			t.Fatal("wrong booking actor")
		}
		var actor string
		s.DB.QueryRow(`SELECT actor FROM audit WHERE booking_id=$1 AND action='STATUS_Waiting' ORDER BY id DESC LIMIT 1`, b.ID).Scan(&actor)
		if actor != name {
			t.Fatal("wrong audit actor", actor)
		}
	}
}
