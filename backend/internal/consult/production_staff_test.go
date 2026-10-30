package consult

import (
	"encoding/json"
	"golang.org/x/crypto/bcrypt"
	"strings"
	"testing"
)

func productionAccounts() []ProductionStaffAccount {
	out := []ProductionStaffAccount{}
	for n, name := range localStaffNames {
		role := "operator"
		if n == 0 {
			role = "admin"
		}
		out = append(out, ProductionStaffAccount{name, "private-production-" + name + "-pass123", role})
	}
	return out
}
func TestProductionStaffValidation(t *testing.T) {
	good := productionAccounts()
	encode := func(v any) string { b, _ := json.Marshal(v); return string(b) }
	if _, e := ParseProductionStaff(encode(good)); e != nil {
		t.Fatal(e)
	}
	for _, raw := range []string{"", `[]`, encode(good) + ` garbage`, encode(good) + ` {}`, strings.Replace(encode(good), `"RD1"`, `"unknown"`, 1), strings.Replace(encode(good), `"private-production-RD1-pass123"`, `"rd1"`, 1), strings.Replace(encode(good), `"private-production-RD1-pass123"`, `"short"`, 1), strings.ReplaceAll(encode(good), `"admin"`, `"operator"`)} {
		if _, e := ParseProductionStaff(raw); e == nil {
			t.Fatal("accepted invalid production roster")
		}
	}
	good[1].Password = good[0].Password
	if _, e := ParseProductionStaff(encode(good)); e == nil {
		t.Fatal("duplicate passwords")
	}
}
func TestProductionStaffUsernamePasswords(t *testing.T) {
	accounts := productionAccounts()
	for i := range accounts {
		accounts[i].Password = accounts[i].Username
	}
	raw, _ := json.Marshal(accounts)
	parsed, err := ParseProductionStaff(string(raw))
	if err != nil || len(parsed) != 7 {
		t.Fatalf("requested seven-account credentials rejected: %v", err)
	}
	for _, a := range parsed {
		if a.Password != a.Username {
			t.Fatal("username password changed")
		}
	}
}
func TestProductionStaffProvisionPreservesDataAndSessions(t *testing.T) {
	s := novemberService(t)
	s.Config.Event.Sample = false
	a, e := NewAuth(s, true)
	if e != nil {
		t.Fatal(e)
	}
	a.Cost = bcrypt.MinCost
	creds := productionAccounts()
	raw, _ := json.Marshal(creds)
	if e = a.ProvisionProductionStaff(testCtx, string(raw), false); e != nil {
		t.Fatal(e)
	}
	c := book(t, s, cleanupInput())
	for _, item := range creds {
		if _, e = a.Login(testCtx, item.Username, item.Password); e != nil {
			t.Fatal(item.Username, e)
		}
		if _, e = a.Login(testCtx, item.Username, item.Username); e == nil {
			t.Fatal("local password accepted")
		}
	}
	id, e := a.Login(testCtx, creds[0].Username, creds[0].Password)
	if e != nil {
		t.Fatal(e)
	}
	if e = a.StoreSession(testCtx, id, ""); e != nil {
		t.Fatal(e)
	}
	var before string
	s.DB.QueryRow(`SELECT password_hash FROM users WHERE username='RD1'`).Scan(&before)
	if e = a.ProvisionProductionStaff(testCtx, string(raw), false); e != nil {
		t.Fatal(e)
	}
	var after string
	var count int
	s.DB.QueryRow(`SELECT password_hash FROM users WHERE username='RD1'`).Scan(&after)
	s.DB.QueryRow(`SELECT count(*) FROM sessions`).Scan(&count)
	if before != after || count != 1 {
		t.Fatal("normal redeploy reset credentials/session")
	}
	creds[0].Password = "changed-production-password-123"
	changed, _ := json.Marshal(creds)
	if e = a.ProvisionProductionStaff(testCtx, string(changed), false); e == nil {
		t.Fatal("silently reset password")
	}
	if _, e = a.Login(testCtx, productionAccounts()[0].Username, productionAccounts()[0].Password); e != nil {
		t.Fatal("failed provision was not atomic")
	}
	if e = a.ProvisionProductionStaff(testCtx, string(changed), true); e != nil {
		t.Fatal(e)
	}
	s.DB.QueryRow(`SELECT count(*) FROM sessions`).Scan(&count)
	if count != 0 {
		t.Fatal("changed account kept session")
	}
	if _, e = a.Login(testCtx, creds[0].Username, creds[0].Password); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Track(testCtx, c.Token); e != nil {
		t.Fatal("provision lost booking/tracking", e)
	}
	// Apply the requested username/password pairs explicitly. Existing sessions
	// are revoked, old passwords stop working, and booking data is preserved.
	for i := range creds {
		creds[i].Password = creds[i].Username
	}
	usernamePasswords, _ := json.Marshal(creds)
	if e = a.ProvisionProductionStaff(testCtx, string(usernamePasswords), false); e == nil {
		t.Fatal("username passwords bypassed explicit provisioning")
	}
	if e = a.ProvisionProductionStaff(testCtx, string(usernamePasswords), true); e != nil {
		t.Fatal(e)
	}
	for _, item := range creds {
		if _, e = a.Login(testCtx, item.Username, item.Password); e != nil {
			t.Fatal(item.Username, e)
		}
	}
	if _, e = a.Login(testCtx, "RD1", "changed-production-password-123"); e == nil {
		t.Fatal("old password still accepted")
	}
	if e = a.ProvisionProductionStaff(testCtx, string(usernamePasswords), false); e != nil {
		t.Fatal("normal redeploy rejected username passwords", e)
	}
	if _, e = s.Track(testCtx, c.Token); e != nil {
		t.Fatal("password change lost booking/tracking", e)
	}
	s.Config.Event.Sample = true
	if e = a.ProvisionProductionStaff(testCtx, string(raw), true); e == nil {
		t.Fatal("production roster accepted for sample")
	}
}
