package consult

import (
	"context"
	"net/mail"
	"strings"
	"unicode/utf8"
)

func normalizeStudentDetails(i Input) (Input, error) {
	i.Email = strings.ToLower(strings.TrimSpace(i.Email))
	i.Nickname = strings.TrimSpace(i.Nickname)
	i.School = strings.TrimSpace(i.School)
	i.Wcode = strings.TrimSpace(i.Wcode)
	a, err := mail.ParseAddress(i.Email)
	if err != nil || a.Address != i.Email || a.Name != "" || strings.ContainsAny(i.Email, "\r\n") || len(i.Email) > 254 || i.Nickname == "" || i.School == "" || utf8.RuneCountInString(i.Nickname) > 80 || utf8.RuneCountInString(i.School) > 160 || len(i.Wcode) > 40 {
		return i, problem("VALIDATION_ERROR", "กรุณากรอกข้อมูลผู้ปกครองและนักเรียนให้ถูกต้อง", 400)
	}
	return i, nil
}

type StudentDetailsEdit struct {
	Wcode    string `json:"wcode"`
	Nickname string `json:"nickname"`
	School   string `json:"school"`
	Email    string `json:"email"`
	Revision int64  `json:"expectedRevision"`
}

func studentDetailsState(b Booking) string {
	text, _ := encode(map[string]string{"wcode": b.Wcode, "nickname": b.Nickname, "school": b.School, "email": b.Email})
	return text
}

// Correct student details at any lifecycle stage without reopening a booking,
// reassigning a queue, requiring an available period, or re-sending a sent email.
func (s *Service) EditStudentDetails(ctx context.Context, id string, in StudentDetailsEdit, actor string) (Booking, error) {
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
	i := b.Input
	i.Wcode = in.Wcode
	i.Nickname = in.Nickname
	i.School = in.School
	i.Email = in.Email
	i, err = normalizeStudentDetails(i)
	if err != nil {
		return b, err
	}
	if b.Wcode == i.Wcode && b.Nickname == i.Nickname && b.School == i.School && b.Email == i.Email {
		return b, nil
	}
	if err = checkNotificationIdle(ctx, tx, id); err != nil {
		return b, err
	}
	previous := studentDetailsState(b)
	b.Input = i
	if err = updateBooking(ctx, tx, &b); err != nil {
		return b, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit(booking_id,action,actor,previous,next,at) VALUES($1,$2,$3,$4,$5,$6)`, b.ID, "EDIT_STUDENT_DETAILS", actor, previous, studentDetailsState(b), stamp()); err != nil {
		return b, err
	}
	if err = bump(ctx, tx); err != nil {
		return b, err
	}
	return b, tx.Commit()
}
