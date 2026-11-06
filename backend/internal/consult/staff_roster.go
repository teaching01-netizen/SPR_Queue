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

// localStaffRole matches the documented local test accounts: RD1 is the Admin
// used for recovery tools, the remaining accounts stay operators.
func localStaffRole(username string) string {
	if username == "RD1" {
		return "admin"
	}
	return "operator"
}

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
		role := localStaffRole(username)
		var hash, currentRole string
		err = tx.QueryRowContext(ctx, `SELECT password_hash,role FROM users WHERE username=$1`, username).Scan(&hash, &currentRole)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if err == nil && currentRole == role && bcrypt.CompareHashAndPassword([]byte(hash), []byte(username)) == nil {
			continue
		}
		encoded, err := bcrypt.GenerateFromPassword([]byte(username), a.Cost)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO users(username,password_hash,role) VALUES($1,$2,$3) ON CONFLICT(username) DO UPDATE SET password_hash=excluded.password_hash,role=excluded.role`, username, string(encoded), role); err != nil {
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
