package consult

import (
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
)

func novemberConfig(t *testing.T) Config {
	t.Helper()
	cfg, err := LoadConfig("../../config/event.json")
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}
func novemberService(t *testing.T) *Service {
	t.Helper()
	s := testService(t)
	s.Config = novemberConfig(t)
	if err := s.migrate(testCtx); err != nil {
		t.Fatal(err)
	}
	return s
}
func TestNovemberExactSchedule(t *testing.T) {
	cfg := novemberConfig(t)
	expected := map[string][]string{
		"Beginner/Math/2026-11-07":       {"13:00-14:00/4", "17:00-18:00/4", "18:00-20:00/8"},
		"Beginner/Verbal/2026-11-07":     {"17:00-18:00/4", "18:00-20:00/7"},
		"Beginner/Combined/2026-11-07":   {"14:00-15:30/5", "15:30-17:00/6"},
		"High Level/Math/2026-11-06":     {"13:00-14:00/4", "17:00-18:00/4", "18:00-20:00/8"},
		"High Level/Verbal/2026-11-06":   {"17:00-18:00/4", "18:00-20:00/7"},
		"High Level/Combined/2026-11-06": {"14:00-15:30/5", "15:30-17:00/6"},
	}
	if len(cfg.Rules) != len(expected) {
		t.Fatal("wrong rule count")
	}
	periodCount := 0
	for _, r := range cfg.Rules {
		key := r.Level + "/" + r.Subject + "/" + r.Date
		got := []string{}
		for _, p := range r.Periods {
			got = append(got, fmt.Sprintf("%s-%s/%d", p.Start, p.End, p.Capacity))
			periodCount++
		}
		if !reflect.DeepEqual(got, expected[key]) {
			t.Fatal(key, got)
		}
		wantMode := "On-site"
		if r.Date == "2026-11-06" {
			wantMode = "Online via Zoom"
		}
		if r.Mode != wantMode {
			t.Fatal("mode changed", r.Mode)
		}
	}
	if periodCount != 14 {
		t.Fatal(periodCount)
	}
	want := []Seminar{{"Math", "2026-11-07", "11:30", "13:00"}, {"Verbal", "2026-11-07", "13:00", "14:00"}}
	if !reflect.DeepEqual(cfg.Seminars, want) {
		t.Fatal("wrong seminar schedule", cfg.Seminars)
	}
	bad := cfg
	bad.Seminars = append([]Seminar(nil), cfg.Seminars...)
	bad.Seminars[0].End = "08:00"
	if bad.Validate() == nil {
		t.Fatal("invalid seminar time accepted")
	}
	production, err := LoadConfig("../../config/event.production.json")
	if err != nil || production.Event.Sample || !reflect.DeepEqual(production.Rules, cfg.Rules) || !reflect.DeepEqual(production.Seminars, cfg.Seminars) {
		t.Fatal("production and local schedules differ", err)
	}
}
func TestNovemberEveryPeriodCapacityAndDates(t *testing.T) {
	s := novemberService(t)
	for _, r := range s.Config.Rules {
		for _, p := range r.Periods {
			i := testInput()
			i.Level = r.Level
			i.Subject = r.Subject
			i.Date = r.Date
			i.PeriodID = p.ID
			for n := 0; n < p.Capacity; n++ {
				book(t, s, i)
			}
			if _, err := s.Book(testCtx, "full-"+digest(r.Level + r.Subject + r.Date + p.ID)[:20], i); !codeIs(err, "PERIOD_FULL") {
				t.Fatal(r, p, err)
			}
			rows, err := s.Periods(testCtx, r.Level, r.Subject, r.Date)
			if err != nil {
				t.Fatal(err)
			}
			for _, row := range rows {
				if row.ID == p.ID && row.Available {
					t.Fatal("full period visible as available", row)
				}
			}
		}
	}
	for _, scope := range []struct{ level, subject, date, period string }{
		{"Beginner", "Math", "2026-11-06", "P1030_1200"}, {"Beginner", "Verbal", "2026-11-06", "P1000_1100"}, {"Beginner", "Combined", "2026-11-06", "P1300_1430"},
		{"High Level", "Math", "2026-11-07", "P1000_1130"}, {"High Level", "Combined", "2026-11-07", "P1430_1610"}, {"Beginner", "Math", "2026-08-15", "P1100"},
		{"High Level", "Verbal", "2026-11-07", "P1700_1800"}, {"High Level", "Verbal", "2026-11-07", "P1800_2000"},
		{"High Level", "Verbal", "2026-11-06", "P1000_1130"}, {"Beginner", "Math", "2026-11-07", "P1030_1200"},
	} {
		i := testInput()
		i.Level = scope.level
		i.Subject = scope.subject
		i.Date = scope.date
		i.PeriodID = scope.period
		if _, err := s.Book(testCtx, "bad-"+digest(fmt.Sprint(scope))[:20], i); !codeIs(err, "INVALID_PERIOD") {
			t.Fatal("unavailable scope accepted", scope, err)
		}
	}
}
func TestNovemberConcurrentVariableCapacities(t *testing.T) {
	s := novemberService(t)
	other, err := Open(s.DatabaseURL, s.Config, s.Key, s.PublicURL)
	if err != nil {
		t.Fatal(err)
	}
	defer other.DB.Close()
	cases := []struct {
		level, subject, date, period string
		cap                          int
	}{
		{"Beginner", "Math", "2026-11-07", "P1300_1400", 4}, {"Beginner", "Combined", "2026-11-07", "P1400_1530", 5},
		{"Beginner", "Combined", "2026-11-07", "P1530_1700", 6}, {"High Level", "Verbal", "2026-11-06", "P1800_2000", 7},
		{"High Level", "Math", "2026-11-06", "P1800_2000", 8},
	}
	for n, c := range cases {
		i := testInput()
		i.Level = c.level
		i.Subject = c.subject
		i.Date = c.date
		i.PeriodID = c.period
		var accepted, full atomic.Int64
		var wg sync.WaitGroup
		start := make(chan struct{})
		for j := 0; j < 20; j++ {
			wg.Add(1)
			go func(j int) {
				defer wg.Done()
				<-start
				target := s
				if j%2 == 0 {
					target = other
				}
				_, err := target.Book(testCtx, fmt.Sprintf("nov-%d-%d", n, j), i)
				if err == nil {
					accepted.Add(1)
				} else if codeIs(err, "PERIOD_FULL") {
					full.Add(1)
				} else {
					t.Errorf("unexpected error %v", err)
				}
			}(j)
		}
		close(start)
		wg.Wait()
		if int(accepted.Load()) != c.cap || int(full.Load()) != 20-c.cap {
			t.Fatal(c, accepted.Load(), full.Load())
		}
	}
}
func TestLocalResetClearsAllEventsPreservesStaffAndReseeds(t *testing.T) {
	s := testService(t)
	a, err := NewAuth(s, false)
	if err != nil {
		t.Fatal(err)
	}
	a.Cost = bcrypt.MinCost
	if err = a.Bootstrap(testCtx, "staff", "reset-test-password-123", false); err != nil {
		t.Fatal(err)
	}
	login, err := a.Login(testCtx, "staff", "reset-test-password-123")
	if err != nil {
		t.Fatal(err)
	}
	if err = a.StoreSession(testCtx, login, ""); err != nil {
		t.Fatal(err)
	}
	old := book(t, s, testInput())
	assign(t, s, defaultScope(), old.Booking.ID)
	cancelBooking(t, s, latest(t, s, old.Booking.ID))
	s.Config = novemberConfig(t)
	s.Config.Event.ID = "local-test-" + s.Config.Event.ID
	if err = s.migrate(testCtx); err != nil {
		t.Fatal(err)
	}
	s.MailEnabled = true
	i := testInput()
	i.Date = "2026-11-07"
	i.PeriodID = "P1300_1400"
	book(t, s, i)
	var fingerprint, hash string
	var revision int64
	s.DB.QueryRow(`SELECT value FROM settings WHERE name='encryption_key_fingerprint'`).Scan(&fingerprint)
	s.DB.QueryRow(`SELECT password_hash FROM users WHERE username='staff'`).Scan(&hash)
	s.DB.QueryRow(`SELECT revision FROM meta WHERE id=1`).Scan(&revision)
	if err = s.ResetLocalTestData(testCtx); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"bookings", "outbox", "audit", "sequences", "sessions"} {
		var count int
		if err = s.DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil || count != 0 {
			t.Fatal(table, count, err)
		}
	}
	var count int
	s.DB.QueryRow(`SELECT COUNT(*) FROM periods`).Scan(&count)
	if count != 14 {
		t.Fatal("wrong reset catalog", count)
	}
	var afterHash, afterFingerprint string
	var afterRevision int64
	s.DB.QueryRow(`SELECT password_hash FROM users WHERE username='staff'`).Scan(&afterHash)
	s.DB.QueryRow(`SELECT value FROM settings WHERE name='encryption_key_fingerprint'`).Scan(&afterFingerprint)
	s.DB.QueryRow(`SELECT revision FROM meta WHERE id=1`).Scan(&afterRevision)
	if hash != afterHash || fingerprint != afterFingerprint || afterRevision != revision+1 {
		t.Fatal("credentials/meta changed incorrectly")
	}
	if _, err = a.Login(testCtx, "staff", "reset-test-password-123"); err != nil {
		t.Fatal("staff login lost", err)
	}
	if _, err = s.Track(testCtx, old.Token); !codeIs(err, "BOOKING_NOT_FOUND") {
		t.Fatal("old token retained", err)
	}
	next := book(t, s, i)
	assign(t, s, Scope{i.Level, i.Date, i.PeriodID, "Math"}, next.Booking.ID)
	b := latest(t, s, next.Booking.ID)
	if *b.QueueNo != "B-M-001" {
		t.Fatal("queue did not restart", *b.QueueNo)
	}
}
func TestLocalResetDeniedOrSeedFailureRollsBack(t *testing.T) {
	s := testService(t)
	old := book(t, s, testInput()).Booking
	if err := s.ResetLocalTestData(testCtx); !codeIs(err, "RESET_NOT_ALLOWED") {
		t.Fatal(err)
	}
	s.Config.Event.ID = "local-test-regression"
	if err := s.migrate(testCtx); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(`CREATE FUNCTION fail_reset_seed() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'seed failure'; END; $$; CREATE TRIGGER fail_reset BEFORE INSERT ON periods FOR EACH ROW EXECUTE FUNCTION fail_reset_seed()`); err != nil {
		t.Fatal(err)
	}
	if err := s.ResetLocalTestData(testCtx); err == nil {
		t.Fatal("seed failure ignored")
	}
	var count int
	s.DB.QueryRow(`SELECT COUNT(*) FROM bookings WHERE id=$1`, old.ID).Scan(&count)
	if count != 1 {
		t.Fatal("reset was not rolled back")
	}
	s.Config.Event.Sample = false
	if err := s.ResetLocalTestData(testCtx); !codeIs(err, "RESET_NOT_ALLOWED") {
		t.Fatal(err)
	}
}
