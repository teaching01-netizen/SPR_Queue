package consult

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type User struct {
	Username string `json:"username"`
	Role     string `json:"role"`
}
type Identity struct {
	User  User
	Token string
}
type Auth struct {
	Service *Service
	Secure  bool
	Cost    int
	dummy   []byte
	limits  *rateLimits
}

func NewAuth(s *Service, secure bool) (*Auth, error) {
	dummy, err := bcrypt.GenerateFromPassword([]byte("unused-constant-dummy-password"), 12)
	return &Auth{Service: s, Secure: secure, Cost: 12, dummy: dummy, limits: newLimits()}, err
}
func (a *Auth) Bootstrap(ctx context.Context, username, password string, reset bool) error {
	if !identifier.MatchString(username) {
		return fmt.Errorf("invalid STAFF_USERNAME")
	}
	tx, err := a.Service.beginWrite(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var hash string
	err = tx.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE username=$1`, username).Scan(&hash)
	if err == nil && !reset {
		if password != "" && bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
			return fmt.Errorf("existing staff password differs; use -reset-staff-password explicitly")
		}
		return nil
	}
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if len(password) < 12 || len(password) > 72 {
		return fmt.Errorf("STAFF_PASSWORD must be 12 to 72 bytes")
	}
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), a.Cost)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO users(username,password_hash,role) VALUES($1,$2,'admin') ON CONFLICT(username) DO UPDATE SET password_hash=excluded.password_hash`, username, string(bytes)); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM sessions WHERE username=$1`, username); err != nil {
		return err
	}
	return tx.Commit()
}
func (a *Auth) csrf(token string) string {
	h := hmac.New(sha256.New, a.Service.Key)
	h.Write([]byte("csrf/" + token))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}
func (a *Auth) Authenticate(r *http.Request) (Identity, error) {
	c, err := r.Cookie("warwick_session")
	if err != nil || len(c.Value) > 100 {
		return Identity{}, problem("UNAUTHORIZED", "กรุณาเข้าสู่ระบบ Staff", 401)
	}
	var user User
	err = a.Service.DB.QueryRowContext(r.Context(), `SELECT u.username,u.role FROM sessions s JOIN users u ON s.username=u.username WHERE s.token_hash=$1 AND s.expires_at>$2`, digest(c.Value), time.Now().Unix()).Scan(&user.Username, &user.Role)
	if err == sql.ErrNoRows {
		return Identity{}, problem("UNAUTHORIZED", "กรุณาเข้าสู่ระบบ Staff", 401)
	}
	if err != nil {
		return Identity{}, err
	}
	return Identity{user, c.Value}, nil
}
func (a *Auth) CheckCSRF(r *http.Request, id Identity) error {
	actual := r.Header.Get("X-CSRF-Token")
	expected := a.csrf(id.Token)
	if subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) != 1 {
		return problem("FORBIDDEN", "คำขอไม่ผ่านการตรวจสอบ กรุณาเข้าสู่ระบบใหม่", 403)
	}
	return nil
}
func (a *Auth) Login(ctx context.Context, username, password string) (Identity, error) {
	var hash string
	var user User
	err := a.Service.DB.QueryRowContext(ctx, `SELECT username,role,password_hash FROM users WHERE username=$1`, username).Scan(&user.Username, &user.Role, &hash)
	if err != nil && err != sql.ErrNoRows {
		return Identity{}, err
	}
	// Local-test credentials stay confined to sample events. Confirmed production
	// events may use explicitly provisioned username/password pairs.
	if password == username && strings.HasPrefix(a.Service.Config.Event.ID, "local-test-") && !a.Service.Config.Event.Sample {
		for _, name := range localStaffNames {
			if username == name {
				err = sql.ErrNoRows
				hash = ""
				break
			}
		}
	}
	candidate := []byte(hash)
	if err == sql.ErrNoRows {
		candidate = a.dummy
	}
	if bcrypt.CompareHashAndPassword(candidate, []byte(password)) != nil || err == sql.ErrNoRows {
		return Identity{}, problem("INVALID_CREDENTIALS", "ชื่อผู้ใช้หรือรหัสผ่านไม่ถูกต้อง", 401)
	}
	token, err := randomToken()
	if err != nil {
		return Identity{}, err
	}
	return Identity{user, token}, nil
}
func (a *Auth) StoreSession(ctx context.Context, id Identity, old string) error {
	tx, err := a.Service.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at<=$1 OR token_hash=$2`, time.Now().Unix(), digest(old)); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO sessions VALUES($1,$2,$3)`, digest(id.Token), id.User.Username, time.Now().Add(8*time.Hour).Unix()); err != nil {
		return err
	}
	return tx.Commit()
}
func (a *Auth) Cookie(w http.ResponseWriter, token string) {
	age := 8 * 60 * 60
	if token == "" {
		age = -1
	}
	http.SetCookie(w, &http.Cookie{Name: "warwick_session", Value: token, Path: "/", HttpOnly: true, Secure: a.Secure, SameSite: http.SameSiteStrictMode, MaxAge: age})
}

type rateBucket struct {
	count   int
	started time.Time
}
type rateLimits struct {
	mu      sync.Mutex
	buckets map[string]rateBucket
}

func newLimits() *rateLimits { return &rateLimits{buckets: map[string]rateBucket{}} }
func (l *rateLimits) Allow(key string, max int, window time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	b := l.buckets[key]
	if now.Sub(b.started) > window {
		b = rateBucket{started: now}
	}
	if b.count >= max {
		return false
	}
	b.count++
	l.buckets[key] = b
	if len(l.buckets) > 10000 {
		for k, v := range l.buckets {
			if now.Sub(v.started) > window {
				delete(l.buckets, k)
			}
		}
	}
	return true
}
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
