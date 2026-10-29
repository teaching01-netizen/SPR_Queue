package consult

import (
	"sync"
	"testing"
)

func TestCombinedEitherSubjectFirst(t *testing.T) {
	for _, first := range []string{"Math", "Verbal"} {
		t.Run(first, func(t *testing.T) {
			s := testService(t)
			i := testInput()
			i.Subject = "Combined"
			i.PeriodID = "P1300"
			c := book(t, s, i)
			b := c.Booking
			scope := Scope{i.Level, i.Date, i.PeriodID, "Math"}
			assign(t, s, scope, b.ID)
			b = transition(t, s, b.ID, "Waiting")
			code, checked, revision := *b.QueueNo, *b.CheckedInAt, b.Revision
			b, err := s.ChooseStartSubject(testCtx, b.ID, first, revision, "staff")
			if err != nil {
				t.Fatal(err)
			}
			if first == "Verbal" {
				if b.Orders["Math"] != 0 || b.Orders["Verbal"] != 0 {
					t.Fatal("old room order retained")
				}
				if _, err = s.Status(testCtx, b.ID, "Calling", b.Revision, "staff", ""); !codeIs(err, "QUEUE_NOT_READY") {
					t.Fatal("calling without reassignment", err)
				}
				if _, err = s.ChooseStartSubject(testCtx, b.ID, "Math", revision, "staff"); !codeIs(err, "REVISION_CONFLICT") {
					t.Fatal("stale switch", err)
				}
			}
			scope.Stream = first
			assign(t, s, scope, b.ID)
			b = transition(t, s, b.ID, "Calling")
			second := "Verbal"
			if first == "Verbal" {
				second = "Math"
			}
			if _, err = s.ChooseStartSubject(testCtx, b.ID, second, b.Revision, "staff"); !codeIs(err, "START_SUBJECT_LOCKED") {
				t.Fatal("changed active room", err)
			}
			b = transition(t, s, b.ID, "Waiting")
			if _, err = s.ChooseStartSubject(testCtx, b.ID, second, b.Revision, "staff"); !codeIs(err, "START_SUBJECT_LOCKED") {
				t.Fatal("changed previously called room", err)
			}
			transition(t, s, b.ID, "Calling")
			transition(t, s, b.ID, "Serving")
			b = transition(t, s, b.ID, "Completed")
			if b.Status != "Waiting" || b.CurrentSubject != second || b.Stages[first].EndedAt == "" || b.CompletedAt != "" || b.Counter != "" {
				t.Fatalf("incorrect handoff: %+v", b)
			}
			if *b.QueueNo != code || *b.CheckedInAt != checked {
				t.Fatal("identity/check-in changed")
			}
			if _, err = s.Status(testCtx, b.ID, "Calling", b.Revision, "staff", ""); !codeIs(err, "QUEUE_NOT_READY") {
				t.Fatal("second room ready before assign", err)
			}
			if _, err = s.ChooseStartSubject(testCtx, b.ID, first, b.Revision, "staff"); !codeIs(err, "START_SUBJECT_LOCKED") {
				t.Fatal("reopened completed subject", err)
			}
			scope.Stream = second
			assign(t, s, scope, b.ID)
			transition(t, s, b.ID, "Calling")
			transition(t, s, b.ID, "Serving")
			b = transition(t, s, b.ID, "Completed")
			if b.Status != "Completed" || b.CompletedAt == "" || b.Stages["Math"].EndedAt == "" || b.Stages["Verbal"].EndedAt == "" || *b.QueueNo != code {
				t.Fatalf("not both completed: %+v", b)
			}
			parent, err := s.Track(testCtx, c.Token)
			if err != nil || parent.Status != "Completed" || *parent.QueueNo != code {
				t.Fatal("parent projection", parent, err)
			}
			audits, err := s.Audit(testCtx)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, a := range audits {
				if a.Action == "CHOOSE_START_SUBJECT" && a.Actor == "staff" {
					found = true
				}
			}
			if first == "Verbal" && !found {
				t.Fatal("missing room switch audit")
			}
		})
	}
}

func TestStartSubjectRestrictionsAndConflict(t *testing.T) {
	s := testService(t)
	solo := book(t, s, testInput()).Booking
	if _, err := s.ChooseStartSubject(testCtx, solo.ID, "Verbal", solo.Revision, "staff"); !codeIs(err, "START_SUBJECT_LOCKED") {
		t.Fatal("solo switch", err)
	}
	i := testInput()
	i.Subject = "Combined"
	i.PeriodID = "P1400"
	b := book(t, s, i).Booking
	if _, err := s.ChooseStartSubject(testCtx, b.ID, "Combined", b.Revision, "staff"); !codeIs(err, "VALIDATION_ERROR") {
		t.Fatal("invalid room", err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for n := 0; n < 2; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.ChooseStartSubject(testCtx, b.ID, "Verbal", b.Revision, "staff")
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	accepted, conflicts := 0, 0
	for err := range results {
		if err == nil {
			accepted++
		} else if codeIs(err, "REVISION_CONFLICT") {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if accepted != 1 || conflicts != 1 {
		t.Fatal("concurrent room switch", accepted, conflicts)
	}
	b = latest(t, s, b.ID)
	secondInput := testInput()
	secondInput.Subject = "Combined"
	secondInput.PeriodID = "P1500_1630"
	missed := book(t, s, secondInput).Booking
	transition(t, s, missed.ID, "Waiting")
	missed = transition(t, s, missed.ID, "No-show")
	switched, err := s.ChooseStartSubject(testCtx, missed.ID, "Verbal", missed.Revision, "staff")
	if err != nil || switched.Status != "No-show" || switched.CheckedInAt == nil {
		t.Fatal("no-show switch", switched, err)
	}
	missed = transition(t, s, missed.ID, "Waiting")
	if missed.CurrentSubject != "Verbal" || missed.CheckedInAt == nil {
		t.Fatal("no-show recovery", missed)
	}
	_, err = s.Status(testCtx, missed.ID, "Cancelled", missed.Revision, "staff", "test cancellation")
	if err != nil {
		t.Fatal(err)
	}
	missed = latest(t, s, missed.ID)
	if _, err = s.ChooseStartSubject(testCtx, missed.ID, "Math", missed.Revision, "staff"); !codeIs(err, "START_SUBJECT_LOCKED") {
		t.Fatal("cancelled switch", err)
	}
	s.Config.Event.ID = "different-event"
	if _, err := s.ChooseStartSubject(testCtx, b.ID, "Math", b.Revision, "staff"); !codeIs(err, "BOOKING_NOT_FOUND") {
		t.Fatal("cross event", err)
	}
}
