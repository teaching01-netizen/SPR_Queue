package consult

import (
	"context"
	"strings"
	"unicode/utf8"
)

type BookingSwap struct {
	OtherID       string `json:"otherBookingId"`
	Revision      int64  `json:"expectedRevision"`
	OtherRevision int64  `json:"otherExpectedRevision"`
	Reason        string `json:"reason"`
	Confirmation  string `json:"confirmation"`
}

func canSwapBooking(b Booking) bool {
	if !contains([]string{"Booked", "Waiting", "No-show"}, b.Status) || b.ArchivedAt != "" {
		return false
	}
	for _, stage := range b.Stages {
		if stage.StartedAt != "" || stage.EndedAt != "" {
			return false
		}
	}
	return true
}

// SwapBookings exchanges two seats in one transaction, including full periods.
func (s *Service) SwapBookings(ctx context.Context, id string, in BookingSwap, actor string) ([]Booking, error) {
	if id == in.OtherID || in.OtherID == "" {
		return nil, problem("VALIDATION_ERROR", "เลือกผู้ลงทะเบียนอีกคนที่ต้องการสลับ", 400)
	}
	if in.Confirmation != "SWAP BOOKINGS "+id+" "+in.OtherID {
		return nil, problem("CONFIRMATION_REQUIRED", "กรุณายืนยันผู้ลงทะเบียนทั้งสองราย", 400)
	}
	in.Reason = strings.TrimSpace(in.Reason)
	if in.Reason == "" || utf8.RuneCountInString(in.Reason) > 500 {
		return nil, problem("VALIDATION_ERROR", "กรุณาระบุเหตุผลไม่เกิน 500 ตัวอักษร", 400)
	}
	tx, err := s.beginWrite(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	a, err := getBooking(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	b, err := getBooking(ctx, tx, in.OtherID)
	if err != nil {
		return nil, err
	}
	if a.EventID != s.Config.Event.ID || b.EventID != s.Config.Event.ID {
		return nil, errNotFound
	}
	if a.Revision != in.Revision || b.Revision != in.OtherRevision {
		return nil, problem("REVISION_CONFLICT", "ข้อมูลของหนึ่งในสองรายเปลี่ยนแล้ว กรุณาเปิดรายการล่าสุดแล้วเลือกใหม่", 409)
	}
	if !canSwapBooking(a) || !canSwapBooking(b) {
		return nil, problem("SWAP_NOT_ALLOWED", "สลับได้เฉพาะรายการก่อนเริ่มปรึกษา หากกำลังเรียกให้พักคิวก่อน", 409)
	}
	if a.Level != b.Level || a.Subject != b.Subject {
		return nil, problem("SWAP_SCOPE_MISMATCH", "เลือกผู้ลงทะเบียนวิชาและระดับเดียวกัน", 400)
	}
	if a.Date == b.Date && a.PeriodID == b.PeriodID {
		return nil, problem("SWAP_SAME_PERIOD", "ทั้งสองรายอยู่ช่วงเวลาเดียวกัน ให้จัดลำดับเรียกแทน", 400)
	}
	for _, x := range []Booking{a, b} {
		if err = checkNotificationIdle(ctx, tx, x.ID); err != nil {
			return nil, err
		}
	}
	previous := []Booking{a, b}
	next := []Booking{a, b}
	capacities := []int{0, 0}
	for n := range next {
		other := previous[1-n]
		x := &next[n]
		x.Date, x.PeriodID = other.Date, other.PeriodID
		normalized, rule, period, e := s.Config.Normalize(x.Input)
		if e != nil {
			return nil, e
		}
		x.Input, x.Mode = normalized, rule.Mode
		capacities[n] = period.Capacity
		x.Orders = map[string]int{}
		x.Stages = map[string]Stage{}
		x.Counter = ""
		if previous[n].Date != x.Date {
			x.CheckedInAt = nil
			x.Status = "Booked"
		}
		x.Staff = actor
	}
	// Release exactly these seats inside the uncommitted transaction to satisfy
	// the immediate capacity trigger. Readers see the previous committed state;
	// other application writers and mail workers wait on the shared meta lock.
	if _, err = tx.ExecContext(ctx, `UPDATE bookings SET status='Cancelled',data=jsonb_set(data,'{status}','"Cancelled"'::jsonb) WHERE id IN ($1,$2)`, a.ID, b.ID); err != nil {
		return nil, err
	}
	for n := range next {
		x := &next[n]
		if err = availableForBooking(ctx, tx, *x, capacities[n]); err != nil {
			return nil, err
		}
		if err = s.queueUpdatedConfirmation(ctx, tx, x); err != nil {
			return nil, err
		}
		if err = updateBooking(ctx, tx, x); err != nil {
			return nil, err
		}
		before, e := encode(previous[n])
		if e != nil {
			return nil, e
		}
		after, e := encode(map[string]any{"booking": x, "otherBookingId": next[1-n].ID, "reason": in.Reason})
		if e != nil {
			return nil, e
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO audit(booking_id,action,actor,previous,next,at) VALUES($1,'SWAP_BOOKING_PERIOD',$2,$3,$4,$5)`, x.ID, actor, before, after, stamp()); err != nil {
			return nil, err
		}
	}
	if err = bump(ctx, tx); err != nil {
		return nil, err
	}
	return next, tx.Commit()
}
