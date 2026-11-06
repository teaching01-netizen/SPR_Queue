package consult

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
)

// The consult duration is event-wide display configuration. It is stored as a
// plain settings row (the same table that holds the encryption fingerprint), so
// changing it needs no schema migration and never touches bookings, audit or
// the outbox.
const (
	consultMinutesSetting = "consult_minutes"
	defaultConsultMinutes = 10
	minConsultMinutes     = 1
	maxConsultMinutes     = 60
)

func validConsultMinutes(minutes int) bool {
	return minutes >= minConsultMinutes && minutes <= maxConsultMinutes
}

// ConsultMinutes returns the configured minutes per consult slot. A missing or
// malformed row falls back to the default so the staff dashboard keeps working.
func (s *Service) ConsultMinutes(ctx context.Context) (int, error) {
	var raw string
	err := s.DB.QueryRowContext(ctx, `SELECT value FROM settings WHERE name=$1`, consultMinutesSetting).Scan(&raw)
	if err == sql.ErrNoRows {
		return defaultConsultMinutes, nil
	}
	if err != nil {
		return 0, err
	}
	minutes, convErr := strconv.Atoi(strings.TrimSpace(raw))
	if convErr != nil || !validConsultMinutes(minutes) {
		return defaultConsultMinutes, nil
	}
	return minutes, nil
}

// SetConsultMinutes persists the admin-editable slot duration. It takes the
// write guard so it serializes with queue writes, but deliberately does not
// bump the revision: open queue drafts and queued mutations stay valid.
func (s *Service) SetConsultMinutes(ctx context.Context, minutes int) error {
	if !validConsultMinutes(minutes) {
		return problem("VALIDATION_ERROR", "เวลาต่อคิวต้องอยู่ระหว่าง 1 ถึง 60 นาที", 400)
	}
	tx, err := s.beginWrite(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT INTO settings(name,value) VALUES($1,$2) ON CONFLICT(name) DO UPDATE SET value=EXCLUDED.value`, consultMinutesSetting, strconv.Itoa(minutes)); err != nil {
		return err
	}
	return tx.Commit()
}
