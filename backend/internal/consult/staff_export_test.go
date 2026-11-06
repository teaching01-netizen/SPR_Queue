package consult

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func exportParts(t *testing.T, data []byte) map[string]string {
	t.Helper()
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	parts := map[string]string{}
	for _, f := range z.File {
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		d := xml.NewDecoder(bytes.NewReader(b))
		for {
			_, err = d.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(f.Name, err)
			}
		}
		parts[f.Name] = string(b)
	}
	return parts
}
func exportRows(t *testing.T, body string) [][]string {
	t.Helper()
	var sheet struct {
		Rows []struct {
			Cells []struct {
				Text  string `xml:"is>t"`
				Value string `xml:"v"`
			} `xml:"c"`
		} `xml:"sheetData>row"`
	}
	if err := xml.Unmarshal([]byte(body), &sheet); err != nil {
		t.Fatal(err)
	}
	rows := [][]string{}
	for _, r := range sheet.Rows {
		row := []string{}
		for _, c := range r.Cells {
			if c.Value != "" {
				row = append(row, c.Value)
			} else {
				row = append(row, c.Text)
			}
		}
		rows = append(rows, row)
	}
	return rows
}

func TestRegistrationWorkbookAllStatusesAndSafeCells(t *testing.T) {
	cfg := novemberConfig(t)
	timeText := "2026-11-04T08:02:03Z"
	q := "B-M-001"
	bookings := []Booking{}
	for n, status := range []string{"Booked", "Waiting", "Calling", "Serving", "Completed", "Cancelled", "No-show"} {
		b := Booking{Input: cleanupInput(), ID: "fixture-" + status, Status: status, Mode: "On-site", QueueNo: &q, CreatedAt: timeText, UpdatedAt: timeText, CheckedInAt: &timeText, CurrentSubject: "Math", Orders: map[string]int{"Math": n + 1}, Stages: map[string]Stage{"Math": {CalledAt: timeText, StartedAt: timeText, EndedAt: timeText}}, Token: "DO-NOT-EXPORT-TOKEN", RequestID: "DO-NOT-EXPORT-REQUEST"}
		b.Wcode = "000001"
		b.Nickname = "นักเรียน <&> 🧑‍🎓"
		b.Questions = []string{"=HYPERLINK(\"https://example.com\")", "บรรทัดหนึ่ง\nบรรทัดสอง\x00\x0b", "+SUM(1,2)"}
		if status == "Completed" {
			b.ArchivedAt = timeText
			b.ArchivedBy = "RD1"
			b.CompletedAt = timeText
		}
		if status == "Cancelled" {
			b.CancelledAt = timeText
			b.CancellationReason = "แจ้งยกเลิก <&>"
		}
		bookings = append(bookings, b)
	}
	combined := bookings[0]
	combined.ID = "combined-fixture"
	combined.Subject = "Combined"
	combined.PeriodID = "P1400_1530"
	combined.CurrentSubject = "Verbal"
	combined.Stages = map[string]Stage{"Verbal": {StartedAt: timeText}}
	bookings = append(bookings, combined)
	high := bookings[0]
	high.ID = "high-fixture"
	high.Level = "High Level"
	high.Subject = "Verbal"
	high.Date = "2026-11-06"
	high.PeriodID = "P1155_1340"
	high.Mode = "Online via Zoom"
	bookings = append(bookings, high)
	legacy := bookings[0]
	legacy.ID = "legacy-fixture"
	legacy.PeriodID = "OLD_PERIOD"
	bookings = append(bookings, legacy)
	keptDone := bookings[4]
	keptDone.ID = "completed-visible"
	keptDone.ArchivedAt = ""
	keptDone.ArchivedBy = ""
	bookings = append(bookings, keptDone)
	bookings[5].Nickname = "Tar"
	bookings[5].Wcode = "W00001"
	before := append([]Booking(nil), bookings...)
	data, err := registrationWorkbook(cfg, bookings)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, bookings) {
		t.Fatal("export reordered caller's snapshot")
	}
	parts := exportParts(t, data)
	all := ""
	for _, body := range parts {
		all += body
	}
	if strings.Contains(all, "DO-NOT-EXPORT") || strings.Contains(all, "<f>") || strings.Contains(all, "TargetMode=\"External\"") {
		t.Fatal("private token, formula or external relationship exported")
	}
	if strings.Contains(all, "fixture-Cancelled") || strings.Contains(all, "fixture-Completed") || strings.Contains(all, "W00001") || strings.Contains(all, ">Tar<") {
		t.Fatal("removed test booking exported")
	}
	rows := exportRows(t, parts["xl/worksheets/sheet1.xml"])
	if len(rows) != len(bookings)-1 || len(rows[0]) != 39 {
		t.Fatal("missing records/columns", len(rows), len(rows[0]))
	}
	for _, r := range rows[1:] {
		if r[2] != "000001" || r[3] != "นักเรียน <&> 🧑‍🎓" || r[19] != "=HYPERLINK(\"https://example.com\")" || r[20] != "บรรทัดหนึ่ง\nบรรทัดสอง" || r[24] != "2026-11-04 15:02:03" {
			t.Fatal("text/time damaged", r)
		}
	}
	foundCombined, foundHigh, foundLegacy := false, false, false
	for _, r := range rows[1:] {
		switch r[0] {
		case "combined-fixture":
			foundCombined = r[7] == "Math + Verbal" && strings.Contains(r[18], "Math 11:30 - 13:00") && strings.Contains(r[18], "Verbal 13:00 - 14:00")
		case "high-fixture":
			foundHigh = r[9] == "11:55 - 13:40" && r[18] == ""
		case "legacy-fixture":
			foundLegacy = r[9] == "รอบเดิม OLD_PERIOD"
		}
	}
	if !foundCombined || !foundHigh || !foundLegacy {
		t.Fatal("combined/high/historical information missing")
	}
	summary := exportRows(t, parts["xl/worksheets/sheet2.xml"])
	if len(summary) != 15 {
		t.Fatal("missing empty/historical periods", len(summary))
	}
	found := false
	for _, r := range summary[1:] {
		if r[0] == "2026-11-07" && r[1] == "Beginner" && r[2] == "Math" && r[3] == "18:00 - 20:00" {
			found = true
			if !reflect.DeepEqual(r[5:], []string{"8", "6", "6", "1", "1", "1", "1", "1", "1"}) {
				t.Fatal("summary counts", r)
			}
		}
	}
	if !found {
		t.Fatal("summary row absent")
	}
	if !strings.Contains(parts["xl/worksheets/sheet1.xml"], `t="n"`) {
		t.Fatal("queue order must be numeric")
	}
	if !strings.Contains(parts["xl/worksheets/sheet1.xml"], `state="frozen"`) || !strings.Contains(parts["xl/worksheets/sheet1.xml"], `<autoFilter ref="A1:AM10"`) {
		t.Fatal("missing freeze/filter")
	}
	if path := os.Getenv("EXPORT_FIXTURE_PATH"); path != "" {
		if err = os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	empty, err := registrationWorkbook(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(exportRows(t, exportParts(t, empty)["xl/worksheets/sheet1.xml"])) != 1 {
		t.Fatal("empty workbook must retain headers")
	}
}

func TestHTTPStaffExportAllRecordsAndAuthorization(t *testing.T) {
	s := testService(t)
	hidden := book(t, s, testInput())
	s.Config = novemberConfig(t)
	s.Config.Event.ID = "local-test-" + s.Config.Event.ID
	s.LocalTestDataManagement = true
	if err := s.migrate(testCtx); err != nil {
		t.Fatal(err)
	}
	cancelled := book(t, s, cleanupInput())
	if _, err := s.Status(testCtx, cancelled.Booking.ID, "Cancelled", cancelled.Booking.Revision, "RD1", "Test cancellation"); err != nil {
		t.Fatal(err)
	}
	done := book(t, s, cleanupInput())
	completed := finishCleanup(t, s, done)
	if _, err := s.ArchiveCompleted(testCtx, completed.ID, true, completed.Revision, "RD1"); err != nil {
		t.Fatal(err)
	}
	book(t, s, cleanupInput())
	input := cleanupInput()
	input.Level = "High Level"
	input.Subject = "Verbal"
	input.Date = "2026-11-06"
	input.PeriodID = "P1000_1130"
	book(t, s, input)
	a, err := NewAuth(s, false)
	if err != nil {
		t.Fatal(err)
	}
	a.Cost = bcrypt.MinCost
	if err = a.SyncLocalStaff(testCtx); err != nil {
		t.Fatal(err)
	}
	// Operators as well as admins can export the data they can already read.
	id, err := a.Login(testCtx, "SM1", "SM1")
	if err != nil {
		t.Fatal(err)
	}
	if err = a.StoreSession(testCtx, id, ""); err != nil {
		t.Fatal(err)
	}
	h := NewHTTP(s, a, []string{"http://localhost:8080"}, "").Handler()
	get := func(auth bool, origin string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/api/staff/export.xlsx?subject=Math&date=2026-11-07", nil)
		if auth {
			r.AddCookie(&http.Cookie{Name: "warwick_session", Value: id.Token})
		}
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := get(false, ""); w.Code != 401 {
		t.Fatal("anonymous export", w.Code)
	}
	if w := get(true, "https://forged.example"); w.Code != 403 {
		t.Fatal("forged origin export", w.Code)
	}
	before, err := s.StaffList(testCtx)
	if err != nil {
		t.Fatal(err)
	}
	w := get(true, "http://localhost:8080")
	if w.Code != 200 || w.Header().Get("Content-Type") != excelContentType || w.Header().Get("Cache-Control") != "no-store" || !strings.Contains(w.Header().Get("Content-Disposition"), ".xlsx") {
		t.Fatal("download failed", w.Code, w.Header())
	}
	parts := exportParts(t, w.Body.Bytes())
	rows := exportRows(t, parts["xl/worksheets/sheet1.xml"])
	if len(rows) != 3 {
		t.Fatal("filters omitted records", len(rows))
	}
	for _, body := range parts {
		if strings.Contains(body, hidden.Booking.ID) || strings.Contains(body, done.Booking.ID) || strings.Contains(body, cancelled.Booking.ID) || strings.Contains(body, done.Token) || strings.Contains(body, cancelled.Token) {
			t.Fatal("other event or private token leaked")
		}
	}
	after, err := s.StaffList(testCtx)
	if err != nil || !sameStaffState(before, after) {
		t.Fatal("export changed booking/queue/revision", err)
	}
	if _, err := s.DB.ExecContext(testCtx, `DELETE FROM sessions WHERE token_hash=$1`, digest(id.Token)); err != nil {
		t.Fatal(err)
	}
	if w := get(true, ""); w.Code != 401 {
		t.Fatal("expired session downloaded", w.Code)
	}
}
