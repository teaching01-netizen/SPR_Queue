package consult

import "context"

// DeleteCancelledBooking permanently deletes exactly one cancelled booking.
// The HTTP endpoint requires Admin, CSRF and a confirmation bound to this ID.
func (s *Service) DeleteCancelledBooking(ctx context.Context, id string, revision int64, confirmation string) error {
	if confirmation != "DELETE CANCELLED BOOKING "+id {
		return problem("CONFIRMATION_REQUIRED", "กรุณายืนยันรายการที่ต้องการลบถาวร", 400)
	}
	tx, err := s.beginWrite(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	b, err := getBooking(ctx, tx, id)
	if err != nil {
		return err
	}
	if b.EventID != s.Config.Event.ID {
		return errNotFound
	}
	if b.Revision != revision {
		return problem("REVISION_CONFLICT", "รายการนี้เปลี่ยนแล้ว กรุณาเปิดรายการล่าสุดก่อนลบ", 409)
	}
	if b.Status != "Cancelled" {
		return problem("CANCELLED_DELETE_NOT_ALLOWED", "ลบถาวรได้เฉพาะรายการยกเลิก", 409)
	}
	if err = checkNotificationIdle(ctx, tx, id); err != nil {
		return err
	}
	for _, q := range []string{`DELETE FROM outbox WHERE booking_id=$1`, `DELETE FROM audit WHERE booking_id=$1`, `DELETE FROM bookings WHERE id=$1`} {
		if _, err = tx.ExecContext(ctx, q, id); err != nil {
			return err
		}
	}
	if err = bump(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}
