package consult

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

var testCtx = context.Background()

func testConfig(t *testing.T) Config {
	t.Helper()
	c, e := LoadConfig("testdata/regression-event.json")
	if e != nil {
		t.Fatal(e)
	}
	return c
}
func testService(t *testing.T) *Service {
	t.Helper()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL is required to run PostgreSQL integration tests")
	}
	admin, e := sql.Open("pgx", base)
	if e != nil {
		t.Fatal(e)
	}
	token, _ := randomToken()
	schema := "warwick_test_" + digest(token)[:20]
	if _, e = admin.Exec("CREATE SCHEMA " + pgx.Identifier{schema}.Sanitize()); e != nil {
		admin.Close()
		t.Fatal(e)
	}
	u, e := url.Parse(base)
	if e != nil {
		t.Fatal(e)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	s, e := Open(u.String(), testConfig(t), bytes.Repeat([]byte{7}, 32), "http://localhost:8080")
	if e != nil {
		admin.Exec("DROP SCHEMA " + pgx.Identifier{schema}.Sanitize() + " CASCADE")
		admin.Close()
		t.Fatal(e)
	}
	t.Cleanup(func() {
		s.DB.Close()
		admin.Exec("DROP SCHEMA " + pgx.Identifier{schema}.Sanitize() + " CASCADE")
		admin.Close()
	})
	return s
}
func testInput() Input {
	return Input{Email: "parent@example.com", Nickname: "Example", School: "School", Level: "Beginner", Subject: "Math", Date: "2026-08-15", PeriodID: "P1100", Questions: []string{"Question 1", "Question 2"}, SeminarConfirmed: true}
}
func book(t *testing.T, s *Service, i Input) Confirmation {
	t.Helper()
	key, _ := randomToken()
	b, e := s.Book(testCtx, key, i)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func latest(t *testing.T, s *Service, id string) Booking {
	t.Helper()
	v, e := s.StaffList(testCtx)
	if e != nil {
		t.Fatal(e)
	}
	for _, b := range v.Bookings {
		if b.ID == id {
			return b
		}
	}
	t.Fatal("not found")
	return Booking{}
}
func transition(t *testing.T, s *Service, id, status string) Booking {
	t.Helper()
	b := latest(t, s, id)
	out, e := s.Status(testCtx, id, status, b.Revision, "staff", "")
	if e != nil {
		t.Fatal(e)
	}
	return out
}
func assign(t *testing.T, s *Service, scope Scope, ids ...string) {
	t.Helper()
	v, e := s.StaffList(testCtx)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Assign(testCtx, scope, ids, v.Revision, "staff"); e != nil {
		t.Fatal(e)
	}
}
func defaultScope() Scope                { return Scope{"Beginner", "2026-08-15", "P1100", "Math"} }
func codeIs(err error, code string) bool { var p *Problem; return errors.As(err, &p) && p.Code == code }
func TestConcurrentBookingAcrossConnections(t *testing.T) {
	s := testService(t)
	other, e := Open(testDatabaseURL(t, s), s.Config, s.Key, s.PublicURL)
	if e != nil {
		t.Fatal(e)
	}
	defer other.DB.Close()
	var accepted, full atomic.Int64
	var wg sync.WaitGroup
	start := make(chan struct{})
	for n := 0; n < 40; n++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			<-start
			target := s
			if n%2 == 0 {
				target = other
			}
			_, e := target.Book(testCtx, fmt.Sprintf("concurrent-%d", n), testInput())
			if e == nil {
				accepted.Add(1)
			} else if codeIs(e, "PERIOD_FULL") {
				full.Add(1)
			} else {
				t.Errorf("unexpected error: %v", e)
			}
		}(n)
	}
	close(start)
	wg.Wait()
	if accepted.Load() != 4 || full.Load() != 36 {
		t.Fatalf("accepted %d full %d", accepted.Load(), full.Load())
	}
	v, _ := s.StaffList(testCtx)
	if len(v.Bookings) != 4 {
		t.Fatal("overbooking")
	}
}
func testDatabaseURL(t *testing.T, s *Service) string { t.Helper(); return s.DatabaseURL }

func TestIdempotencyAndPayloadConflict(t *testing.T) {
	s := testService(t)
	i := testInput()
	a, e := s.Book(testCtx, "retry-key", i)
	if e != nil {
		t.Fatal(e)
	}
	for n := 0; n < 3; n++ {
		book(t, s, i)
	}
	b, e := s.Book(testCtx, "retry-key", i)
	if e != nil || a.Booking.ID != b.Booking.ID || a.Token != b.Token {
		t.Fatal("retry duplicated or lost token", e)
	}
	i.Nickname = "Different"
	if _, e = s.Book(testCtx, "retry-key", i); !codeIs(e, "IDEMPOTENCY_CONFLICT") {
		t.Fatal(e)
	}
}
func TestValidation(t *testing.T) {
	s := testService(t)
	cases := map[string]func(*Input){"subject": func(i *Input) { i.Subject = "IELTS" }, "level": func(i *Input) { i.Level = "Admin" }, "period": func(i *Input) { i.PeriodID = "P1315" }, "date": func(i *Input) { i.Date = "2026-08-14" }, "email": func(i *Input) { i.Email = "bad" }, "email-header": func(i *Input) { i.Email = "a@example.com\r\nBcc: x@example.com" }, "question": func(i *Input) { i.Questions[0] = " " }, "seminar": func(i *Input) { i.SeminarConfirmed = false }, "long-question": func(i *Input) { i.Questions[0] = strings.Repeat("x", 1001) }}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			i := testInput()
			change(&i)
			if _, e := s.Book(testCtx, "invalid-"+name, i); e == nil {
				t.Fatal("accepted invalid input")
			}
		})
	}
}
func TestCapacityCancelCompleteNoShow(t *testing.T) {
	s := testService(t)
	a := book(t, s, testInput())
	b := book(t, s, testInput())
	book(t, s, testInput())
	book(t, s, testInput())
	if _, e := s.Status(testCtx, a.Booking.ID, "Cancelled", a.Booking.Revision, "staff", "Changed plans"); e != nil {
		t.Fatal(e)
	}
	book(t, s, testInput())
	transition(t, s, b.Booking.ID, "Waiting")
	transition(t, s, b.Booking.ID, "No-show")
	if _, e := s.Book(testCtx, "full", testInput()); !codeIs(e, "PERIOD_FULL") {
		t.Fatal(e)
	}
}
func TestQueueStateAndStableNumbers(t *testing.T) {
	s := testService(t)
	a := book(t, s, testInput())
	b := book(t, s, testInput())
	if _, e := s.Status(testCtx, a.Booking.ID, "Completed", 1, "staff", ""); !codeIs(e, "INVALID_STATUS_TRANSITION") {
		t.Fatal(e)
	}
	assign(t, s, defaultScope(), a.Booking.ID, b.Booking.ID)
	original := latest(t, s, a.Booking.ID)
	assign(t, s, defaultScope(), b.Booking.ID, a.Booking.ID)
	current := latest(t, s, a.Booking.ID)
	if *current.QueueNo != *original.QueueNo || current.Orders["Math"] != 2 {
		t.Fatal("queue identity changed")
	}
	waiting := transition(t, s, a.Booking.ID, "Waiting")
	called := transition(t, s, a.Booking.ID, "Calling")
	if _, e := s.Status(testCtx, a.Booking.ID, "Serving", waiting.Revision, "staff", ""); !codeIs(e, "REVISION_CONFLICT") {
		t.Fatal(e)
	}
	if _, e := s.Status(testCtx, a.Booking.ID, "Waiting", called.Revision, "staff", ""); e != nil {
		t.Fatal(e)
	}
	recheck := transition(t, s, a.Booking.ID, "Waiting")
	if *recheck.CheckedInAt != *waiting.CheckedInAt {
		t.Fatal("check-in timestamp overwritten")
	}
	transition(t, s, a.Booking.ID, "Calling")
	transition(t, s, a.Booking.ID, "Serving")
	complete := transition(t, s, a.Booking.ID, "Completed")
	if complete.CompletedAt == "" {
		t.Fatal("missing completion timestamp")
	}
}
func TestNoShowRecovery(t *testing.T) {
	s := testService(t)
	a := book(t, s, testInput())
	waiting := transition(t, s, a.Booking.ID, "Waiting")
	transition(t, s, a.Booking.ID, "No-show")
	recover := transition(t, s, a.Booking.ID, "Waiting")
	if *recover.CheckedInAt != *waiting.CheckedInAt {
		t.Fatal("checkin changed")
	}
	audit, _ := s.Audit(testCtx)
	if audit[0].Action != "RECOVER_NO_SHOW" {
		t.Fatal("missing recovery audit")
	}
}
func TestSingleActiveStreamConcurrent(t *testing.T) {
	s := testService(t)
	a := book(t, s, testInput())
	b := book(t, s, testInput())
	assign(t, s, defaultScope(), a.Booking.ID, b.Booking.ID)
	aa := transition(t, s, a.Booking.ID, "Waiting")
	bb := transition(t, s, b.Booking.ID, "Waiting")
	var wg sync.WaitGroup
	var ok atomic.Int64
	for _, b := range []Booking{aa, bb} {
		wg.Add(1)
		go func(b Booking) {
			defer wg.Done()
			_, e := s.Status(testCtx, b.ID, "Calling", b.Revision, "staff", "")
			if e == nil {
				ok.Add(1)
			} else if !codeIs(e, "STREAM_BUSY") {
				t.Error(e)
			}
		}(b)
	}
	wg.Wait()
	if ok.Load() != 1 {
		t.Fatal("multiple active queues")
	}
}
func TestCombinedMathVerbal(t *testing.T) {
	s := testService(t)
	i := testInput()
	i.Subject = "Combined"
	i.PeriodID = "P1300"
	a := book(t, s, i)
	sc := Scope{i.Level, i.Date, i.PeriodID, "Math"}
	assign(t, s, sc, a.Booking.ID)
	transition(t, s, a.Booking.ID, "Waiting")
	transition(t, s, a.Booking.ID, "Calling")
	transition(t, s, a.Booking.ID, "Serving")
	b := transition(t, s, a.Booking.ID, "Completed")
	if b.CurrentSubject != "Verbal" || b.Status != "Waiting" || b.CompletedAt != "" || b.Stages["Math"].EndedAt == "" {
		t.Fatal("incorrect handoff")
	}
	if _, e := s.Status(testCtx, b.ID, "Calling", b.Revision, "staff", ""); !codeIs(e, "QUEUE_NOT_READY") {
		t.Fatal(e)
	}
	sc.Stream = "Verbal"
	assign(t, s, sc, b.ID)
	transition(t, s, b.ID, "Calling")
	transition(t, s, b.ID, "Serving")
	finished := transition(t, s, b.ID, "Completed")
	if finished.CompletedAt == "" || finished.Stages["Verbal"].EndedAt == "" || *finished.QueueNo != *b.QueueNo {
		t.Fatal("incorrect final stage")
	}
}
func TestTrackingProjectionAndHighLevel(t *testing.T) {
	s := testService(t)
	i := testInput()
	i.Level = "High Level"
	i.Date = "2026-08-14"
	i.PeriodID = "P1100"
	i.SeminarConfirmed = false
	a := book(t, s, i)
	sc := Scope{i.Level, i.Date, i.PeriodID, "Math"}
	assign(t, s, sc, a.Booking.ID)
	transition(t, s, a.Booking.ID, "Waiting")
	transition(t, s, a.Booking.ID, "Calling")
	p, e := s.Track(testCtx, a.Token)
	if e != nil || len(p.CurrentQueues) != 1 {
		t.Fatal(e)
	}
	text, _ := encode(p)
	var obj map[string]any
	json.Unmarshal([]byte(text), &obj)
	allowed := map[string]bool{"queueNo": true, "currentQueues": true, "currentSubject": true, "status": true, "counter": true, "queuesUntilTurn": true, "queueCountState": true}
	for key := range obj {
		if !allowed[key] {
			t.Fatal("unexpected public field", key)
		}
	}
	if len(obj) != 7 || obj["queueNo"] == nil || obj["queuesUntilTurn"] != float64(0) || obj["queueCountState"] != "current" || strings.Contains(text, "example.com") || strings.Contains(text, "P1100") {
		t.Fatal("private information disclosed")
	}
	if _, e = s.Track(testCtx, "invalidtoken"); !codeIs(e, "BOOKING_NOT_FOUND") {
		t.Fatal(e)
	}
	staff, _ := s.StaffList(testCtx)
	raw, _ := encode(staff)
	if strings.Contains(raw, a.Token) || strings.Contains(raw, "token") {
		t.Fatal("staff list leaks token")
	}
}
func TestPersistenceAndTokenEncryption(t *testing.T) {
	s := testService(t)
	a := book(t, s, testInput())
	path := testDatabaseURL(t, s)
	var encrypted, hash string
	if e := s.DB.QueryRow(`SELECT token_cipher,token_hash FROM bookings WHERE id=$1`, a.Booking.ID).Scan(&encrypted, &hash); e != nil {
		t.Fatal(e)
	}
	if strings.Contains(encrypted, a.Token) || hash == a.Token {
		t.Fatal("token stored plaintext")
	}
	other, e := Open(path, s.Config, s.Key, s.PublicURL)
	if e != nil {
		t.Fatal(e)
	}
	defer other.DB.Close()
	if _, e = other.Track(testCtx, a.Token); e != nil {
		t.Fatal("record missing after reopen", e)
	}
}
func TestOrderRevisionConflictAndAtomicRollback(t *testing.T) {
	s := testService(t)
	a := book(t, s, testInput())
	v, _ := s.StaffList(testCtx)
	b := book(t, s, testInput())
	if e := s.Assign(testCtx, defaultScope(), []string{a.Booking.ID, b.Booking.ID}, v.Revision, "staff"); !codeIs(e, "REVISION_CONFLICT") {
		t.Fatal(e)
	}
	v, _ = s.StaffList(testCtx)
	if e := s.Assign(testCtx, defaultScope(), []string{a.Booking.ID, "missing"}, v.Revision, "staff"); !codeIs(e, "STALE_QUEUE") {
		t.Fatal(e)
	}
	if latest(t, s, a.Booking.ID).QueueNo != nil {
		t.Fatal("partial assignment")
	}
}
func TestRescheduleAtomic(t *testing.T) {
	s := testService(t)
	a := book(t, s, testInput())
	i := testInput()
	i.PeriodID = "P1630_1800"
	for n := 0; n < 6; n++ {
		book(t, s, i)
	}
	if _, e := s.Reschedule(testCtx, a.Booking.ID, a.Booking.Date, "P1630_1800", 1, "admin"); !codeIs(e, "PERIOD_FULL") {
		t.Fatal(e)
	}
	if latest(t, s, a.Booking.ID).PeriodID != "P1100" {
		t.Fatal("old reservation lost")
	}
	b, e := s.Reschedule(testCtx, a.Booking.ID, a.Booking.Date, "P1800", 1, "admin")
	if e != nil || b.ID != a.Booking.ID {
		t.Fatal(e)
	}
	periods, _ := s.Periods(testCtx, "Beginner", "Math", a.Booking.Date)
	if !periods[0].Available || periods[1].Available {
		t.Fatal("incorrect capacity after move")
	}
}

type fakeMailer struct {
	err  error
	sent int
}

func (m *fakeMailer) Send(_ context.Context, b Booking, tracking string) error {
	m.sent++
	if !strings.Contains(tracking, "#/track/") {
		return fmt.Errorf("missing tracking link")
	}
	return m.err
}
func TestEmailFailureAndRetry(t *testing.T) {
	s := testService(t)
	s.MailEnabled = true
	a := book(t, s, testInput())
	if a.Booking.EmailStatus != "Pending" {
		t.Fatal("not queued")
	}
	mail := &fakeMailer{err: fmt.Errorf("smtp unavailable")}
	if _, e := s.ProcessMail(testCtx, mail); e != nil {
		t.Fatal(e)
	}
	b := latest(t, s, a.Booking.ID)
	if b.Status != "Booked" || b.EmailStatus != "Failed" {
		t.Fatal("booking lost or falsely sent")
	}
	s.DB.Exec(`UPDATE outbox SET next_at=0`)
	mail.err = nil
	if _, e := s.ProcessMail(testCtx, mail); e != nil {
		t.Fatal(e)
	}
	if latest(t, s, a.Booking.ID).EmailStatus != "Sent" || mail.sent != 2 {
		t.Fatal("retry failed")
	}
}
func TestConfigCapacityBelowReservationsRejected(t *testing.T) {
	s := testService(t)
	for n := 0; n < 3; n++ {
		book(t, s, testInput())
	}
	c := s.Config
	c.Rules[0].Periods[0].Capacity = 2
	if other, e := Open(testDatabaseURL(t, s), c, s.Key, s.PublicURL); e == nil {
		other.DB.Close()
		t.Fatal("capacity reduced below occupancy")
	}
}

func TestWrongEncryptionKeyRejected(t *testing.T) {
	s := testService(t)
	book(t, s, testInput())
	other, e := Open(testDatabaseURL(t, s), s.Config, bytes.Repeat([]byte{9}, 32), s.PublicURL)
	if e == nil {
		other.DB.Close()
		t.Fatal("wrong APP_SECRET accepted")
	}
}
func TestCancelledMailIsNotSent(t *testing.T) {
	s := testService(t)
	s.MailEnabled = true
	a := book(t, s, testInput())
	if _, e := s.Status(testCtx, a.Booking.ID, "Cancelled", 1, "staff", "Cancelled before delivery"); e != nil {
		t.Fatal(e)
	}
	m := &fakeMailer{}
	if _, e := s.ProcessMail(testCtx, m); e != nil {
		t.Fatal(e)
	}
	if m.sent != 0 {
		t.Fatal("cancelled booking email was sent")
	}
}
func TestRescheduleWaitsForDelivery(t *testing.T) {
	s := testService(t)
	s.MailEnabled = true
	a := book(t, s, testInput())
	s.DB.Exec(`UPDATE outbox SET state='Sending' WHERE booking_id=$1`, a.Booking.ID)
	if _, e := s.Reschedule(testCtx, a.Booking.ID, a.Booking.Date, "P1630_1800", 1, "admin"); !codeIs(e, "NOTIFICATION_BUSY") {
		t.Fatal(e)
	}
	if latest(t, s, a.Booking.ID).PeriodID != "P1100" {
		t.Fatal("rescheduled while email was in flight")
	}
}
