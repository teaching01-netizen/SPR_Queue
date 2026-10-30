package consult

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"io"
	"strings"
)

type ProductionStaffAccount struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

func ParseProductionStaff(raw string) ([]ProductionStaffAccount, error) {
	if len(raw) > 32768 {
		return nil, fmt.Errorf("STAFF_ACCOUNTS_JSON is too large")
	}
	var accounts []ProductionStaffAccount
	d := json.NewDecoder(strings.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&accounts); err != nil {
		return nil, fmt.Errorf("invalid STAFF_ACCOUNTS_JSON")
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("invalid STAFF_ACCOUNTS_JSON")
	}
	if len(accounts) != len(localStaffNames) {
		return nil, fmt.Errorf("configure exactly RD1-RD3 and SM1-SM4 for production")
	}
	allowed := map[string]bool{}
	for _, name := range localStaffNames {
		allowed[name] = true
	}
	seen := map[string]bool{}
	passwords := map[string]bool{}
	admins := 0
	for _, a := range accounts {
		// The event's seven named Staff accounts may use the exact username as
		// their password, as requested. Other passwords keep the length checks.
		usernamePassword := a.Password == a.Username
		if !allowed[a.Username] || seen[a.Username] || len(a.Password) > 72 || (!usernamePassword && (len(a.Password) < 12 || strings.EqualFold(a.Username, a.Password))) || passwords[a.Password] {
			return nil, fmt.Errorf("production Staff usernames/passwords are missing, duplicated or too weak")
		}
		if a.Role != "operator" && a.Role != "admin" {
			return nil, fmt.Errorf("invalid production Staff role")
		}
		if a.Role == "admin" {
			admins++
		}
		seen[a.Username] = true
		passwords[a.Password] = true
	}
	if admins < 1 {
		return nil, fmt.Errorf("at least one production Staff admin is required")
	}
	return accounts, nil
}

// Initial provisioning is automatic. Password changes/removing old accounts require an explicit CLI flag.
// Matching credentials keep existing sessions on normal redeploys.
func (a *Auth) ProvisionProductionStaff(ctx context.Context, raw string, applyChanges bool) error {
	if a.Service.Config.Event.Sample || strings.HasPrefix(a.Service.Config.Event.ID, "local-test-") {
		return fmt.Errorf("production Staff requires a confirmed production event")
	}
	accounts, err := ParseProductionStaff(raw)
	if err != nil {
		return err
	}
	tx, err := a.Service.beginWrite(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT username FROM users`)
	if err != nil {
		return err
	}
	allowed := map[string]bool{}
	for _, item := range accounts {
		allowed[item.Username] = true
	}
	var removed []string
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		if !allowed[name] {
			removed = append(removed, name)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(removed) > 0 && !applyChanges {
		return fmt.Errorf("extra Staff accounts exist; use -provision-staff explicitly to apply the seven-account roster")
	}
	for _, item := range accounts {
		var hash, role string
		err = tx.QueryRowContext(ctx, `SELECT password_hash,role FROM users WHERE username=$1`, item.Username).Scan(&hash, &role)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		existing := err == nil
		if existing && role == item.Role && bcrypt.CompareHashAndPassword([]byte(hash), []byte(item.Password)) == nil {
			continue
		}
		if existing && !applyChanges {
			return fmt.Errorf("production Staff credentials changed; use -provision-staff explicitly")
		}
		encoded, err := bcrypt.GenerateFromPassword([]byte(item.Password), a.Cost)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO users(username,password_hash,role) VALUES($1,$2,$3) ON CONFLICT(username) DO UPDATE SET password_hash=excluded.password_hash,role=excluded.role`, item.Username, string(encoded), item.Role); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM sessions WHERE username=$1`, item.Username); err != nil {
			return err
		}
	}
	for _, name := range removed {
		if _, err = tx.ExecContext(ctx, `DELETE FROM sessions WHERE username=$1`, name); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM users WHERE username=$1`, name); err != nil {
			return err
		}
	}
	return tx.Commit()
}
