package consult

// Next queues are a preview of the saved room order, never a promise of an appointment time.
// With an active consultation, stay within that period. Otherwise preview the earliest
// period with checked-in, ordered waiting students. An unscheduled second Combined stage
// is not ready for the second room until Staff has ordered it.
func (s *Service) upcoming(bookings []Booking, level, date, stream string) []LiveQueue {
	activePeriod := ""
	for _, b := range bookings {
		if b.Level == level && b.Date == date && b.CurrentSubject == stream && contains([]string{"Calling", "Serving"}, b.Status) {
			activePeriod = b.PeriodID
			break
		}
	}
	waiting, _ := s.remainingRoomQueue(bookings, level, date, stream)
	out := []LiveQueue{}
	for _, c := range waiting {
		if activePeriod != "" && c.booking.PeriodID != activePeriod {
			continue
		}
		if len(out) == 2 {
			break
		}
		if activePeriod == "" && len(out) > 0 && c.booking.PeriodID != waiting[0].booking.PeriodID {
			break
		}
		out = append(out, LiveQueue{QueueNo: *c.booking.QueueNo, State: "Waiting", PeriodStart: c.period.Start, PeriodEnd: c.period.End})
	}
	return out
}
