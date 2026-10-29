package consult

import (
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"testing"
)

func TestPostgresCapacityConstraintAndMigrationChecksum(t *testing.T) {
	s := testService(t)
	var first Confirmation
	for n := 0; n < 4; n++ {
		first = book(t, s, testInput())
	}
	_, err := s.DB.ExecContext(testCtx, `INSERT INTO bookings(id,event_id,level,subject,date,period_id,current_subject,status,queue_no,request_key,payload_hash,token_hash,token_cipher,data)
 SELECT $1,event_id,level,subject,date,period_id,current_subject,status,NULL,$2,payload_hash,$3,token_cipher,jsonb_set(data,'{id}',to_jsonb($1::text)) FROM bookings WHERE id=$4`, "constraint-test-id", "constraint-test-key", "constraint-test-token", first.Booking.ID)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" || pgErr.ConstraintName != "booking_capacity" {
		t.Fatalf("database did not reject overbooking: %v", err)
	}
	list, err := s.StaffList(testCtx)
	if err != nil || len(list.Bookings) != 4 {
		t.Fatal("failed SQL mutation changed bookings", err)
	}
	if _, err = s.DB.ExecContext(testCtx, `UPDATE schema_migrations SET checksum='altered' WHERE version='001_initial.sql'`); err != nil {
		t.Fatal(err)
	}
	other, err := Open(s.DatabaseURL, s.Config, s.Key, s.PublicURL)
	if err == nil {
		other.DB.Close()
		t.Fatal("changed migration checksum accepted")
	}
}

func TestPostgresConcurrentStaffBootstrap(t *testing.T) {
	s := testService(t)
	a, err := NewAuth(s, false)
	if err != nil {
		t.Fatal(err)
	}
	a.Cost = 4
	result := make(chan error, 2)
	for n := 0; n < 2; n++ {
		go func() { result <- a.Bootstrap(testCtx, "parallel-staff", "fixture-password-123", false) }()
	}
	for n := 0; n < 2; n++ {
		if err = <-result; err != nil {
			t.Fatal(err)
		}
	}
	identity, err := a.Login(testCtx, "parallel-staff", "fixture-password-123")
	if err != nil {
		t.Fatal(err)
	}
	if err = a.StoreSession(testCtx, identity, ""); err != nil {
		t.Fatal(err)
	}
	if err = a.Bootstrap(testCtx, "parallel-staff", "different-password-123", false); err == nil {
		t.Fatal("startup replaced existing password")
	}
	if err = a.Bootstrap(testCtx, "parallel-staff", "fixture-password-123", false); err != nil {
		t.Fatal(err)
	}
	var sessions int
	if err = s.DB.QueryRowContext(testCtx, `SELECT COUNT(*) FROM sessions WHERE username=$1`, "parallel-staff").Scan(&sessions); err != nil || sessions != 1 {
		t.Fatal("repeat bootstrap revoked session", err)
	}
}
