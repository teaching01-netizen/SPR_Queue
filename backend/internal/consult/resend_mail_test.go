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
