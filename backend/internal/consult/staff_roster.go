package consult

import (
	"context"
	"database/sql"
	"golang.org/x/crypto/bcrypt"
	"strings"
)

// User-specified fixed credentials are confined to the local test environment.
// Production continues to use the private credential bootstrap.
var localStaffNames = [...]string{"RD1", "RD2", "RD3", "SM1", "SM2", "SM3", "SM4"}

func (a *Auth) SyncLocalStaff(ctx context.Context) error {
	if !a.Service.Config.Event.Sample || !strings.HasPrefix(a.Service.Config.Event.ID, "local-test-") {
		return problem("LOCAL_STAFF_NOT_ALLOWED", "บัญชีทดสอบชุดนี้ใช้ได้เฉพาะระบบทดลองในเครื่อง", 403)
	}
	tx, err := a.Service.beginWrite(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, username := range localStaffNames {
		var hash, role string
		err = tx.QueryRowContext(ctx, `SELECT password_hash,role FROM users WHERE username=$1`, username).Scan(&hash, &role)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if err == nil && role == "operator" && bcrypt.CompareHashAndPassword([]byte(hash), []byte(username)) == nil {
			continue
		}
		encoded, err := bcrypt.GenerateFromPassword([]byte(username), a.Cost)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO users(username,password_hash,role) VALUES($1,$2,'operator') ON CONFLICT(username) DO UPDATE SET password_hash=excluded.password_hash,role='operator'`, username, string(encoded)); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM sessions WHERE username=$1`, username); err != nil {
			return err
		}
	}
	args := make([]any, len(localStaffNames))
	for n, name := range localStaffNames {
		args[n] = name
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM sessions WHERE username NOT IN ($1,$2,$3,$4,$5,$6,$7)`, args...); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM users WHERE username NOT IN ($1,$2,$3,$4,$5,$6,$7)`, args...); err != nil {
		return err
	}
	return tx.Commit()
}
