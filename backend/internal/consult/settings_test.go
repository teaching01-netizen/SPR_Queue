package consult

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// sameStaffState compares the stable part of two staff snapshots. The consult
// timing projection (server clock and estimates) legitimately changes between
// polls and is never stored, so mutation tests ignore it.
func sameStaffState(before, after StaffView) bool {
	before.ServerTime, after.ServerTime = "", ""
	before.Estimates, after.Estimates = nil, nil
	return reflect.DeepEqual(before, after)
}

func metaRevision(t *testing.T, s *Service) int64 {
	t.Helper()
	var revision int64
	if err := s.DB.QueryRowContext(testCtx, `SELECT revision FROM meta WHERE id=1`).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	return revision
}

func TestConsultMinutesSetting(t *testing.T) {
	s := testService(t)
	if minutes, err := s.ConsultMinutes(testCtx); err != nil || minutes != defaultConsultMinutes {
		t.Fatalf("default = %d, %v", minutes, err)
	}
	before := metaRevision(t, s)
	if err := s.SetConsultMinutes(testCtx, 15); err != nil {
		t.Fatal(err)
	}
	if minutes, err := s.ConsultMinutes(testCtx); err != nil || minutes != 15 {
		t.Fatalf("saved = %d, %v", minutes, err)
	}
	// Display configuration only: open queue drafts stay valid.
	if after := metaRevision(t, s); after != before {
		t.Fatalf("setting bumped revision: %d -> %d", before, after)
	}
	view, err := s.StaffList(testCtx)
	if err != nil {
		t.Fatal(err)
	}
	if view.ConsultMinutes != 15 {
		t.Fatalf("staff view minutes = %d", view.ConsultMinutes)
	}
	if _, err := time.Parse(time.RFC3339Nano, view.ServerTime); err != nil {
		t.Fatalf("staff view server time: %v", err)
	}
	for _, invalid := range []int{-5, 0, 61, 500} {
		err := s.SetConsultMinutes(testCtx, invalid)
		var p *Problem
		if !errors.As(err, &p) || p.Status != 400 {
			t.Fatalf("minutes %d accepted: %v", invalid, err)
		}
	}
	// A corrupted or out-of-range stored value falls back to the default so the
	// dashboard keeps rendering instead of failing the whole staff poll.
	for _, stored := range []string{"corrupt", "0", "999", ""} {
		if _, err := s.DB.ExecContext(testCtx, `UPDATE settings SET value=$2 WHERE name=$1`, consultMinutesSetting, stored); err != nil {
			t.Fatal(err)
		}
		if minutes, err := s.ConsultMinutes(testCtx); err != nil || minutes != defaultConsultMinutes {
			t.Fatalf("stored %q fallback = %d, %v", stored, minutes, err)
		}
	}
}

func TestStaffListIncludesConsultTimes(t *testing.T) {
	s := testService(t)
	first := book(t, s, testInput())
	second := book(t, s, testInput())
	assign(t, s, defaultScope(), first.Booking.ID, second.Booking.ID)

	view, err := s.StaffList(testCtx)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Estimates) != 2 {
		t.Fatalf("estimates = %+v", view.Estimates)
	}
	firstAt, err := time.Parse(time.RFC3339Nano, view.Estimates[first.Booking.ID].StartAt)
	if err != nil {
		t.Fatal(err)
	}
	secondAt, err := time.Parse(time.RFC3339Nano, view.Estimates[second.Booking.ID].StartAt)
	if err != nil {
		t.Fatal(err)
	}
	if secondAt.Sub(firstAt) != 10*time.Minute {
		t.Fatalf("spacing = %v, want 10m", secondAt.Sub(firstAt))
	}

	// Parent and public surfaces keep working from the same snapshot without the
	// staff-only timing projection.
	snap, err := s.snapshot(testCtx)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Estimates != nil || snap.ServerTime != "" || snap.ConsultMinutes != 0 {
		t.Fatalf("snapshot leaked staff timing: %+v", snap)
	}
	tracked, err := s.Track(testCtx, first.Token)
	if err != nil {
		t.Fatal(err)
	}
	text, err := encode(tracked)
	if err != nil {
		t.Fatal(err)
	}
	for _, leaked := range []string{"estimates", "serverTime", "consultMinutes"} {
		if strings.Contains(text, leaked) {
			t.Fatalf("parent payload leaked %q: %s", leaked, text)
		}
	}
}

func TestHTTPConsultMinutesSetting(t *testing.T) {
	s := testService(t)
	a, err := NewAuth(s, false)
	if err != nil {
		t.Fatal(err)
	}
	a.Cost = bcrypt.MinCost
	a.dummy, _ = bcrypt.GenerateFromPassword([]byte("dummy-test-password"), bcrypt.MinCost)
	if err = a.Bootstrap(testCtx, "admin", "test-only-password-123", false); err != nil {
		t.Fatal(err)
	}
	h := NewHTTP(s, a, []string{"http://localhost:8080"}, "")
	h.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := h.Handler()
	const origin = "http://localhost:8080"
	send := func(method, path string, value any, cookie *http.Cookie, csrf string) *httptest.ResponseRecorder {
		var buf bytes.Buffer
		if value != nil {
			json.NewEncoder(&buf).Encode(value)
		}
		r := httptest.NewRequest(method, path, &buf)
		r.RemoteAddr = "127.0.0.1:1234"
		r.Header.Set("Content-Type", "application/json")
		if method != "GET" {
			r.Header.Set("Origin", origin)
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
	login := send("POST", "/api/auth/login", map[string]string{"username": "admin", "password": "test-only-password-123"}, nil, "")
	if login.Code != 200 {
		t.Fatal(login.Body.String())
	}
	var session struct {
		CSRF string `json:"csrfToken"`
	}
	json.Unmarshal(login.Body.Bytes(), &session)
	cookie := login.Result().Cookies()[0]
	const path = "/api/staff/settings/consult-minutes"

	if w := send("PUT", path, map[string]int{"minutes": 12}, nil, ""); w.Code != 401 {
		t.Fatalf("anonymous = %d", w.Code)
	}
	if w := send("PUT", path, map[string]int{"minutes": 12}, cookie, ""); w.Code != 403 {
		t.Fatalf("missing CSRF = %d", w.Code)
	}
	if w := send("PUT", path, map[string]any{"minutes": 12, "actor": "forged"}, cookie, session.CSRF); w.Code != 400 {
		t.Fatalf("unknown field = %d", w.Code)
	}
	for _, invalid := range []int{0, 61} {
		if w := send("PUT", path, map[string]int{"minutes": invalid}, cookie, session.CSRF); w.Code != 400 {
			t.Fatalf("minutes %d = %d", invalid, w.Code)
		}
	}
	if _, err = s.DB.ExecContext(testCtx, `UPDATE users SET role='operator' WHERE username='admin'`); err != nil {
		t.Fatal(err)
	}
	if w := send("PUT", path, map[string]int{"minutes": 12}, cookie, session.CSRF); w.Code != 403 {
		t.Fatalf("operator = %d", w.Code)
	}
	if _, err = s.DB.ExecContext(testCtx, `UPDATE users SET role='admin' WHERE username='admin'`); err != nil {
		t.Fatal(err)
	}

	before := metaRevision(t, s)
	w := send("PUT", path, map[string]int{"minutes": 12}, cookie, session.CSRF)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"consultMinutes":12`) {
		t.Fatalf("save = %d %s", w.Code, w.Body.String())
	}
	if after := metaRevision(t, s); after != before {
		t.Fatalf("setting bumped revision: %d -> %d", before, after)
	}
	// The booking page reads the same value from the public event payload.
	event := send("GET", "/api/event", nil, nil, "")
	if event.Code != 200 || !strings.Contains(event.Body.String(), `"consultMinutes":12`) || !strings.Contains(event.Body.String(), `"event":{`) {
		t.Fatalf("event = %d %s", event.Code, event.Body.String())
	}
	// Staff polling receives the value, the server clock and the time estimates.
	list := send("GET", "/api/staff/bookings", nil, cookie, "")
	if list.Code != 200 || !strings.Contains(list.Body.String(), `"consultMinutes":12`) || !strings.Contains(list.Body.String(), `"serverTime":"`) {
		t.Fatalf("staff list = %d %s", list.Code, list.Body.String())
	}
}
