package consult

// Preview the first two checked-in, ordered parents across booking periods.
// A Combined second stage joins only after Staff has assigned its room order.
func (s *Service) upcoming(bookings []Booking, level, date, stream string) []LiveQueue {
	waiting := s.remainingRoomQueue(bookings, level, date, stream)
	out := []LiveQueue{}
	for _, c := range waiting {
		if len(out) == 2 {
			break
		}
		out = append(out, LiveQueue{QueueNo: *c.booking.QueueNo, State: "Waiting", PeriodStart: c.period.Start, PeriodEnd: c.period.End})
	}
	return out
}
