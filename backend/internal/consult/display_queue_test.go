package consult

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestDisplayUpcomingOrderAndPrivacy(t *testing.T) {
	s := testService(t)
	scope := defaultScope()
	ids := []string{}
	for n := 0; n < 4; n++ {
		b := book(t, s, testInput())
		ids = append(ids, b.Booking.ID)
		if n != 3 {
			transition(t, s, b.Booking.ID, "Waiting")
		}
	}
	assign(t, s, scope, ids[2], ids[0], ids[1], ids[3])
	live := transition(t, s, ids[2], "Calling")
	out, err := s.Display(testCtx, scope.Level, scope.Date)
	if err != nil {
		t.Fatal(err)
	}
	codes := []string{*latest(t, s, ids[0]).QueueNo, *latest(t, s, ids[1]).QueueNo}
	if len(out["Math"]) != 1 || out["Math"][0].QueueNo != *live.QueueNo || len(out["MathNext"]) != 2 || out["MathNext"][0].QueueNo != codes[0] || out["MathNext"][1].QueueNo != codes[1] {
		t.Fatal("upcoming differs from saved order", out)
	}
	if len(out["VerbalNext"]) != 0 {
		t.Fatal("wrong room")
	}
	payload, _ := json.Marshal(out)
	for _, private := range []string{"nickname", "email", "school", "questions", "parent@example.com", "token", "Example"} {
		if strings.Contains(string(payload), private) {
			t.Fatal("public data leak", private)
		}
	}
	transition(t, s, ids[0], "No-show")
	out, _ = s.Display(testCtx, scope.Level, scope.Date)
	if len(out["MathNext"]) != 1 || out["MathNext"][0].QueueNo != codes[1] {
		t.Fatal("No-show/unchecked included", out)
	}
	assign(t, s, scope, ids[1], ids[3])
	out, _ = s.Display(testCtx, scope.Level, scope.Date)
	if len(out["MathNext"]) != 1 {
		t.Fatal("reorder didn't apply")
	}
}
func TestUpcomingCombinedRoomAndPeriodScope(t *testing.T) {
	s := &Service{Config: novemberConfig(t)}
	checked := "checked"
	period := "P1700_1800"
	code := func(v string) *string { return &v }
	b := func(q, subject, status, room, p string, order int) Booking {
		return Booking{Input: Input{Level: "Beginner", Subject: subject, Date: "2026-11-07", PeriodID: p}, QueueNo: code(q), CurrentSubject: room, Status: status, CheckedInAt: &checked, Orders: map[string]int{room: order}}
	}
	bookings := []Booking{b("B-V-010", "Verbal", "Calling", "Verbal", period, 1), b("B-MV-003", "Combined", "Waiting", "Math", "P1300_1430", 1), b("B-V-002", "Verbal", "Waiting", "Verbal", period, 2), b("B-V-001", "Verbal", "Waiting", "Verbal", period, 1), b("B-V-004", "Verbal", "Waiting", "Verbal", period, 0), b("B-V-005", "Verbal", "Waiting", "Verbal", "P1000_1100", 1)}
	v := s.upcoming(bookings, "Beginner", "2026-11-07", "Verbal")
	if len(v) != 2 || !reflect.DeepEqual([]string{v[0].QueueNo, v[1].QueueNo}, []string{"B-V-001", "B-V-002"}) {
		t.Fatal("wrong subject, period or order", v)
	}
	bookings[1].CurrentSubject = "Verbal"
	bookings[1].Orders = map[string]int{"Math": 1}
	if len(s.upcoming(bookings, "Beginner", "2026-11-07", "Verbal")) != 2 {
		t.Fatal("unordered second stage")
	}
	bookings[0].Status = "Completed"
	v = s.upcoming(bookings, "Beginner", "2026-11-07", "Verbal")
	if len(v) != 1 || v[0].QueueNo != "B-V-005" {
		t.Fatal("idle earliest ready period", v)
	}
	if len(s.upcoming(bookings, "High Level", "2026-11-07", "Verbal")) != 0 || len(s.upcoming(bookings, "Beginner", "2026-11-06", "Verbal")) != 0 {
		t.Fatal("scope leak")
	}
}
