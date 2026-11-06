package consult

import (
	"context"
	"database/sql"
	"fmt"
)

// nextAvailableQueueCode returns the lowest unused code under the serialized
// write lock. A code still attached to a cancelled row is released only when
// that exact code is about to be reused; unrelated cancelled history is left
// untouched so a normal queue save does not rewrite every old row.
func nextAvailableQueueCode(ctx context.Context, tx *sql.Tx, b Booking, actor string) (string, error) {
	level, subject := "B", "M"
	if b.Level == "High Level" {
		level = "H"
	}
	if b.Subject == "Verbal" {
		subject = "V"
	}
	if b.Subject == "Combined" {
		subject = "MV"
	}
	prefix := level + "-" + subject
	rows, err := tx.QueryContext(ctx, `SELECT queue_no,status FROM bookings WHERE event_id=$1 AND queue_no IS NOT NULL`, b.EventID)
	if err != nil {
		return "", err
	}
	used := map[string]string{}
	for rows.Next() {
		var code, status string
		if err = rows.Scan(&code, &status); err != nil {
			rows.Close()
			return "", err
		}
		used[code] = status
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return "", err
	}
	for n := 1; ; n++ {
		code := fmt.Sprintf("%s-%03d", prefix, n)
		status, taken := used[code]
		if !taken {
			return code, nil
		}
		if status == "Cancelled" {
			if err = releaseCancelledQueueCode(ctx, tx, b.EventID, code, actor); err != nil {
				return "", err
			}
			return code, nil
		}
	}
}

// releaseCancelledQueueCode removes a code from one cancelled booking so the
// caller can reuse it. Active and completed bookings are never renumbered.
func releaseCancelledQueueCode(ctx context.Context, tx *sql.Tx, event, code, actor string) error {
	b, err := scanBooking(tx.QueryRowContext(ctx, `SELECT data FROM bookings WHERE event_id=$1 AND queue_no=$2`, event, code))
	if err != nil {
		return err
	}
	if b.Status != "Cancelled" {
		return problem("STALE_QUEUE", "รหัสคิวนี้ถูกใช้งานอยู่ กรุณารีเฟรชและจัดลำดับใหม่", 409)
	}
	previous := bookingManagementState(b)
	b.QueueNo = nil
	b.Orders = map[string]int{}
	b.Counter = ""
	b.Staff = actor
	if err = updateBooking(ctx, tx, &b); err != nil {
		return err
	}
	return auditBookingChange(ctx, tx, b, "RELEASE_CANCELLED_QUEUE_CODE", previous, actor)
}
