package consult

import (
	"bytes"
	"encoding/json"
	"golang.org/x/crypto/bcrypt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPAuthorizationAndPrivacy(t *testing.T) {
	s := testService(t)
	a, e := NewAuth(s, false)
	if e != nil {
		t.Fatal(e)
	}
	a.Cost = bcrypt.MinCost
	a.dummy, _ = bcrypt.GenerateFromPassword([]byte("dummy-test-password"), bcrypt.MinCost)
	if e = a.Bootstrap(testCtx, "staff", "test-only-password-123", false); e != nil {
		t.Fatal(e)
	}
	h := NewHTTP(s, a, []string{"http://localhost:8080"}, "")
	h.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := h.Handler()
	send := func(method, path string, value any, cookie *http.Cookie, csrf, origin, key string) *httptest.ResponseRecorder {
		var buf bytes.Buffer
		if value != nil {
			json.NewEncoder(&buf).Encode(value)
		}
		r := httptest.NewRequest(method, path, &buf)
		r.RemoteAddr = "127.0.0.1:1234"
		r.Header.Set("Content-Type", "application/json")
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		if csrf != "" {
			r.Header.Set("X-CSRF-Token", csrf)
		}
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	t.Run("anonymous staff access rejected", func(t *testing.T) {
		for _, path := range []string{"/api/staff/bookings", "/api/staff/audit"} {
			if w := send("GET", path, nil, nil, "", "", ""); w.Code != 401 {
				t.Fatal(w.Code)
			}
		}
		if w := send("POST", "/api/staff/bookings/random/actions", map[string]any{"status": "Calling", "expectedRevision": 1}, nil, "", "", ""); w.Code != 401 {
			t.Fatal(w.Code)
		}
	})
	t.Run("unknown field and forged origin rejected", func(t *testing.T) {
		w := send("POST", "/api/bookings", map[string]any{"level": "Beginner", "capacity": 999}, nil, "", "http://localhost:8080", "spoof")
		if w.Code != 400 {
			t.Fatal(w.Code)
		}
		w = send("POST", "/api/auth/login", map[string]string{"username": "staff", "password": "test-only-password-123"}, nil, "", "https://evil.example", "")
		if w.Code != 403 {
			t.Fatal(w.Code)
		}
	})
	login := send("POST", "/api/auth/login", map[string]string{"username": "staff", "password": "test-only-password-123"}, nil, "", "http://localhost:8080", "")
	if login.Code != 200 {
		t.Fatal(login.Body.String())
	}
	var session struct {
		User User   `json:"user"`
		CSRF string `json:"csrfToken"`
	}
	json.Unmarshal(login.Body.Bytes(), &session)
	cookies := login.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("no session cookie")
	}
	cookie := cookies[0]
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatal("cookie flags")
	}
	t.Run("me restores CSRF and staff identity", func(t *testing.T) {
		w := send("GET", "/api/auth/me", nil, cookie, "", "", "")
		if w.Code != 200 || !strings.Contains(w.Body.String(), session.CSRF) {
			t.Fatal(w.Body.String())
		}
	})
	t.Run("mutation without CSRF rejected", func(t *testing.T) {
		w := send("PUT", "/api/staff/queue-order", map[string]any{}, cookie, "", "http://localhost:8080", "")
		if w.Code != 403 {
			t.Fatal(w.Code)
		}
	})
	created := send("POST", "/api/bookings", testInput(), nil, "", "http://localhost:8080", "http-book")
	if created.Code != 201 {
		t.Fatal(created.Body.String())
	}
	var confirmation Confirmation
	json.Unmarshal(created.Body.Bytes(), &confirmation)
	t.Run("staff actor cannot be forged", func(t *testing.T) {
		v, _ := s.StaffList(testCtx)
		w := send("PUT", "/api/staff/queue-order", map[string]any{"scope": defaultScope(), "bookingIds": []string{confirmation.Booking.ID}, "expectedRevision": v.Revision, "staff": "Attacker"}, cookie, session.CSRF, "http://localhost:8080", "")
		if w.Code != 400 {
			t.Fatal(w.Code)
		}
		w = send("PUT", "/api/staff/queue-order", map[string]any{"scope": defaultScope(), "bookingIds": []string{confirmation.Booking.ID}, "expectedRevision": v.Revision}, cookie, session.CSRF, "http://localhost:8080", "")
		if w.Code != 200 {
			t.Fatal(w.Body.String())
		}
		rows, _ := s.Audit(testCtx)
		if rows[0].Actor != "staff" {
			t.Fatal("untrusted actor")
		}
	})
	t.Run("parent and public display exclude private data", func(t *testing.T) {
		for _, path := range []string{"/api/track/" + confirmation.Token, "/api/display?level=Beginner&date=2026-08-15"} {
			w := send("GET", path, nil, nil, "", "", "")
			if w.Code != 200 || strings.Contains(w.Body.String(), "parent@example.com") || strings.Contains(w.Body.String(), "School") {
				t.Fatal(w.Body.String())
			}
			if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Frame-Options") != "DENY" {
				t.Fatal("missing security headers")
			}
		}
		w := send("GET", "/api/track/invalid", nil, nil, "", "", "")
		if w.Code != 404 || strings.Contains(w.Body.String(), confirmation.Booking.ID) {
			t.Fatal("invalid token disclosure")
		}
	})
	t.Run("staff list excludes tokens", func(t *testing.T) {
		w := send("GET", "/api/staff/bookings", nil, cookie, "", "", "")
		if w.Code != 200 || strings.Contains(w.Body.String(), confirmation.Token) {
			t.Fatal(w.Body.String())
		}
	})
	t.Run("start subject uses staff auth, CSRF, validation and revision", func(t *testing.T) {
		i := testInput()
		i.Subject = "Combined"
		i.PeriodID = "P1300"
		b := book(t, s, i).Booking
		path := "/api/staff/bookings/" + b.ID + "/start-subject"
		payload := map[string]any{"stream": "Verbal", "expectedRevision": b.Revision}
		if w := send("POST", path, payload, nil, "", "http://localhost:8080", ""); w.Code != 401 {
			t.Fatal(w.Code)
		}
		if w := send("POST", path, payload, cookie, "", "http://localhost:8080", ""); w.Code != 403 {
			t.Fatal(w.Code)
		}
		if w := send("POST", path, payload, cookie, session.CSRF, "https://evil.example", ""); w.Code != 403 {
			t.Fatal(w.Code)
		}
		payload["actor"] = "forged"
		if w := send("POST", path, payload, cookie, session.CSRF, "http://localhost:8080", ""); w.Code != 400 {
			t.Fatal(w.Code)
		}
		delete(payload, "actor")
		w := send("POST", path, payload, cookie, session.CSRF, "http://localhost:8080", "")
		if w.Code != 200 {
			t.Fatal(w.Body.String())
		}
		if w := send("POST", path, payload, cookie, session.CSRF, "http://localhost:8080", ""); w.Code != 409 {
			t.Fatal(w.Code)
		}
		changed := latest(t, s, b.ID)
		if changed.CurrentSubject != "Verbal" || changed.Staff != "staff" {
			t.Fatal(changed)
		}
	})
	t.Run("edit preflight advertises PATCH", func(t *testing.T) {
		w := send("OPTIONS", "/api/staff/bookings/sample", nil, nil, "", "http://localhost:8080", "")
		if w.Code != 204 || !strings.Contains(w.Header().Get("Access-Control-Allow-Methods"), "PATCH") {
			t.Fatal(w.Code, w.Header())
		}
	})
	t.Run("operators can edit cancel and restore with protected requests", func(t *testing.T) {
		if _, err := s.DB.Exec(`UPDATE users SET role='operator' WHERE username='staff'`); err != nil {
			t.Fatal(err)
		}
		defer s.DB.Exec(`UPDATE users SET role='admin' WHERE username='staff'`)
		b := book(t, s, testInputForPeriod("P1800")).Booking
		path := "/api/staff/bookings/" + b.ID
		in := editValues(b)
		in.Subject = "Verbal"
		in.PeriodID = "P1900"
		if w := send("PATCH", path, in, nil, "", "http://localhost:8080", ""); w.Code != 401 {
			t.Fatal(w.Code)
		}
		if w := send("PATCH", path, in, cookie, "", "http://localhost:8080", ""); w.Code != 403 {
			t.Fatal(w.Code)
		}
		if w := send("PATCH", path, in, cookie, session.CSRF, "https://evil.example", ""); w.Code != 403 {
			t.Fatal(w.Code)
		}
		forged := map[string]any{"subject": "Verbal", "date": b.Date, "periodId": "P1900", "seminarConfirmed": true, "expectedRevision": b.Revision, "level": "High Level"}
		if w := send("PATCH", path, forged, cookie, session.CSRF, "http://localhost:8080", ""); w.Code != 400 {
			t.Fatal(w.Code)
		}
		w := send("PATCH", path, in, cookie, session.CSRF, "http://localhost:8080", "")
		if w.Code != 200 {
			t.Fatal(w.Body.String())
		}
		b = latest(t, s, b.ID)
		if b.Subject != "Verbal" || b.Staff != "staff" {
			t.Fatal(b)
		}
		b = cancelBooking(t, s, b)
		payload := map[string]any{"expectedRevision": b.Revision}
		if w := send("POST", path+"/restore", payload, nil, "", "http://localhost:8080", ""); w.Code != 401 {
			t.Fatal(w.Code)
		}
		if w := send("POST", path+"/restore", payload, cookie, "", "http://localhost:8080", ""); w.Code != 403 {
			t.Fatal(w.Code)
		}
		if w := send("POST", path+"/restore", payload, cookie, session.CSRF, "https://evil.example", ""); w.Code != 403 {
			t.Fatal(w.Code)
		}
		payload["staff"] = "forged"
		if w := send("POST", path+"/restore", payload, cookie, session.CSRF, "http://localhost:8080", ""); w.Code != 400 {
			t.Fatal(w.Code)
		}
		delete(payload, "staff")
		if w := send("POST", path+"/restore", payload, cookie, session.CSRF, "http://localhost:8080", ""); w.Code != 200 {
			t.Fatal(w.Body.String())
		}
		if latest(t, s, b.ID).Status != "Booked" {
			t.Fatal("restore did not persist")
		}
	})
	t.Run("logout revokes session", func(t *testing.T) {
		if w := send("POST", "/api/auth/logout", map[string]any{}, cookie, session.CSRF, "http://localhost:8080", ""); w.Code != 200 {
			t.Fatal(w.Code)
		}
		if w := send("GET", "/api/staff/bookings", nil, cookie, "", "", ""); w.Code != 401 {
			t.Fatal("session still valid")
		}
	})
}
