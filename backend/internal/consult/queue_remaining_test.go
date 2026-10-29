package consult

import "testing"

func TestTrackRemainingQueueFollowsCurrentOrder(t *testing.T) {
	s := novemberService(t)
	ids := []string{}
	tokens := []string{}
	for n := 0; n < 5; n++ {
		b := book(t, s, Input{Email: "parent@example.com", Nickname: "Private", School: "School", Level: "Beginner", Subject: "Math", Date: "2026-11-07", PeriodID: "P1030_1200", Questions: []string{"one", "two"}, SeminarConfirmed: true})
		ids = append(ids, b.Booking.ID)
		tokens = append(tokens, b.Token)
	}
	own := func(want *int, state string) {
		t.Helper()
		v, err := s.Track(testCtx, tokens[4])
		if err != nil {
			t.Fatal(err)
		}
		if v.QueueCountState != state || (v.QueuesUntilTurn == nil) != (want == nil) || (want != nil && *v.QueuesUntilTurn != *want) {
			t.Fatalf("want %v/%s, got %+v", want, state, v)
		}
	}
	value := func(n int) *int { return &n }
	own(nil, "awaiting-check-in")
	for _, id := range ids {
		transition(t, s, id, "Waiting")
	}
	own(nil, "awaiting-order")
	scope := Scope{"Beginner", "2026-11-07", "P1030_1200", "Math"}
	assign(t, s, scope, ids...)
	transition(t, s, ids[0], "Calling")
	own(value(4), "ready")
	_, err := s.Status(testCtx, ids[1], "Cancelled", latest(t, s, ids[1]).Revision, "staff", "cancel for counter test")
	if err != nil {
		t.Fatal(err)
	}
	own(value(3), "ready")
	transition(t, s, ids[2], "No-show")
	own(value(2), "ready")
	transition(t, s, ids[0], "Serving")
	own(value(2), "ready")
	transition(t, s, ids[0], "Completed")
	own(value(2), "ready")
	assign(t, s, scope, ids[4], ids[3])
	own(value(1), "ready")
	transition(t, s, ids[4], "Calling")
	own(value(0), "current")
	transition(t, s, ids[4], "Serving")
	own(value(0), "current")
	transition(t, s, ids[4], "Completed")
	own(nil, "inactive")
}
func TestRemainingAcrossPeriodsAndCombinedHandoff(t *testing.T) {
	s := &Service{Config: novemberConfig(t)}
	check := "checked"
	code := func(v string) *string { return &v }
	makeBooking := func(id, subject, period string, order int) Booking {
		return Booking{ID: id, Input: Input{Level: "Beginner", Subject: subject, Date: "2026-11-07", PeriodID: period}, QueueNo: code(id), CurrentSubject: "Math", Status: "Waiting", CheckedInAt: &check, Orders: map[string]int{"Math": order}}
	}
	live := makeBooking("live", "Math", "P1030_1200", 1)
	live.Status = "Calling"
	live.Stages = map[string]Stage{"Math": {CalledAt: "2026-11-07T03:00:00Z"}}
	before := makeBooking("ready", "Math", "P1030_1200", 2)
	own := makeBooking("combined", "Combined", "P1300_1430", 1)
	bookings := []Booking{live, before, own}
	n, state := s.queuesUntilTurn(bookings, own)
	if n == nil || *n != 2 || state != "ready" {
		t.Fatal("count earlier remaining period", n, state)
	}
	before.Level = "High Level"
	bookings[1] = before
	n, _ = s.queuesUntilTurn(bookings, own)
	if n == nil || *n != 1 {
		t.Fatal("mixed levels")
	}
	// The Combined code survives, but second-room order must be assigned before counting.
	own.CurrentSubject = "Verbal"
	own.Stages = map[string]Stage{"Math": {EndedAt: "2026-11-07T05:00:00Z"}}
	own.Orders = map[string]int{}
	bookings[2] = own
	n, state = s.queuesUntilTurn(bookings, own)
	if n != nil || state != "awaiting-order" {
		t.Fatal("guessed second room position")
	}
	own.Orders = map[string]int{"Verbal": 1}
	bookings[2] = own
	n, state = s.queuesUntilTurn(bookings, own)
	if n == nil || *n != 1 || state != "ready" {
		t.Fatal("combined new room")
	}
	own = makeBooking("old-period", "Math", "P1030_1200", 1)
	live = makeBooking("later", "Math", "P1700_1800", 1)
	live.Status = "Completed"
	live.Stages = map[string]Stage{"Math": {CalledAt: "2026-11-07T10:00:00Z", EndedAt: "2026-11-07T10:10:00Z"}}
	n, state = s.queuesUntilTurn([]Booking{live, own}, own)
	if n != nil || state != "staff-review" {
		t.Fatal("misleading number for past period")
	}
}
