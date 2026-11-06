package consult

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"strings"
	"time"
)

type Service struct {
	DB                      *sql.DB
	Config                  Config
	Key                     []byte
	PublicURL               string
	LocalTestDataManagement bool
	MailEnabled             bool
	DatabaseURL             string
}

func Open(databaseURL string, cfg Config, key []byte, publicURL string) (*Service, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("APP_SECRET must decode to 32 bytes")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	parsed, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid DATABASE_URL")
	}
	parsed.ConnectTimeout = 5 * time.Second
	parsed.RuntimeParams["statement_timeout"] = "15000"
	parsed.RuntimeParams["lock_timeout"] = "10000"
	db := stdlib.OpenDB(*parsed)
	db.SetMaxOpenConns(12)
	db.SetMaxIdleConns(6)
	db.SetConnMaxLifetime(30 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err = db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("cannot connect to PostgreSQL: %w", err)
	}
	s := &Service{DB: db, Config: cfg, Key: key, PublicURL: publicURL, DatabaseURL: databaseURL}
	if err = s.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

//go:embed migrations/*.sql
var migrationFiles embed.FS

func (s *Service) migrate(ctx context.Context) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(724018551)`); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations(version TEXT PRIMARY KEY,checksum TEXT NOT NULL,applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	files, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return err
	}
	for _, f := range files {
		if !strings.HasSuffix(f.Name(), ".sql") {
			continue
		}
		bytes, e := migrationFiles.ReadFile("migrations/" + f.Name())
		if e != nil {
			return e
		}
		checksum := digest(string(bytes))
		var saved string
		e = tx.QueryRowContext(ctx, `SELECT checksum FROM schema_migrations WHERE version=$1`, f.Name()).Scan(&saved)
		if e == nil {
			if saved != checksum {
				return fmt.Errorf("migration checksum changed: %s", f.Name())
			}
			continue
		}
		if e != sql.ErrNoRows {
			return e
		}
		if _, e = tx.ExecContext(ctx, string(bytes)); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, `INSERT INTO schema_migrations(version,checksum) VALUES($1,$2)`, f.Name(), checksum); e != nil {
			return e
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	tx, err = s.beginWrite(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var stored string
	err = tx.QueryRowContext(ctx, `SELECT value FROM settings WHERE name='encryption_key_fingerprint'`).Scan(&stored)
	fingerprint := digest(string(s.Key))
	if err == nil && stored != fingerprint {
		return fmt.Errorf("APP_SECRET does not match this database; restore the original secret")
	}
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if err == sql.ErrNoRows {
		if _, err = tx.ExecContext(ctx, `INSERT INTO settings(name,value) VALUES('encryption_key_fingerprint',$1)`, fingerprint); err != nil {
			return err
		}
	}
	if err = s.seedPeriods(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) seedPeriods(ctx context.Context, tx *sql.Tx) error {
	var err error
	for _, r := range s.Config.Rules {
		for _, p := range r.Periods {
			var occupied int
			if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM bookings WHERE event_id=$1 AND level=$2 AND subject=$3 AND date=$4 AND period_id=$5 AND status<>'Cancelled'`, s.Config.Event.ID, r.Level, r.Subject, r.Date, p.ID).Scan(&occupied); err != nil {
				return err
			}
			if occupied > p.Capacity {
				return fmt.Errorf("configured capacity is below existing reservations")
			}
			_, err = tx.ExecContext(ctx, `INSERT INTO periods(event_id,level,subject,date,period_id,start,"end",capacity,mode) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(event_id,level,subject,date,period_id) DO UPDATE SET start=excluded.start,"end"=excluded."end",capacity=excluded.capacity,mode=excluded.mode`, s.Config.Event.ID, r.Level, r.Subject, r.Date, p.ID, p.Start, p.End, p.Capacity, r.Mode)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// ResetLocalTestData clears all event test rows in this dedicated database,
// preserves account/secret/migration tables, then seeds the current catalog.
// The CLI also verifies local-test mode, database name, user and local host.
func (s *Service) ResetLocalTestData(ctx context.Context) error {
	if !s.Config.Event.Sample || !strings.HasPrefix(s.Config.Event.ID, "local-test-") {
		return problem("RESET_NOT_ALLOWED", "ล้างได้เฉพาะกิจกรรมทดสอบในเครื่อง", 403)
	}
	if err := s.Config.Validate(); err != nil {
		return err
	}
	tx, err := s.beginWrite(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `TRUNCATE outbox,audit,bookings,sequences,periods,sessions RESTART IDENTITY`); err != nil {
		return err
	}
	if err = s.seedPeriods(ctx, tx); err != nil {
		return err
	}
	if err = bump(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}

// Conservative guard for queue/capacity writes across independent Go instances.
// Read-only snapshots do not acquire this lock.
func (s *Service) beginWrite(ctx context.Context) (*sql.Tx, error) {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	var revision int64
	if err = tx.QueryRowContext(ctx, `SELECT revision FROM meta WHERE id=1 FOR UPDATE`).Scan(&revision); err != nil {
		tx.Rollback()
		return nil, err
	}
	return tx, nil
}

type rowScanner interface{ Scan(...any) error }

func scanBooking(row rowScanner) (Booking, error) {
	var text string
	var b Booking
	if err := row.Scan(&text); err != nil {
		if err == sql.ErrNoRows {
			return b, errNotFound
		}
		return b, err
	}
	err := json.Unmarshal([]byte(text), &b)
	return b, err
}
func getBooking(ctx context.Context, tx *sql.Tx, id string) (Booking, error) {
	return scanBooking(tx.QueryRowContext(ctx, `SELECT data FROM bookings WHERE id=$1`, id))
}
func updateBooking(ctx context.Context, tx *sql.Tx, b *Booking) error {
	b.Revision++
	b.UpdatedAt = stamp()
	text, err := encode(b)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE bookings SET level=$1,subject=$2,date=$3,period_id=$4,current_subject=$5,status=$6,queue_no=$7,data=$8 WHERE id=$9`, b.Level, b.Subject, b.Date, b.PeriodID, b.CurrentSubject, b.Status, b.QueueNo, text, b.ID)
	return err
}
func addAudit(ctx context.Context, tx *sql.Tx, b Booking, action, previous, actor string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO audit(booking_id,action,actor,previous,next,at) VALUES($1,$2,$3,$4,$5,$6)`, b.ID, action, actor, previous, state(b), stamp())
	return err
}

// assignmentState records the position values a queue save actually changes.
// state() only captures subject/status, which made ASSIGN_QUEUE history useless
// for explaining why a parent saw a different number.
func assignmentState(b Booking) string {
	text, _ := encode(map[string]any{
		"status": b.Status, "currentSubject": b.CurrentSubject,
		"queueNo": b.QueueNo, "orders": b.Orders, "counter": b.Counter,
	})
	return text
}
func addAssignmentAudit(ctx context.Context, tx *sql.Tx, b Booking, action, previous, actor string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO audit(booking_id,action,actor,previous,next,at) VALUES($1,$2,$3,$4,$5,$6)`, b.ID, action, actor, previous, assignmentState(b), stamp())
	return err
}
func bump(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `UPDATE meta SET revision=revision+1 WHERE id=1`)
	return err
}
func (s *Service) snapshot(ctx context.Context) (StaffView, error) {
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return StaffView{}, err
	}
	defer tx.Rollback()
	view := StaffView{Bookings: []Booking{}, CanManageTestData: s.CanManageTestData()}
	if err = tx.QueryRowContext(ctx, `SELECT revision FROM meta WHERE id=1`).Scan(&view.Revision); err != nil {
		return view, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT data FROM bookings WHERE event_id=$1 ORDER BY date,period_id,id`, s.Config.Event.ID)
	if err != nil {
		return view, err
	}
	for rows.Next() {
		b, e := scanBooking(rows)
		if e != nil {
			rows.Close()
			return view, e
		}
		view.Bookings = append(view.Bookings, b)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return view, err
	}
	return view, tx.Commit()
}
func (s *Service) StaffList(ctx context.Context) (StaffView, error) { return s.snapshot(ctx) }
