package consult

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestResendHTTPSMailerTemplateAndIdempotency(t *testing.T) {
	cfg := testConfig(t)
	b := Booking{ID: "private-booking", EventID: cfg.Event.ID, Input: testInput()}
	tracking := "https://consult.example/#/track/owned-token"
	var keys []string
	var bodies []map[string]any
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.Header.Get("Authorization") != "Bearer fixture-api-key" {
			t.Error("incorrect API authorization")
		}
		var payload map[string]any
		json.NewDecoder(r.Body).Decode(&payload)
		bodies = append(bodies, payload)
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"id":"accepted-message"}`)
	}))
	defer server.Close()
	m := ResendMailer{APIKey: "fixture-api-key", From: "Warwick <consult@example.com>", Config: cfg, endpoint: server.URL, client: server.Client()}
	for n := 0; n < 2; n++ {
		if e := m.Send(context.Background(), b, tracking); e != nil {
			t.Fatal(e)
		}
	}
	if keys[0] == "" || keys[0] != keys[1] {
		t.Fatal("retry is not idempotent")
	}
	if !strings.Contains(bodies[0]["html"].(string), tracking) || !strings.Contains(bodies[0]["text"].(string), tracking) {
		t.Fatal("private link missing from email")
	}
	b.Nickname = "Updated student"
	if e := m.Send(context.Background(), b, tracking); e != nil {
		t.Fatal(e)
	}
	if keys[2] == keys[0] {
		t.Fatal("updated confirmation suppressed")
	}
}
func TestResendHTTPSMailerFailuresDoNotLeakSecrets(t *testing.T) {
	cfg := testConfig(t)
	b := Booking{ID: "test", Input: testInput()}
	for _, status := range []int{302, 401, 429, 500, 200} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			io.WriteString(w, `{"error":"fixture-api-key private-parent"}`)
		}))
		m := ResendMailer{APIKey: "fixture-api-key", From: "consult@example.com", Config: cfg, endpoint: server.URL}
		err := m.Send(context.Background(), b, "https://consult.example/#/track/owned-token")
		server.Close()
		if err == nil || strings.Contains(err.Error(), "fixture-api-key") || strings.Contains(err.Error(), "private-parent") {
			t.Fatal("invalid response accepted or leaked", err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	m := ResendMailer{APIKey: "fixture-api-key", From: "consult@example.com", Config: cfg}
	if e := m.Send(ctx, b, "https://consult.example/#/track/owned-token"); e == nil {
		t.Fatal("timeout ignored")
	}
}

func TestCurrentRegistrationQueuesHTTPSConfirmation(t *testing.T) {
	s := novemberService(t)
	i := cleanupInput()
	i.Subject = "Combined"
	i.PeriodID = "P1400_1530"
	// Disabled delivery must remain explicit to the parent and queue no work.
	off := book(t, s, i)
	var count int
	if err := s.DB.QueryRow(`SELECT count(*) FROM outbox`).Scan(&count); err != nil || count != 0 || off.Booking.EmailStatus != "Not configured" {
		t.Fatal("disabled mail unexpectedly queued work", err)
	}
	s.MailEnabled = true
	on := book(t, s, i)
	if on.Booking.EmailStatus != "Pending" {
		t.Fatal("registration did not queue confirmation")
	}
	requests := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		var payload struct {
			To   []string `json:"to"`
			HTML string   `json:"html"`
			Text string   `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		if len(payload.To) != 1 || payload.To[0] != i.Email || !strings.Contains(payload.HTML, on.TrackingURL) {
			t.Error("confirmation recipient or personal link mismatch")
		}
		for _, want := range []string{"14:00 - 15:30", "11:30 - 13:00", "13:00 - 14:00"} {
			if !strings.Contains(payload.HTML, want) || !strings.Contains(payload.Text, want) {
				t.Error("confirmation lacks current consult/seminar time", want)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"id":"accepted-fixture-message"}`)
	}))
	defer server.Close()
	m := ResendMailer{APIKey: "fixture-api-key", From: "Warwick <consult@example.com>", Config: s.Config, endpoint: server.URL, client: server.Client()}
	if done, err := s.ProcessMail(testCtx, m); err != nil || !done {
		t.Fatal("queued confirmation was not delivered", err)
	}
	if latest(t, s, on.Booking.ID).EmailStatus != "Sent" {
		t.Fatal("provider acceptance did not update email status")
	}
	if done, err := s.ProcessMail(testCtx, m); err != nil || done || requests != 1 {
		t.Fatal("sent confirmation was delivered again", err)
	}
}
