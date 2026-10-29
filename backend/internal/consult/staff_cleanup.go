package consult

import (
	"context"
	"strings"
	"time"
)

// Archive is reversible and keeps the completed consultation, capacity and tracking link.
func (s *Service) ArchiveCompleted(ctx context.Context, id string, archived bool, revision int64, actor string) (Booking, error) {
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
		return b, problem("REVISION_CONFLICT", "รายการนี้เปลี่ยนแล้ว กรุณาเปิดข้อมูลล่าสุด", 409)
	}
	if b.Status != "Completed" {
		return b, problem("ARCHIVE_NOT_ALLOWED", "เก็บประวัติได้เฉพาะรายการที่จบครบแล้ว", 409)
	}
	if (b.ArchivedAt != "") == archived {
		return b, nil
	}
	previous := bookingManagementState(b)
	action := "ARCHIVE_COMPLETED"
	b.ArchivedAt = stamp()
	b.ArchivedBy = actor
	if !archived {
		b.ArchivedAt = ""
		b.ArchivedBy = ""
		action = "UNARCHIVE_COMPLETED"
	}
	b.Staff = actor
	if err = updateBooking(ctx, tx, &b); err != nil {
		return b, err
	}
	if err = auditBookingChange(ctx, tx, b, action, previous, actor); err != nil {
		return b, err
	}
	if err = bump(ctx, tx); err != nil {
		return b, err
	}
	return b, tx.Commit()
}
func (s *Service) CanManageTestData() bool {
	return s.LocalTestDataManagement && s.Config.Event.Sample && strings.HasPrefix(s.Config.Event.ID, "local-test-") && !s.MailEnabled
}
func (s *Service) DeleteTestBooking(ctx context.Context, id string, revision int64, confirmation string) error {
	if !s.CanManageTestData() {
		return problem("TEST_DELETE_NOT_ALLOWED", "ลบถาวรได้เฉพาะฐานข้อมูลเทสในเครื่องที่ปิดส่งเมล", 403)
	}
	if confirmation != "DELETE TEST BOOKING" {
		return problem("CONFIRMATION_REQUIRED", "กรุณายืนยันการลบรายการทดสอบ", 400)
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
		return problem("REVISION_CONFLICT", "รายการนี้เปลี่ยนแล้ว กรุณาปิดหน้าต่างและเปิดรายการล่าสุดก่อนลบ", 409)
	}
	var sending int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM outbox WHERE booking_id=$1 AND state='Sending' AND lease_at>$2`, id, time.Now().Unix()-300).Scan(&sending); err != nil {
		return err
	}
	if sending > 0 {
		return problem("NOTIFICATION_BUSY", "รายการนี้ยังมีงานส่งเมลค้าง กรุณาลองใหม่ภายหลัง", 409)
	}
	for _, query := range []string{`DELETE FROM outbox WHERE booking_id=$1`, `DELETE FROM audit WHERE booking_id=$1`, `DELETE FROM bookings WHERE id=$1`} {
		if _, err = tx.ExecContext(ctx, query, id); err != nil {
			return err
		}
	}
	// Keep sequences so deleting one test cannot reuse a queue code belonging to other rows.
	if err = bump(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Service) ClearCurrentTestEvent(ctx context.Context, revision int64, confirmation string) (int, error) {
	if !s.CanManageTestData() {
		return 0, problem("TEST_DELETE_NOT_ALLOWED", "ล้างได้เฉพาะฐานข้อมูลเทสในเครื่องที่ปิดส่งเมล", 403)
	}
	if confirmation != "CLEAR TEST DATA" {
		return 0, problem("CONFIRMATION_REQUIRED", "พิมพ์ CLEAR TEST DATA เพื่อยืนยัน", 400)
	}
	tx, err := s.beginWrite(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var current int64
	if err = tx.QueryRowContext(ctx, `SELECT revision FROM meta WHERE id=1`).Scan(&current); err != nil {
		return 0, err
	}
	if current != revision {
		return 0, problem("REVISION_CONFLICT", "ข้อมูลเปลี่ยนแล้ว กรุณาปิดหน้าต่างแล้วเปิดใหม่เพื่อยืนยันรายการล่าสุด", 409)
	}
	event := s.Config.Event.ID
	var sending int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM outbox o JOIN bookings b ON o.booking_id=b.id WHERE b.event_id=$1 AND o.state='Sending' AND o.lease_at>$2`, event, time.Now().Unix()-300).Scan(&sending); err != nil {
		return 0, err
	}
	if sending > 0 {
		return 0, problem("NOTIFICATION_BUSY", "ยังมีงานส่งเมลค้าง กรุณาลองใหม่ภายหลัง", 409)
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM bookings WHERE event_id=$1`, event).Scan(&count); err != nil {
		return 0, err
	}
	for _, q := range []string{`DELETE FROM outbox WHERE booking_id IN (SELECT id FROM bookings WHERE event_id=$1)`, `DELETE FROM audit WHERE booking_id IN (SELECT id FROM bookings WHERE event_id=$1)`, `DELETE FROM bookings WHERE event_id=$1`} {
		if _, err = tx.ExecContext(ctx, q, event); err != nil {
			return 0, err
		}
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM sequences WHERE left(scope,length($1))=$1`, event+"/"); err != nil {
		return 0, err
	}
	if err = bump(ctx, tx); err != nil {
		return 0, err
	}
	return count, tx.Commit()
}
