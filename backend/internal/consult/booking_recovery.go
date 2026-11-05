package consult

import (
	"context"
	"strings"
	"unicode/utf8"
)

type BookingRecovery struct {
	Action           string `json:"action"`
	Reason           string `json:"reason"`
	StartSubject     string `json:"startSubject,omitempty"`
	ReleaseQueueCode bool   `json:"releaseQueueCode"`
	Revision         int64  `json:"expectedRevision"`
	Confirmation     string `json:"confirmation"`
}

// RecoverBooking is exposed only through the authenticated Admin endpoint.
// It changes one booking, preserving identity and tracking; codes can be released.
// The pre-change state is retained in audit. Other bookings are not renumbered.
func (s *Service) RecoverBooking(ctx context.Context, id string, in BookingRecovery, actor string) (Booking, error) {
	if in.Action != "cancel" && in.Action != "reset-checkin" {
		return Booking{}, problem("VALIDATION_ERROR", "เลือกยกเลิกคืนที่ว่าง หรือคืนเป็นรอเช็กอิน", 400)
	}
	in.Reason = strings.TrimSpace(in.Reason)
	if in.Reason == "" || utf8.RuneCountInString(in.Reason) > 500 {
		return Booking{}, problem("VALIDATION_ERROR", "กรุณาระบุเหตุผลไม่เกิน 500 ตัวอักษร", 400)
	}
	if in.Confirmation != "RECOVER BOOKING "+id {
		return Booking{}, problem("CONFIRMATION_REQUIRED", "กรุณายืนยันรายการที่ต้องการจัดการ", 400)
	}
	tx, err := s.beginWrite(ctx)
	if err != nil {
		return Booking{}, err
	}
	defer tx.Rollback()
	b, err := getBooking(ctx, tx, id)
	if err != nil {
		return b, err
	}
	if b.EventID != s.Config.Event.ID {
		return Booking{}, errNotFound
	}
	if b.Revision != in.Revision {
		return b, problem("REVISION_CONFLICT", "รายการนี้เปลี่ยนแล้ว กรุณาปิดหน้าต่างแล้วเปิดรายการล่าสุด", 409)
	}
	if err = checkNotificationIdle(ctx, tx, b.ID); err != nil {
		return b, err
	}
	previous, err := encode(b)
	if err != nil {
		return b, err
	}
	action := "ADMIN_CANCEL_BOOKING"
	if in.Action == "cancel" {
		if in.StartSubject != "" {
			return b, problem("VALIDATION_ERROR", "การยกเลิกไม่ใช้วิชาเริ่มต้น", 400)
		}
		if b.Status == "Cancelled" && b.QueueNo == nil {
			return b, nil
		}
		b.Status = "Cancelled"
		b.QueueNo = nil
		b.CancelledAt = stamp()
		b.CancellationReason = in.Reason
		b.Counter = ""
		b.Orders = map[string]int{}
		// Cancel an unsent notification; an already delivered email is not resent.
		result, e := tx.ExecContext(ctx, `UPDATE outbox SET state='Cancelled',next_at=0,lease_at=0,last_error='BOOKING_CANCELLED' WHERE booking_id=$1 AND state IN ('Pending','Failed')`, b.ID)
		if e != nil {
			return b, e
		}
		if n, _ := result.RowsAffected(); n > 0 {
			b.EmailStatus = "Cancelled"
		}
	} else {
		action = "ADMIN_RESET_CHECKIN"
		stream := b.Subject
		if b.Subject == "Combined" {
			stream = in.StartSubject
			if stream != "Math" && stream != "Verbal" {
				return b, problem("VALIDATION_ERROR", "เลือกวิชาแรก Math หรือ Verbal สำหรับ Math + Verbal", 400)
			}
		} else if in.StartSubject != "" {
			return b, problem("VALIDATION_ERROR", "วิชาแรกใช้กับ Math + Verbal เท่านั้น", 400)
		}
		if b.Status == "Cancelled" {
			_, period, e := s.Config.Match(b.Input)
			if e != nil {
				return b, e
			}
			if e = availableForBooking(ctx, tx, b, period.Capacity); e != nil {
				return b, e
			}
		}
		if in.ReleaseQueueCode || b.Status == "Cancelled" {
			b.QueueNo = nil
		}
		b.Status = "Booked"
		b.CheckedInAt = nil
		b.CurrentSubject = stream
		b.Orders = map[string]int{}
		b.Stages = map[string]Stage{}
		b.Counter = ""
		b.CompletedAt = ""
		b.CancelledAt = ""
		b.CancellationReason = ""
		b.ArchivedAt = ""
		b.ArchivedBy = ""
	}
	b.Staff = actor
	if err = updateBooking(ctx, tx, &b); err != nil {
		return b, err
	}
	next, err := encode(map[string]any{"booking": b, "reason": in.Reason})
	if err != nil {
		return b, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit(booking_id,action,actor,previous,next,at) VALUES($1,$2,$3,$4,$5,$6)`, b.ID, action, actor, previous, next, stamp()); err != nil {
		return b, err
	}
	if err = bump(ctx, tx); err != nil {
		return b, err
	}
	return b, tx.Commit()
}
