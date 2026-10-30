package consult

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

const scriptFixtureURL = "https://script.google.com/macros/s/deployment_fixture_123/exec"
const scriptFixtureSecret = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func readScriptMessage(t *testing.T, r *http.Request) appsScriptMessage {
	t.Helper()
	if r.Method != "POST" || r.Header.Get("Content-Type") != "application/json" {
		t.Error("incorrect relay method/content type")
	}
	var envelope appsScriptEnvelope
	if err := json.NewDecoder(r.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	mac := hmac.New(sha256.New, []byte(scriptFixtureSecret))
	mac.Write([]byte(strconv.FormatInt(envelope.Timestamp, 10) + "\n" + envelope.Payload))
	if envelope.Signature != hex.EncodeToString(mac.Sum(nil)) {
		t.Error("invalid signed request")
	}
	raw, err := base64.StdEncoding.DecodeString(envelope.Payload)
	if err != nil {
		t.Fatal(err)
	}
	var message appsScriptMessage
	if err := json.Unmarshal(raw, &message); err != nil {
		t.Fatal(err)
	}
	return message
}

func TestAppsScriptSignedTemplateAndRetryIdentity(t *testing.T) {
	cfg := novemberConfig(t)
	i := cleanupInput()
	i.Subject, i.PeriodID = "Combined", "P1400_1530"
	b := Booking{ID: "booking-fixture", EventID: cfg.Event.ID, Input: i, Mode: "On-site"}
	tracking := "https://consult.example/#/track/private-fixture-token"
	var ids []string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		message := readScriptMessage(t, r)
		ids = append(ids, message.RequestID)
		if message.To != i.Email || message.Subject != "Warwick Private Consult - Booking Confirmation" {
			t.Error("wrong recipient/subject")
		}
		for _, body := range []string{message.HTML, message.Text} {
			for _, want := range []string{tracking, "14:00 - 15:30", "11:30 - 13:00", "13:00 - 14:00"} {
				if !strings.Contains(body, want) {
					t.Error("lost template detail", want)
				}
			}
		}
		status := "sent"
		if len(ids) == 2 {
			status = "duplicate"
		}
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "requestId": message.RequestID, "status": status})
	}))
	defer server.Close()
	m := AppsScriptMailer{URL: scriptFixtureURL, Secret: scriptFixtureSecret, Config: cfg, endpoint: server.URL, client: server.Client()}
	for n := 0; n < 2; n++ {
		if err := m.Send(testCtx, b, tracking); err != nil {
			t.Fatal(err)
		}
	}
	if ids[0] == "" || ids[0] != ids[1] {
		t.Fatal("retry identity changed")
	}
	b.Nickname = "ชื่อใหม่"
	if err := m.Send(testCtx, b, tracking); err != nil || ids[2] == ids[0] {
		t.Fatal("updated confirmation suppressed", err)
	}
}

type scriptRoundTrip func(*http.Request) (*http.Response, error)

func (f scriptRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAppsScriptResponseRedirectAndFailures(t *testing.T) {
	cfg := novemberConfig(t)
	b := Booking{ID: "fixture", EventID: cfg.Event.ID, Input: cleanupInput()}
	for _, target := range []string{"https://script.googleusercontent.com/macros/echo?user_content_key=fixture", "https://accounts.google.com/login", "https://attacker.example/macros/echo"} {
		calls, id := 0, ""
		client := &http.Client{Transport: scriptRoundTrip(func(r *http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				id = readScriptMessage(t, r).RequestID
				return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{target}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
			}
			if r.Method != "GET" || r.Body != nil {
				t.Error("signed body forwarded to redirect")
			}
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(fmt.Sprintf(`{"ok":true,"requestId":%q,"status":"sent"}`, id))), Request: r}, nil
		})}
		m := AppsScriptMailer{URL: scriptFixtureURL, Secret: scriptFixtureSecret, Config: cfg, client: client}
		err := m.Send(testCtx, b, "https://consult.example/#/track/token")
		if strings.Contains(target, "script.googleusercontent.com") {
			if err != nil || calls != 2 {
				t.Fatal("ContentService redirect failed", err)
			}
		} else if err == nil || calls != 1 || strings.Contains(err.Error(), target) {
			t.Fatal("unsafe redirect followed or disclosed")
		}
	}
	for _, body := range []string{`{"ok":false,"code":"QUOTA_EXCEEDED"}`, `{"ok":true,"requestId":"wrong","status":"sent"}`, `<html>Sign in</html>`, `{"ok":true,"requestId":"wrong","status":"duplicate"}`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, body) }))
		m := AppsScriptMailer{URL: scriptFixtureURL, Secret: scriptFixtureSecret, Config: cfg, endpoint: server.URL}
		err := m.Send(testCtx, b, "https://consult.example/#/track/private-token")
		server.Close()
		if err == nil || strings.Contains(err.Error(), scriptFixtureSecret) || strings.Contains(err.Error(), "private-token") {
			t.Fatal("false success or leaked credentials", err)
		}
	}
	for _, method := range []string{"POST", "PUT"} {
		u, _ := url.Parse("https://script.googleusercontent.com/macros/echo")
		if appsScriptRedirect(&http.Request{Method: method, URL: u}, nil) == nil {
			t.Fatal("body-preserving redirect accepted")
		}
	}
}

func TestAppsScriptConfigurationAndOutbox(t *testing.T) {
	for _, endpoint := range []string{"http://script.google.com/macros/s/deployment_fixture_123/exec", "https://script.google.com/macros/s/deployment_fixture_123/dev", "https://script.google.com.evil.test/macros/s/deployment_fixture_123/exec", scriptFixtureURL + "?key=secret"} {
		if ValidateAppsScriptSettings(endpoint, scriptFixtureSecret) == nil {
			t.Fatal("invalid deployment URL accepted")
		}
	}
	if ValidateAppsScriptSettings(scriptFixtureURL, "short") == nil {
		t.Fatal("weak key accepted")
	}
	s := novemberService(t)
	s.MailEnabled = true
	c := book(t, s, cleanupInput())
	accepted := false
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		message := readScriptMessage(t, r)
		if !accepted {
			io.WriteString(w, `{"ok":false,"code":"QUOTA_EXCEEDED"}`)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "requestId": message.RequestID, "status": "sent"})
	}))
	defer server.Close()
	m := AppsScriptMailer{URL: scriptFixtureURL, Secret: scriptFixtureSecret, Config: s.Config, endpoint: server.URL, client: server.Client()}
	if done, err := s.ProcessMail(context.Background(), m); err != nil || !done || latest(t, s, c.Booking.ID).EmailStatus != "Failed" {
		t.Fatal("failure was marked sent", err)
	}
	if _, err := s.DB.Exec(`UPDATE outbox SET next_at=0`); err != nil {
		t.Fatal(err)
	}
	accepted = true
	if done, err := s.ProcessMail(testCtx, m); err != nil || !done || latest(t, s, c.Booking.ID).EmailStatus != "Sent" {
		t.Fatal("retry did not send", err)
	}
	if done, err := s.ProcessMail(testCtx, m); err != nil || done {
		t.Fatal("sent mail queued again", err)
	}
}
