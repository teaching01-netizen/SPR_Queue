package consult

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

func TestRegressionBookingFixture(t *testing.T) {
	cfg := testConfig(t)
	expected := map[string][]Period{
		"Math":     {{"P1100", "11:00", "12:00", 4}, {"P1630_1800", "16:30", "18:00", 6}, {"P1800", "18:00", "19:00", 4}, {"P1900", "19:00", "20:00", 4}},
		"Verbal":   {{"P1000", "10:00", "11:00", 4}, {"P1100", "11:00", "12:00", 4}, {"P1630_1800", "16:30", "18:00", 6}, {"P1800", "18:00", "19:00", 4}, {"P1900", "19:00", "20:00", 4}},
		"Combined": {{"P1300", "13:00", "14:00", 4}, {"P1400", "14:00", "15:00", 4}, {"P1500_1630", "15:00", "16:30", 6}},
	}
	for _, r := range cfg.Rules {
		want := expected[r.Subject]
		if len(r.Periods) != len(want) {
			t.Fatalf("%s %s has wrong period count", r.Level, r.Subject)
		}
		for n, p := range r.Periods {
			if p != want[n] {
				t.Fatalf("%s %s unexpected period: %+v", r.Level, r.Subject, p)
			}
		}
	}
	i := testInput()
	i.PeriodID = "P1300"
	if _, _, err := cfg.Match(i); !codeIs(err, "INVALID_PERIOD") {
		t.Fatal("removed Math period remains bookable", err)
	}
	i.Subject = "Combined"
	i.PeriodID = "P1500"
	if _, _, err := cfg.Match(i); !codeIs(err, "INVALID_PERIOD") {
		t.Fatal("Combined accepted retired period", err)
	}
}

func TestExtendedCombinedConcurrentCapacity(t *testing.T) {
	s := testService(t)
	other, err := Open(s.DatabaseURL, s.Config, s.Key, s.PublicURL)
	if err != nil {
		t.Fatal(err)
	}
	defer other.DB.Close()
	i := testInput()
	i.Subject = "Combined"
	i.PeriodID = "P1500_1630"
	var accepted, full atomic.Int64
	var wg sync.WaitGroup
	start := make(chan struct{})
	for n := 0; n < 20; n++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			<-start
			target := s
			if n%2 == 0 {
				target = other
			}
			_, err := target.Book(testCtx, fmt.Sprintf("extended-%d", n), i)
			if err == nil {
				accepted.Add(1)
			} else if codeIs(err, "PERIOD_FULL") {
				full.Add(1)
			} else {
				t.Errorf("unexpected error: %v", err)
			}
		}(n)
	}
	close(start)
	wg.Wait()
	if accepted.Load() != 6 || full.Load() != 14 {
		t.Fatalf("accepted %d full %d", accepted.Load(), full.Load())
	}
	periods, err := s.Periods(testCtx, i.Level, i.Subject, i.Date)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range periods {
		if p.ID == i.PeriodID && p.Available {
			t.Fatal("full six-slot period remains available")
		}
	}
	// The regular Combined period and extended Verbal period keep their own quotas.
	i.PeriodID = "P1300"
	for n := 0; n < 4; n++ {
		book(t, s, i)
	}
	if _, err = s.Book(testCtx, "hourly-fifth", i); !codeIs(err, "PERIOD_FULL") {
		t.Fatal("hourly quota not enforced", err)
	}
	i.Subject = "Verbal"
	i.PeriodID = "P1630_1800"
	for n := 0; n < 6; n++ {
		book(t, s, i)
	}
	if _, err = s.Book(testCtx, "verbal-seventh", i); !codeIs(err, "PERIOD_FULL") {
		t.Fatal("Verbal quota not enforced", err)
	}
	// A cancelled booking releases exactly one place in the extended period.
	list, err := s.StaffList(testCtx)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range list.Bookings {
		if b.PeriodID == "P1500_1630" {
			if _, err = s.Status(testCtx, b.ID, "Cancelled", b.Revision, "staff", "test cancellation"); err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	i.Subject = "Combined"
	i.PeriodID = "P1500_1630"
	book(t, s, i)
	if _, err = s.Book(testCtx, "extended-seventh", i); !codeIs(err, "PERIOD_FULL") {
		t.Fatal("extended period exceeded six after replacement", err)
	}
}

func TestExtendedSoloConcurrentCapacity(t *testing.T) {
	s := testService(t)
	other, err := Open(s.DatabaseURL, s.Config, s.Key, s.PublicURL)
	if err != nil {
		t.Fatal(err)
	}
	defer other.DB.Close()
	for _, subject := range []string{"Math", "Verbal"} {
		i := testInput()
		i.Subject = subject
		i.PeriodID = "P1630_1800"
		var accepted, full atomic.Int64
		var wg sync.WaitGroup
		start := make(chan struct{})
		for n := 0; n < 20; n++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				<-start
				target := s
				if n%2 == 0 {
					target = other
				}
				_, err := target.Book(testCtx, fmt.Sprintf("%s-solo-%d", subject, n), i)
				if err == nil {
					accepted.Add(1)
				} else if codeIs(err, "PERIOD_FULL") {
					full.Add(1)
				} else {
					t.Errorf("unexpected error: %v", err)
				}
			}(n)
		}
		close(start)
		wg.Wait()
		if accepted.Load() != 6 || full.Load() != 14 {
			t.Fatalf("%s accepted %d full %d", subject, accepted.Load(), full.Load())
		}
		periods, err := s.Periods(testCtx, i.Level, subject, i.Date)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range periods {
			if p.ID == i.PeriodID && (p.Start != "16:30" || p.End != "18:00" || p.Available) {
				t.Fatalf("wrong extended %s availability: %+v", subject, p)
			}
		}
		i.PeriodID = "P1700"
		if _, err := s.Book(testCtx, "retired-"+subject, i); !codeIs(err, "INVALID_PERIOD") {
			t.Fatal("old hourly slot accepted", err)
		}
	}
	i := testInput()
	i.Subject = "Verbal"
	for _, id := range []string{"P1300", "P1400", "P1500"} {
		i.PeriodID = id
		if _, err := s.Book(testCtx, "retired-verbal-"+id, i); !codeIs(err, "INVALID_PERIOD") {
			t.Fatal("removed Verbal afternoon slot accepted", err)
		}
	}
}
