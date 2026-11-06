package consult

import "testing"

func TestMathAndVerbalRunConcurrently(t *testing.T) {
	for _, combined := range []bool{false, true} {
		name := "single-subject"
		if combined {
			name = "combined-different-students"
		}
		t.Run(name, func(t *testing.T) {
			s := testService(t)
			students := make([]Confirmation, 2)
			ready := make([]Booking, 2)
			for n, stream := range []string{"Math", "Verbal"} {
				i := testInput()
				i.Subject = stream
				if combined {
					i.Subject, i.PeriodID = "Combined", "P1300"
				}
				students[n] = book(t, s, i)
				b := students[n].Booking
				if combined {
					var err error
					b, err = s.ChooseStartSubject(testCtx, b.ID, stream, b.Revision, "staff")
					if err != nil {
						t.Fatal(err)
					}
				}
				assign(t, s, Scope{i.Level, i.Date, i.PeriodID, stream}, b.ID)
				ready[n] = transition(t, s, b.ID, "Waiting")
			}
			// Release both requests together; both rooms must remain active after commit.
			for _, next := range []string{"Calling", "Serving"} {
				start := make(chan struct{})
				results := make(chan error, 2)
				for _, b := range ready {
					go func(b Booking) {
						<-start
						_, err := s.Status(testCtx, b.ID, next, b.Revision, "staff", "")
						results <- err
					}(b)
				}
				close(start)
				for range ready {
					if err := <-results; err != nil {
						t.Fatalf("parallel %s: %v", next, err)
					}
				}
				display, err := s.Display(testCtx, ready[0].Level, ready[0].Date)
				if err != nil {
					t.Fatal(err)
				}
				for n, stream := range []string{"Math", "Verbal"} {
					ready[n] = latest(t, s, ready[n].ID)
					if len(display[stream]) != 1 || display[stream][0].State != next || display[stream][0].QueueNo != *ready[n].QueueNo {
						t.Fatalf("%s display: %+v", stream, display)
					}
					parent, err := s.Track(testCtx, students[n].Token)
					if err != nil || parent.Status != next || parent.CurrentSubject != stream || len(parent.CurrentQueues) != 1 || parent.CurrentQueues[0].QueueNo != *ready[n].QueueNo {
						t.Fatalf("%s tracking: %+v, %v", stream, parent, err)
					}
				}
			}
			transition(t, s, ready[0].ID, "Completed")
			verbal := latest(t, s, ready[1].ID)
			if verbal.Status != "Serving" || verbal.Stages["Verbal"] != ready[1].Stages["Verbal"] || verbal.Revision != ready[1].Revision {
				t.Fatal("finishing Math changed the active Verbal consultation")
			}
			if combined {
				math := latest(t, s, ready[0].ID)
				if math.Status != "Waiting" || math.CurrentSubject != "Verbal" {
					t.Fatal("missing Combined handoff")
				}
				assign(t, s, Scope{math.Level, math.Date, math.PeriodID, "Verbal"}, math.ID)
				math = latest(t, s, math.ID)
				if _, err := s.Status(testCtx, math.ID, "Calling", math.Revision, "staff", ""); !codeIs(err, "STREAM_BUSY") {
					t.Fatalf("handoff entered occupied Verbal room: %v", err)
				}
			}
		})
	}
}
