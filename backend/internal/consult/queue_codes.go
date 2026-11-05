package consult

import (
	"context"
	"database/sql"
	"fmt"
)

// beginWrite serializes all booking mutations using the shared meta-row lock.
// Read occupancy inside that transaction so simultaneous saves cannot reuse a code.
func nextAvailableQueueCode(ctx context.Context, tx *sql.Tx, b Booking) (string, error) {
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
	rows, err := tx.QueryContext(ctx, `SELECT queue_no FROM bookings WHERE event_id=$1 AND queue_no IS NOT NULL`, b.EventID)
	if err != nil {
		return "", err
	}
	used := map[string]bool{}
	for rows.Next() {
		var code string
		if err = rows.Scan(&code); err != nil {
			rows.Close()
			return "", err
		}
		used[code] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return "", err
	}
	for n := 1; ; n++ {
		code := fmt.Sprintf("%s-%03d", prefix, n)
		if !used[code] {
			return code, nil
		}
	}
}

func releaseCancelledQueueCodes(ctx context.Context, tx *sql.Tx, event, actor string) error {
	rows, err := tx.QueryContext(ctx, `SELECT data FROM bookings WHERE event_id=$1 AND status='Cancelled' AND queue_no IS NOT NULL`, event)
	if err != nil {
		return err
	}
	bookings := []Booking{}
	for rows.Next() {
		b, e := scanBooking(rows)
		if e != nil {
			rows.Close()
			return e
		}
		bookings = append(bookings, b)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, b := range bookings {
		previous := bookingManagementState(b)
		b.QueueNo = nil
		b.Orders = map[string]int{}
		b.Counter = ""
		b.Staff = actor
		if err = updateBooking(ctx, tx, &b); err != nil {
			return err
		}
		if err = auditBookingChange(ctx, tx, b, "RELEASE_CANCELLED_QUEUE_CODE", previous, actor); err != nil {
			return err
		}
	}
	return nil
}
