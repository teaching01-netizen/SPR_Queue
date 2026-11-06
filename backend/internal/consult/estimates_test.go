package consult

import (
	"fmt"
	"testing"
	"time"
)

var testBangkok = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		panic(err)
	}
	return loc
}()

// eventClock builds an instant on an event date in the event time zone; the
// fixture event runs in Asia/Bangkok, so 11:00 local is 04:00 UTC.
func eventClock(date string, hour, minute int) time.Time {
	at, err := time.ParseInLocation("2006-01-02 15:04", fmt.Sprintf("%s %02d:%02d", date, hour, minute), testBangkok)
	if err != nil {
		panic(err)
	}
	return at
}

func eventStamp(date string, hour, minute int) string {
	return eventClock(date, hour, minute).UTC().Format(time.RFC3339Nano)
}

func queuedBooking(id, level, date, periodID, subject, current, status string, order int, stage Stage) Booking {
	b := Booking{
		Input:          Input{Level: level, Date: date, PeriodID: periodID, Subject: subject},
		ID:             id,
		CurrentSubject: current,
		Status:         status,
		Orders:         map[string]int{},
		Stages:         map[string]Stage{},
	}
	if order > 0 {
		b.Orders[current] = order
	}
	if stage != (Stage{}) {
		b.Stages[current] = stage
	}
	return b
}

func wantEstimate(t *testing.T, got map[string]SlotEstimate, id, date string, hour, minute int, late bool) {
	t.Helper()
	estimate, ok := got[id]
	if !ok {
		t.Fatalf("missing estimate for %s", id)
	}
	want := eventClock(date, hour, minute).UTC().Format(time.RFC3339Nano)
	if estimate.StartAt != want {
		t.Fatalf("%s start = %s, want %s", id, estimate.StartAt, want)
	}
	if estimate.Late != late {
		t.Fatalf("%s late = %v, want %v", id, estimate.Late, late)
	}
}

func wantNoEstimate(t *testing.T, got map[string]SlotEstimate, id string) {
	t.Helper()
	if estimate, ok := got[id]; ok {
		t.Fatalf("%s should have no estimate, got %+v", id, estimate)
	}
}

// A clean room queues from the period start at the configured spacing.
func TestEstimatesBaselineSchedule(t *testing.T) {
	cfg := testConfig(t)
	bookings := []Booking{
		queuedBooking("b1", "Beginner", "2026-08-15", "P1100", "Math", "Math", "Waiting", 1, Stage{}),
		queuedBooking("b2", "Beginner", "2026-08-15", "P1100", "Math", "Math", "Booked", 2, Stage{}),
	}
	got := computeEstimates(bookings, eventClock("2026-08-15", 10, 30), 10, cfg)
	wantEstimate(t, got, "b1", "2026-08-15", 11, 0, false)
	wantEstimate(t, got, "b2", "2026-08-15", 11, 10, false)
}

// The first slot never advertises a time that already passed.
func TestEstimatesNeverInThePast(t *testing.T) {
	cfg := testConfig(t)
	bookings := []Booking{
		queuedBooking("b1", "Beginner", "2026-08-15", "P1100", "Math", "Math", "Waiting", 1, Stage{}),
		queuedBooking("b2", "Beginner", "2026-08-15", "P1100", "Math", "Math", "Waiting", 2, Stage{}),
	}
	got := computeEstimates(bookings, eventClock("2026-08-15", 11, 35), 10, cfg)
	wantEstimate(t, got, "b1", "2026-08-15", 11, 35, false)
	wantEstimate(t, got, "b2", "2026-08-15", 11, 45, false)
}

// A slot that takes 20 minutes instead of 10 pushes every following slot.
func TestEstimatesLateSlotShiftsFollowingSlots(t *testing.T) {
	cfg := testConfig(t)
	served := queuedBooking("served", "Beginner", "2026-08-15", "P1000", "Verbal", "Verbal", "Completed", 0,
		Stage{StartedAt: eventStamp("2026-08-15", 10, 10), EndedAt: eventStamp("2026-08-15", 10, 30)})
	bookings := []Booking{
		served,
		queuedBooking("p1", "Beginner", "2026-08-15", "P1000", "Verbal", "Verbal", "Waiting", 1, Stage{}),
		queuedBooking("p2", "Beginner", "2026-08-15", "P1000", "Verbal", "Verbal", "Waiting", 2, Stage{}),
	}
	got := computeEstimates(bookings, eventClock("2026-08-15", 9, 55), 10, cfg)
	wantEstimate(t, got, "p1", "2026-08-15", 10, 30, false)
	wantEstimate(t, got, "p2", "2026-08-15", 10, 40, false)
}

// Finishing early pulls the queue forward but never before "now".
func TestEstimatesEarlyFinishPullsQueueForward(t *testing.T) {
	cfg := testConfig(t)
	served := queuedBooking("served", "Beginner", "2026-08-15", "P1000", "Verbal", "Verbal", "Completed", 0,
		Stage{StartedAt: eventStamp("2026-08-15", 9, 58), EndedAt: eventStamp("2026-08-15", 10, 15)})
	bookings := []Booking{
		served,
		queuedBooking("p1", "Beginner", "2026-08-15", "P1000", "Verbal", "Verbal", "Waiting", 1, Stage{}),
		queuedBooking("p2", "Beginner", "2026-08-15", "P1000", "Verbal", "Verbal", "Waiting", 2, Stage{}),
	}
	got := computeEstimates(bookings, eventClock("2026-08-15", 10, 5), 10, cfg)
	wantEstimate(t, got, "p1", "2026-08-15", 10, 15, false)
	wantEstimate(t, got, "p2", "2026-08-15", 10, 25, false)

	// Early finish before the period starts still cannot beat the period start.
	early := computeEstimates(bookings, eventClock("2026-08-15", 9, 30), 10, cfg)
	wantEstimate(t, early, "p1", "2026-08-15", 10, 15, false)
}

// A session running past its configured duration keeps sliding with "now".
func TestEstimatesServingOverrunSlides(t *testing.T) {
	cfg := testConfig(t)
	serving := queuedBooking("serving", "Beginner", "2026-08-15", "P1100", "Math", "Math", "Serving", 0,
		Stage{CalledAt: eventStamp("2026-08-15", 11, 0), StartedAt: eventStamp("2026-08-15", 11, 7)})
	bookings := []Booking{
		serving,
		queuedBooking("p1", "Beginner", "2026-08-15", "P1100", "Math", "Math", "Waiting", 1, Stage{}),
		queuedBooking("p2", "Beginner", "2026-08-15", "P1100", "Math", "Math", "Waiting", 2, Stage{}),
	}
	// Expected end is 11:17, but it is already 11:25, so the next slot starts now.
	got := computeEstimates(bookings, eventClock("2026-08-15", 11, 25), 10, cfg)
	wantEstimate(t, got, "p1", "2026-08-15", 11, 25, false)
	wantEstimate(t, got, "p2", "2026-08-15", 11, 35, false)

	// Before the expected end the projection uses startedAt + minutes.
	onTime := computeEstimates(bookings, eventClock("2026-08-15", 11, 10), 10, cfg)
	wantEstimate(t, onTime, "p1", "2026-08-15", 11, 17, false)
}

// While a parent is called but not seated the room frees up at calledAt + minutes.
func TestEstimatesCallingProjectsExpectedEnd(t *testing.T) {
	cfg := testConfig(t)
	calling := queuedBooking("calling", "Beginner", "2026-08-15", "P1100", "Math", "Math", "Calling", 0,
		Stage{CalledAt: eventStamp("2026-08-15", 11, 5)})
	bookings := []Booking{
		calling,
		queuedBooking("p1", "Beginner", "2026-08-15", "P1100", "Math", "Math", "Waiting", 1, Stage{}),
	}
	got := computeEstimates(bookings, eventClock("2026-08-15", 11, 6), 10, cfg)
	wantEstimate(t, got, "p1", "2026-08-15", 11, 16, false)
}

// Only a still-running session carries over into the next period; a session
// that already ended before the next period starts does not shift it.
func TestEstimatesCrossPeriodOnlyRunningSessionDelays(t *testing.T) {
	cfg := testConfig(t)
	running := queuedBooking("running", "Beginner", "2026-08-15", "P1000", "Verbal", "Verbal", "Serving", 0,
		Stage{StartedAt: eventStamp("2026-08-15", 10, 55)})
	waiting := []Booking{
		running,
		queuedBooking("p1", "Beginner", "2026-08-15", "P1100", "Verbal", "Verbal", "Waiting", 1, Stage{}),
	}
	got := computeEstimates(waiting, eventClock("2026-08-15", 11, 0), 10, cfg)
	wantEstimate(t, got, "p1", "2026-08-15", 11, 5, false)

	finished := queuedBooking("finished", "Beginner", "2026-08-15", "P1000", "Verbal", "Verbal", "Completed", 0,
		Stage{StartedAt: eventStamp("2026-08-15", 10, 20), EndedAt: eventStamp("2026-08-15", 10, 40)})
	fresh := []Booking{
		finished,
		queuedBooking("p2", "Beginner", "2026-08-15", "P1100", "Verbal", "Verbal", "Waiting", 1, Stage{}),
	}
	after := computeEstimates(fresh, eventClock("2026-08-15", 10, 50), 10, cfg)
	wantEstimate(t, after, "p2", "2026-08-15", 11, 0, false)
}

// A chain that passes the period end is flagged late instead of hidden.
func TestEstimatesLateFlagAfterPeriodEnd(t *testing.T) {
	cfg := testConfig(t)
	bookings := []Booking{
		queuedBooking("p1", "Beginner", "2026-08-15", "P1100", "Verbal", "Verbal", "Waiting", 1, Stage{}),
		queuedBooking("p2", "Beginner", "2026-08-15", "P1100", "Verbal", "Verbal", "Waiting", 2, Stage{}),
	}
	got := computeEstimates(bookings, eventClock("2026-08-15", 11, 55), 10, cfg)
	wantEstimate(t, got, "p1", "2026-08-15", 11, 55, false)
	wantEstimate(t, got, "p2", "2026-08-15", 12, 5, true)
}

// Rows without a saved position, finished rooms and inactive rows consume no
// slot and get no estimate.
func TestEstimatesSkipIneligibleRows(t *testing.T) {
	cfg := testConfig(t)
	bookings := []Booking{
		queuedBooking("cancelled", "Beginner", "2026-08-15", "P1100", "Math", "Math", "Cancelled", 1, Stage{}),
		queuedBooking("noshow", "Beginner", "2026-08-15", "P1100", "Math", "Math", "No-show", 2, Stage{}),
		queuedBooking("unpositioned", "Beginner", "2026-08-15", "P1100", "Math", "Math", "Waiting", 0, Stage{}),
		queuedBooking("done-current-room", "Beginner", "2026-08-15", "P1100", "Math", "Math", "Waiting", 3,
			Stage{StartedAt: eventStamp("2026-08-15", 10, 50), EndedAt: eventStamp("2026-08-15", 11, 0)}),
		queuedBooking("valid", "Beginner", "2026-08-15", "P1100", "Math", "Math", "Waiting", 1, Stage{}),
	}
	got := computeEstimates(bookings, eventClock("2026-08-15", 10, 0), 10, cfg)
	for _, id := range []string{"cancelled", "noshow", "unpositioned", "done-current-room"} {
		wantNoEstimate(t, got, id)
	}
	// The skipped rows do not shift the valid row: it is still first in line.
	wantEstimate(t, got, "valid", "2026-08-15", 11, 0, false)
}

// A booking whose period was removed from the event configuration is left
// alone instead of being projected against a fake floor time.
func TestEstimatesRetiredPeriodSkipped(t *testing.T) {
	cfg := testConfig(t)
	bookings := []Booking{
		queuedBooking("retired", "Beginner", "2026-08-15", "P_REMOVED", "Math", "Math", "Waiting", 1, Stage{}),
		queuedBooking("valid", "Beginner", "2026-08-15", "P1100", "Math", "Math", "Waiting", 1, Stage{}),
	}
	got := computeEstimates(bookings, eventClock("2026-08-15", 10, 0), 10, cfg)
	wantNoEstimate(t, got, "retired")
	wantEstimate(t, got, "valid", "2026-08-15", 11, 0, false)
}

// The chain uses whatever minutes the admin configured.
func TestEstimatesRespectConfiguredMinutes(t *testing.T) {
	cfg := testConfig(t)
	bookings := []Booking{
		queuedBooking("b1", "Beginner", "2026-08-15", "P1100", "Math", "Math", "Waiting", 1, Stage{}),
		queuedBooking("b2", "Beginner", "2026-08-15", "P1100", "Math", "Math", "Waiting", 2, Stage{}),
	}
	got := computeEstimates(bookings, eventClock("2026-08-15", 10, 0), 15, cfg)
	wantEstimate(t, got, "b1", "2026-08-15", 11, 0, false)
	wantEstimate(t, got, "b2", "2026-08-15", 11, 15, false)
}

// Levels and rooms never share a queue chain.
func TestEstimatesRoomsAndLevelsAreSeparate(t *testing.T) {
	cfg := testConfig(t)
	// A finished Math session must not shift the Verbal room's queue.
	mathServed := queuedBooking("math-served", "Beginner", "2026-08-15", "P1100", "Math", "Math", "Completed", 0,
		Stage{StartedAt: eventStamp("2026-08-15", 10, 30), EndedAt: eventStamp("2026-08-15", 10, 40)})
	got := computeEstimates([]Booking{
		mathServed,
		queuedBooking("verbal", "Beginner", "2026-08-15", "P1000", "Verbal", "Verbal", "Waiting", 1, Stage{}),
	}, eventClock("2026-08-15", 10, 0), 10, cfg)
	wantEstimate(t, got, "verbal", "2026-08-15", 10, 0, false)

	// A finished Beginner Verbal session must not shift High Level Verbal.
	beginnerServed := queuedBooking("beginner-served", "Beginner", "2026-08-15", "P1000", "Verbal", "Verbal", "Completed", 0,
		Stage{StartedAt: eventStamp("2026-08-15", 10, 20), EndedAt: eventStamp("2026-08-15", 10, 40)})
	levels := computeEstimates([]Booking{
		beginnerServed,
		queuedBooking("high", "High Level", "2026-08-15", "P1000", "Verbal", "Verbal", "Waiting", 1, Stage{}),
	}, eventClock("2026-08-15", 10, 0), 10, cfg)
	wantEstimate(t, levels, "high", "2026-08-15", 10, 0, false)
}

// A Combined parent leaving one room seeds that room's chain, while the parent
// itself only joins the next room's chain once its position there is saved.
func TestEstimatesCombinedHandoffSeedsFinishedRoom(t *testing.T) {
	cfg := testConfig(t)
	handoff := Booking{
		Input:          Input{Level: "Beginner", Date: "2026-08-15", PeriodID: "P1500_1630", Subject: "Combined"},
		ID:             "combined",
		CurrentSubject: "Verbal",
		Status:         "Waiting",
		Orders:         map[string]int{},
		Stages: map[string]Stage{
			"Math": {CalledAt: eventStamp("2026-08-15", 16, 5), StartedAt: eventStamp("2026-08-15", 16, 10), EndedAt: eventStamp("2026-08-15", 16, 40)},
		},
	}
	bookings := []Booking{
		handoff,
		queuedBooking("math-pending", "Beginner", "2026-08-15", "P1630_1800", "Math", "Math", "Waiting", 1, Stage{}),
	}
	got := computeEstimates(bookings, eventClock("2026-08-15", 16, 30), 10, cfg)
	wantNoEstimate(t, got, "combined")
	wantEstimate(t, got, "math-pending", "2026-08-15", 16, 40, false)

	handoff.Orders["Verbal"] = 1
	withPosition := computeEstimates([]Booking{handoff}, eventClock("2026-08-15", 16, 30), 10, cfg)
	wantEstimate(t, withPosition, "combined", "2026-08-15", 16, 30, false)
}

// The estimate clock is the event time zone, not the server's local zone.
func TestEstimatesUseEventTimezone(t *testing.T) {
	cfg := testConfig(t)
	bookings := []Booking{
		queuedBooking("b1", "Beginner", "2026-08-15", "P1100", "Math", "Math", "Waiting", 1, Stage{}),
	}
	got := computeEstimates(bookings, eventClock("2026-08-15", 10, 0), 10, cfg)
	if got["b1"].StartAt != "2026-08-15T04:00:00Z" {
		t.Fatalf("start = %s, want 2026-08-15T04:00:00Z (11:00 Bangkok)", got["b1"].StartAt)
	}
}

// Nothing to project means no estimate map at all (the JSON field stays out).
func TestEstimatesEmptyWhenNothingQueued(t *testing.T) {
	cfg := testConfig(t)
	if got := computeEstimates(nil, eventClock("2026-08-15", 10, 0), 10, cfg); got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
	completed := queuedBooking("done", "Beginner", "2026-08-15", "P1100", "Math", "Math", "Completed", 0,
		Stage{StartedAt: eventStamp("2026-08-15", 10, 0), EndedAt: eventStamp("2026-08-15", 10, 10)})
	if got := computeEstimates([]Booking{completed}, eventClock("2026-08-15", 10, 20), 10, cfg); got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}
