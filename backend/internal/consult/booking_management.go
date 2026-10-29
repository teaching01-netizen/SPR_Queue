package consult

import (
	"context"
	"database/sql"
)

type BookingEdit struct {
	Subject          string `json:"subject"`
	Date             string `json:"date"`
	PeriodID         string `json:"periodId"`
	StartSubject     string `json:"startSubject,omitempty"`
	SeminarConfirmed bool   `json:"seminarConfirmed"`
	Revision         int64  `json:"expectedRevision"`
}

func bookingManagementState(b Booking) string {
	text, _ := encode(map[string]any{
		"level": b.Level, "subject": b.Subject, "date": b.Date, "periodId": b.PeriodID,
		"mode": b.Mode, "status": b.Status, "currentSubject": b.CurrentSubject,
		"queueNo": b.QueueNo, "stages": b.Stages, "checkedInAt": b.CheckedInAt,
		"archivedAt": b.ArchivedAt, "archivedBy": b.ArchivedBy,
		"cancelledAt": b.CancelledAt, "cancellationReason": b.CancellationReason,
	})
	return text
}
func auditBookingChange(ctx context.Context, tx *sql.Tx, b Booking, action, previous, actor string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO audit(booking_id,action,actor,previous,next,at) VALUES($1,$2,$3,$4,$5,$6)`, b.ID, action, actor, previous, bookingManagementState(b), stamp())
	return err
}
func checkNotificationIdle(ctx context.Context, tx *sql.Tx, id string) error {
	var state string
	if err := tx.QueryRowContext(ctx, `SELECT state FROM outbox WHERE booking_id=$1`, id).Scan(&state); err != nil && err != sql.ErrNoRows {
		return err
	}
	if state == "Sending" {
		return problem("NOTIFICATION_BUSY", "ระบบกำลังส่งอีเมล กรุณาลองใหม่ภายหลัง", 409)
	}
	return nil
}
func (s *Service) queueUpdatedConfirmation(ctx context.Context, tx *sql.Tx, b *Booking) error {
	if !s.MailEnabled || b.Status == "Cancelled" {
		return nil
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO outbox(booking_id,state) VALUES($1,'Pending') ON CONFLICT(booking_id) DO UPDATE SET state='Pending',attempts=0,next_at=0,lease_at=0,last_error=''`, b.ID)
	if err == nil {
		b.EmailStatus = "Pending"
	}
	return err
}
func availableForBooking(ctx context.Context, tx *sql.Tx, b Booking, capacity int) error {
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM bookings WHERE event_id=$1 AND level=$2 AND subject=$3 AND date=$4 AND period_id=$5 AND status!='Cancelled' AND id!=$6`, b.EventID, b.Level, b.Subject, b.Date, b.PeriodID, b.ID).Scan(&count); err != nil {
		return err
	}
	if count >= capacity {
		return problem("PERIOD_FULL", "รอบนี้เต็มแล้ว กรุณาเลือกวิชาหรือช่วงเวลาอื่น", 409)
	}
	return nil
}

// EditBooking keeps the booking/tracking identity and replaces a reservation
// atomically. Consultation history cannot be reassigned to a different subject.
func (s *Service) EditBooking(ctx context.Context, id string, in BookingEdit, actor string) (Booking, error) {
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
	if !contains([]string{"Booked", "Waiting", "No-show", "Cancelled"}, b.Status) {
		return b, problem("BOOKING_EDIT_LOCKED", "แก้รายการได้ก่อนเริ่มปรึกษา หากกำลังเรียกคิวให้พักคิวก่อน", 409)
	}
	for _, stage := range b.Stages {
		if stage.StartedAt != "" || stage.EndedAt != "" {
			return b, problem("BOOKING_EDIT_LOCKED", "เริ่มปรึกษาแล้ว ไม่สามารถเปลี่ยนรอบหรือวิชาของประวัติเดิมได้", 409)
		}
	}
	previous := bookingManagementState(b)
	input := b.Input
	input.Subject = in.Subject
	input.Date = in.Date
	input.PeriodID = in.PeriodID
	input.SeminarConfirmed = in.SeminarConfirmed
	normalized, rule, period, err := s.Config.Normalize(input)
	if err != nil {
		return b, err
	}
	stream := normalized.Subject
	if stream == "Combined" {
		stream = in.StartSubject
		if stream == "" {
			stream = "Math"
			if b.Subject == "Combined" {
				stream = b.CurrentSubject
			}
		}
		if stream != "Math" && stream != "Verbal" {
			return b, problem("VALIDATION_ERROR", "เลือกวิชาเริ่มต้น Math หรือ Verbal", 400)
		}
	} else if in.StartSubject != "" {
		return b, problem("VALIDATION_ERROR", "วิชาเริ่มต้นใช้กับ Math + Verbal เท่านั้น", 400)
	}
	if b.Subject == normalized.Subject && b.Date == normalized.Date && b.PeriodID == normalized.PeriodID && b.CurrentSubject == stream && b.SeminarConfirmed == normalized.SeminarConfirmed {
		return b, nil
	}
	if err = checkNotificationIdle(ctx, tx, b.ID); err != nil {
		return b, err
	}
	subjectChanged := b.Subject != normalized.Subject
	dateChanged := b.Date != normalized.Date
	b.Input = normalized
	b.Mode = rule.Mode
	b.CurrentSubject = stream
	if b.Status != "Cancelled" {
		if err = availableForBooking(ctx, tx, b, period.Capacity); err != nil {
			return b, err
		}
	}
	b.Orders = map[string]int{}
	b.Stages = map[string]Stage{}
	b.Counter = ""
	if subjectChanged {
		b.QueueNo = nil
	}
	if dateChanged {
		b.CheckedInAt = nil
		if b.Status != "Cancelled" {
			b.Status = "Booked"
		}
	}
	b.Staff = actor
	if err = s.queueUpdatedConfirmation(ctx, tx, &b); err != nil {
		return b, err
	}
	if err = updateBooking(ctx, tx, &b); err != nil {
		return b, err
	}
	if err = auditBookingChange(ctx, tx, b, "EDIT_BOOKING", previous, actor); err != nil {
		return b, err
	}
	if err = bump(ctx, tx); err != nil {
		return b, err
	}
	return b, tx.Commit()
}

// RestoreBooking reserves capacity again; it never revives an active room call
// or reopens completed stages. A cancelled booking can be edited first if its
// original period was retired or is now full.
func (s *Service) RestoreBooking(ctx context.Context, id string, revision int64, actor string) (Booking, error) {
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
	if b.Revision != revision {
		return b, problem("REVISION_CONFLICT", "รายการนี้เปลี่ยนแล้ว กรุณาปิดหน้าต่างแล้วเปิดรายการล่าสุด", 409)
	}
	if b.Status != "Cancelled" {
		return b, problem("RESTORE_NOT_ALLOWED", "คืนได้เฉพาะรายการยกเลิก", 409)
	}
	if b.CompletedAt != "" || (b.Subject == "Combined" && b.Stages["Math"].EndedAt != "" && b.Stages["Verbal"].EndedAt != "") {
		return b, problem("RESTORE_NOT_ALLOWED", "รายการนี้ปรึกษาเสร็จแล้ว", 409)
	}
	_, _, period, err := s.Config.Normalize(b.Input)
	if err != nil {
		return b, err
	}
	if err = checkNotificationIdle(ctx, tx, b.ID); err != nil {
		return b, err
	}
	if err = availableForBooking(ctx, tx, b, period.Capacity); err != nil {
		return b, err
	}
	previous := bookingManagementState(b)
	b.Status = "Booked"
	if b.CheckedInAt != nil {
		b.Status = "Waiting"
	}
	b.CancelledAt = ""
	b.CancellationReason = ""
	b.Counter = ""
	b.Orders = map[string]int{}
	b.Staff = actor
	if err = s.queueUpdatedConfirmation(ctx, tx, &b); err != nil {
		return b, err
	}
	if err = updateBooking(ctx, tx, &b); err != nil {
		return b, err
	}
	if err = auditBookingChange(ctx, tx, b, "RESTORE_BOOKING", previous, actor); err != nil {
		return b, err
	}
	if err = bump(ctx, tx); err != nil {
		return b, err
	}
	return b, tx.Commit()
}
