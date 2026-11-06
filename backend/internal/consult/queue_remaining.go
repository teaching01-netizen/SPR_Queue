package consult

import "sort"

type roomCandidate struct {
	booking Booking
	period  Period
}

func readyForRoom(b Booking, stream string) bool {
	return b.CurrentSubject == stream && b.Status == "Waiting" && b.CheckedInAt != nil && b.QueueNo != nil && b.Orders[stream] > 0 && b.Stages[stream].EndedAt == ""
}
func compareRoomCandidate(a, b roomCandidate) bool {
	if a.period.Start != b.period.Start {
		return a.period.Start < b.period.Start
	}
	if a.period.End != b.period.End {
		return a.period.End < b.period.End
	}
	if a.booking.Orders[a.booking.CurrentSubject] != b.booking.Orders[b.booking.CurrentSubject] {
		return a.booking.Orders[a.booking.CurrentSubject] < b.booking.Orders[b.booking.CurrentSubject]
	}
	return a.booking.ID < b.booking.ID
}

// Saved order is local to a period and room. Earlier booked periods take
// priority, including arrivals after a later-period parent was served early.
func (s *Service) remainingRoomQueue(bookings []Booking, level, date, stream string) []roomCandidate {
	waiting := []roomCandidate{}
	for _, b := range bookings {
		if b.Level != level || b.Date != date || !readyForRoom(b, stream) {
			continue
		}
		_, p, err := s.Config.Match(b.Input)
		if err != nil {
			continue
		}
		waiting = append(waiting, roomCandidate{b, p})
	}
	sort.Slice(waiting, func(i, j int) bool { return compareRoomCandidate(waiting[i], waiting[j]) })
	return waiting
}
func (s *Service) queuesUntilTurn(bookings []Booking, own Booking) (*int, string) {
	if contains([]string{"Calling", "Serving"}, own.Status) {
		zero := 0
		return &zero, "current"
	}
	if contains([]string{"Completed", "Cancelled", "No-show"}, own.Status) {
		return nil, "inactive"
	}
	if own.CheckedInAt == nil {
		return nil, "awaiting-check-in"
	}
	if !readyForRoom(own, own.CurrentSubject) {
		return nil, "awaiting-order"
	}
	waiting := s.remainingRoomQueue(bookings, own.Level, own.Date, own.CurrentSubject)
	for n, b := range waiting {
		if b.booking.ID == own.ID {
			turns := n + 1
			return &turns, "ready"
		}
	}
	return nil, "awaiting-order"
}
