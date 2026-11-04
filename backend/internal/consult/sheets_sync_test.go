package consult

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

type sheetRecorder struct {
	calls []SheetSnapshot
	fail  bool
}

func (r *sheetRecorder) Sync(_ context.Context, p SheetSnapshot) error {
	r.calls = append(r.calls, p)
	if r.fail {
		return errors.New("offline")
	}
	return nil
}

func TestSheetSnapshotsRetryLatestAndPreserveRows(t *testing.T) {
	s := cleanupService(t)
	initial := cleanupInput()
	initial.Questions = []string{"ข้อความ\x0b\n=1+1", "two"}
	one := book(t, s, initial)
	i := cleanupInput()
	i.Level = "High Level"
	i.Subject = "Verbal"
	i.Date = "2026-11-06"
	i.PeriodID = "P1000_1130"
	two := book(t, s, i)
	p, err := s.sheetSnapshot(testCtx)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Headers) != 39 || len(p.Rows) != 2 || p.Rows[0][0] != one.Booking.ID || p.Rows[1][0] != two.Booking.ID {
		t.Fatal("not oldest-registration first", p)
	}
	if p.Rows[0][19] != "ข้อความ\n=1+1" {
		t.Fatal("cell control text not normalized")
	}
	for _, row := range p.Rows {
		if strings.Contains(strings.Join(row, "|"), one.Token) {
			t.Fatal("private token included")
		}
	}
	excel, err := s.StaffExport(testCtx)
	if err != nil {
		t.Fatal(err)
	}
	excelRows := exportRows(t, exportParts(t, excel)["xl/worksheets/sheet1.xml"])
	if strings.Join(p.Headers, "|") != strings.Join(excelRows[0], "|") {
		t.Fatal("Excel/Sheets columns drifted")
	}
	progress := sheetSyncProgress{}
	sink := &sheetRecorder{fail: true}
	now := time.Now()
	if _, err = progress.once(testCtx, s, sink, now); err == nil || progress.sent {
		t.Fatal("failure advanced delivery cursor")
	}
	corrected, err := s.EditStudentDetails(testCtx, one.Booking.ID, StudentDetailsEdit{"00001", "Corrected", "School", "new@example.com", one.Booking.Revision}, "SM1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Status(testCtx, two.Booking.ID, "Cancelled", two.Booking.Revision, "SM1", "Test removal"); err != nil {
		t.Fatal(err)
	}
	three := book(t, s, cleanupInput())
	sink.fail = false
	if sent, err := progress.once(testCtx, s, sink, now.Add(30*time.Second)); err != nil || !sent {
		t.Fatal("retry failed", err)
	}
	last := sink.calls[len(sink.calls)-1]
	if len(last.Rows) != 2 || last.Rows[0][0] != corrected.ID || last.Rows[0][2] != "00001" || last.Rows[0][3] != "Corrected" || last.Rows[1][0] != three.Booking.ID {
		t.Fatal("retry did not take latest snapshot", last)
	}
	if sent, err := progress.once(testCtx, s, sink, now.Add(time.Minute)); err != nil || sent {
		t.Fatal("unchanged data pushed repeatedly", err)
	}
	if sent, err := progress.once(testCtx, s, sink, now.Add(6*time.Minute)); err != nil || !sent {
		t.Fatal("reconciliation missing", err)
	}
	// A new worker after a restart always reconciles using stable booking IDs.
	restarted := sheetSyncProgress{}
	if sent, err := restarted.once(testCtx, s, sink, now.Add(7*time.Minute)); err != nil || !sent {
		t.Fatal(err)
	}
	view, _ := s.StaffList(testCtx)
	if view.Bookings[0].Revision < 1 {
		t.Fatal("snapshot mutated database")
	}
}

func TestAppsScriptSheetTransportSignatureAndAcknowledgment(t *testing.T) {
	secret := strings.Repeat("ab", 32)
	now := time.Unix(1793760000, 0)
	p := SheetSnapshot{Action: "sync-registrations", EventID: "production-fixture", Revision: 8, Headers: []string{"รหัสการจอง"}, Rows: [][]string{{"literal =1+1", "ชื่อไทย"}}, RequestID: strings.Repeat("c", 64)}
	mode := "ok"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.Header.Get("Content-Type") != "application/json" {
			t.Error("wrong request")
		}
		var e appsScriptEnvelope
		if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
			t.Error(err)
		}
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(strconv.FormatInt(e.Timestamp, 10) + "\n" + e.Payload))
		if e.Timestamp != now.Unix() || hex.EncodeToString(mac.Sum(nil)) != e.Signature {
			t.Error("bad signature")
		}
		raw, err := base64.StdEncoding.DecodeString(e.Payload)
		if err != nil || !strings.Contains(string(raw), "ชื่อไทย") {
			t.Error("payload damaged")
		}
		switch mode {
		case "ok":
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "status": "synced", "requestId": p.RequestID, "revision": 8})
		case "stale":
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "status": "stale", "requestId": p.RequestID, "revision": 9})
		case "wrong-id":
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "status": "synced", "requestId": "wrong", "revision": 8})
		case "old":
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "status": "synced", "requestId": p.RequestID, "revision": 7})
		case "error":
			w.Write([]byte(`{"ok":false,"code":"TARGET_NOT_EMPTY"}`))
		case "trailing":
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "status": "synced", "requestId": p.RequestID, "revision": 8})
			w.Write([]byte(`{}`))
		case "login":
			http.Redirect(w, r, "https://accounts.google.com/", 302)
		}
	}))
	defer server.Close()
	m := AppsScriptSheetSink{URL: "https://script.google.com/macros/s/deployment_fixture_123/exec", Secret: secret, client: server.Client(), endpoint: server.URL, now: func() time.Time { return now }}
	for _, value := range []string{"ok", "stale", "wrong-id", "old", "error", "trailing", "login"} {
		mode = value
		err := m.Sync(testCtx, p)
		if (value == "ok" || value == "stale") != (err == nil) {
			t.Fatal(value, err)
		}
		if err != nil && (strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), p.RequestID)) {
			t.Fatal("private state leaked")
		}
	}
	ctx, cancel := context.WithCancel(testCtx)
	cancel()
	if err := m.Sync(ctx, p); err == nil {
		t.Fatal("cancel ignored")
	}
}
