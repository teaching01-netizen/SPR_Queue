package consult

import (
	"sort"
	"time"
)

// SlotEstimate is a display-only projection of when a queued slot should start.
// StartAt is an RFC3339Nano UTC timestamp; Late means the predicted start is
// already past the end of the slot's own period.
type SlotEstimate struct {
	StartAt string `json:"startAt"`
	Late    bool   `json:"late,omitempty"`
}

type roomKey struct{ level, date, stream string }

type pendingSlot struct {
	id    string
	start time.Time
	end   time.Time
	order int
	ready bool
}

// computeEstimates projects the consult start time of each queued slot.
//
// A room (level + date + room stream) is one chain: a slot starts when the
// previous one starts plus the configured minutes. Real stage timestamps move
// the chain, so a slot that takes 20 minutes instead of 10 delays every slot
// after it; finishing early pulls the queue forward but never into the past.
// A slot that is still being served keeps sliding with "now" so the queue never
// advertises a start time that has already gone by. Levels, dates and rooms
// (Math/Verbal) never share a chain.
//
// Only rows that hold a saved position are projected. Rows without a position,
// cancelled/no-show rows, rows whose period left the event configuration and
// rows whose current room already ended are skipped and consume no slot.
// Checked-in, ready parents can start before their booked period. They precede
// absent parents, whose estimates still respect the booked period start.
func computeEstimates(bookings []Booking, now time.Time, minutes int, cfg Config) map[string]SlotEstimate {
	if minutes <= 0 || len(bookings) == 0 {
		return nil
	}
	if now.IsZero() {
		now = time.Now()
	}
	now = now.UTC()
	loc, err := time.LoadLocation(cfg.Event.Timezone)
	if err != nil {
		loc = time.UTC
	}
	minuteStep := time.Duration(minutes) * time.Minute

	// The chain cursor for a room starts at the end of its last real session.
	// A called/serving session is projected forward from the configured
	// duration, and a session that has overrun keeps following "now".
	cursor := map[roomKey]time.Time{}
	pending := map[roomKey][]pendingSlot{}
	rooms := map[roomKey]bool{}
	for n := range bookings {
		b := &bookings[n]
		for _, stream := range []string{"Math", "Verbal"} {
			key := roomKey{b.Level, b.Date, stream}
			stage := b.Stages[stream]
			// A Combined parent leaves one room and continues in the other; the
			// finished room's next queue still starts after that real end.
			if ended, parseErr := time.Parse(time.RFC3339Nano, stage.EndedAt); parseErr == nil && ended.After(cursor[key]) {
				cursor[key] = ended
			}
			if b.CurrentSubject != stream {
				continue
			}
			if contains([]string{"Calling", "Serving"}, b.Status) {
				if end, ok := projectedSessionEnd(b.Status, stage, now, minuteStep); ok && end.After(cursor[key]) {
					cursor[key] = end
				}
			}
			if !contains([]string{"Booked", "Waiting"}, b.Status) || b.Orders[stream] <= 0 || stage.EndedAt != "" {
				continue
			}
			_, period, matchErr := cfg.Match(b.Input)
			if matchErr != nil {
				continue
			}
			start, startErr := parseEventClock(b.Date, period.Start, loc)
			end, endErr := parseEventClock(b.Date, period.End, loc)
			if startErr != nil || endErr != nil {
				continue
			}
			rooms[key] = true
			pending[key] = append(pending[key], pendingSlot{id: b.ID, start: start, end: end, order: b.Orders[stream], ready: readyForRoom(*b, stream)})
		}
	}
	if len(pending) == 0 {
		return nil
	}
	out := map[string]SlotEstimate{}
	for key := range rooms {
		rows := pending[key]
		sort.Slice(rows, func(i, j int) bool {
			// Absent parents keep schedule-based estimates but cannot delay arrivals.
			if rows[i].ready != rows[j].ready {
				return rows[i].ready
			}
			if !rows[i].start.Equal(rows[j].start) {
				return rows[i].start.Before(rows[j].start)
			}
			if !rows[i].end.Equal(rows[j].end) {
				return rows[i].end.Before(rows[j].end)
			}
			if rows[i].order != rows[j].order {
				return rows[i].order < rows[j].order
			}
			return rows[i].id < rows[j].id
		})
		base := cursor[key]
		for _, row := range rows {
			floor := row.start
			if row.ready {
				floor = now
			}
			if now.After(floor) {
				floor = now
			}
			at := floor
			if base.After(at) {
				at = base
			}
			out[row.id] = SlotEstimate{StartAt: at.UTC().Format(time.RFC3339Nano), Late: at.After(row.end)}
			base = at.Add(minuteStep)
		}
	}
	return out
}

// projectedSessionEnd estimates when the room's current session frees up.
func projectedSessionEnd(status string, stage Stage, now time.Time, step time.Duration) (time.Time, bool) {
	switch status {
	case "Serving":
		started, err := time.Parse(time.RFC3339Nano, stage.StartedAt)
		if err != nil {
			return time.Time{}, false
		}
		end := started.Add(step)
		if now.After(end) {
			return now, true
		}
		return end, true
	case "Calling":
		called, err := time.Parse(time.RFC3339Nano, stage.CalledAt)
		if err != nil {
			return time.Time{}, false
		}
		at := called
		if now.After(at) {
			at = now
		}
		return at.Add(step), true
	}
	return time.Time{}, false
}

// parseEventClock builds an instant from an event date/clock pair in the event
// time zone, so estimates do not depend on the server's local zone.
func parseEventClock(date, clock string, loc *time.Location) (time.Time, error) {
	return time.ParseInLocation("2006-01-02 15:04", date+" "+clock, loc)
}
