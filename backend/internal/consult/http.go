package consult

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type HTTP struct {
	Service   *Service
	Auth      *Auth
	Origins   map[string]bool
	StaticDir string
	Logger    *slog.Logger
	limits    *rateLimits
}

func NewHTTP(s *Service, a *Auth, origins []string, staticDir string) *HTTP {
	h := &HTTP{Service: s, Auth: a, Origins: map[string]bool{}, StaticDir: staticDir, Logger: slog.New(slog.NewJSONHandler(os.Stdout, nil)), limits: newLimits()}
	for _, o := range origins {
		h.Origins[strings.TrimRight(o, "/")] = true
	}
	return h
}

type recorded struct {
	http.ResponseWriter
	status int
}

func (w *recorded) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
		w.ResponseWriter.WriteHeader(status)
	}
}
func (w *recorded) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(b)
}
func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func decode(w http.ResponseWriter, r *http.Request, out any) error {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return problem("VALIDATION_ERROR", "กรุณาส่งข้อมูลแบบ JSON", 400)
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32768))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return problem("VALIDATION_ERROR", "รูปแบบข้อมูลไม่ถูกต้อง", 400)
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return problem("VALIDATION_ERROR", "รูปแบบข้อมูลไม่ถูกต้อง", 400)
	}
	return nil
}
func (h *HTTP) fail(w http.ResponseWriter, r *http.Request, err error) {
	var p *Problem
	if !errors.As(err, &p) {
		p = &Problem{"INTERNAL_ERROR", "ระบบไม่สามารถทำรายการได้ กรุณาลองใหม่", 500}
		h.Logger.Error("request_failure", "correlationId", w.Header().Get("X-Request-ID"), "category", "INTERNAL_ERROR")
	}
	respond(w, p.Status, map[string]any{"error": map[string]any{"code": p.Code, "message": p.Message, "correlationId": w.Header().Get("X-Request-ID")}})
}
func (h *HTTP) staff(w http.ResponseWriter, r *http.Request, mutate, admin bool) (Identity, bool) {
	id, err := h.Auth.Authenticate(r)
	if err == nil && admin && id.User.Role != "admin" {
		err = problem("FORBIDDEN", "รายการนี้ต้องใช้สิทธิ์ Admin", 403)
	}
	if err == nil && mutate {
		err = h.Auth.CheckCSRF(r, id)
	}
	if err != nil {
		h.fail(w, r, err)
		return id, false
	}
	return id, true
}
func (h *HTTP) Handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		if err := h.Service.DB.PingContext(r.Context()); err != nil {
			h.fail(w, r, err)
			return
		}
		respond(w, 200, map[string]string{"status": "ok"})
	})
	m.HandleFunc("GET /api/event", func(w http.ResponseWriter, r *http.Request) { respond(w, 200, h.Service.Config) })
	m.HandleFunc("GET /api/periods", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		out, err := h.Service.Periods(r.Context(), q.Get("level"), q.Get("subject"), q.Get("date"))
		if err != nil {
			h.fail(w, r, err)
			return
		}
		respond(w, 200, out)
	})
	m.HandleFunc("POST /api/bookings", func(w http.ResponseWriter, r *http.Request) {
		if !h.limits.Allow("book/"+clientIP(r), 120, time.Minute) {
			w.Header().Set("Retry-After", "60")
			h.fail(w, r, problem("RATE_LIMITED", "มีคำขอจำนวนมาก กรุณาลองใหม่ภายหลัง", 429))
			return
		}
		var input Input
		if err := decode(w, r, &input); err != nil {
			h.fail(w, r, err)
			return
		}
		out, err := h.Service.Book(r.Context(), r.Header.Get("Idempotency-Key"), input)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		respond(w, 201, out)
	})
	m.HandleFunc("GET /api/track/{token}", func(w http.ResponseWriter, r *http.Request) {
		if !h.limits.Allow("track/"+clientIP(r), 600, time.Minute) {
			h.fail(w, r, problem("RATE_LIMITED", "กรุณาลองใหม่ภายหลัง", 429))
			return
		}
		out, err := h.Service.Track(r.Context(), r.PathValue("token"))
		if err != nil {
			h.fail(w, r, err)
			return
		}
		respond(w, 200, out)
	})
	m.HandleFunc("GET /api/display", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		out, err := h.Service.Display(r.Context(), q.Get("level"), q.Get("date"))
		if err != nil {
			h.fail(w, r, err)
			return
		}
		respond(w, 200, out)
	})
	m.HandleFunc("POST /api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if !h.Auth.limits.Allow("login/"+clientIP(r), 12, 5*time.Minute) {
			w.Header().Set("Retry-After", "300")
			h.fail(w, r, problem("RATE_LIMITED", "ลองเข้าสู่ระบบหลายครั้ง กรุณารอสักครู่", 429))
			return
		}
		var in struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := decode(w, r, &in); err != nil {
			h.fail(w, r, err)
			return
		}
		if len(in.Password) > 72 || len(in.Username) > 100 {
			h.fail(w, r, problem("INVALID_CREDENTIALS", "ชื่อผู้ใช้หรือรหัสผ่านไม่ถูกต้อง", 401))
			return
		}
		id, err := h.Auth.Login(r.Context(), in.Username, in.Password)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		old := ""
		if c, e := r.Cookie("warwick_session"); e == nil {
			old = c.Value
		}
		if err = h.Auth.StoreSession(r.Context(), id, old); err != nil {
			h.fail(w, r, err)
			return
		}
		h.Auth.Cookie(w, id.Token)
		respond(w, 200, map[string]any{"user": id.User, "csrfToken": h.Auth.csrf(id.Token)})
	})
	m.HandleFunc("GET /api/auth/me", func(w http.ResponseWriter, r *http.Request) {
		id, ok := h.staff(w, r, false, false)
		if !ok {
			return
		}
		respond(w, 200, map[string]any{"user": id.User, "csrfToken": h.Auth.csrf(id.Token)})
	})
	m.HandleFunc("POST /api/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		id, ok := h.staff(w, r, true, false)
		if !ok {
			return
		}
		if _, err := h.Service.DB.ExecContext(r.Context(), `DELETE FROM sessions WHERE token_hash=$1`, digest(id.Token)); err != nil {
			h.fail(w, r, err)
			return
		}
		h.Auth.Cookie(w, "")
		respond(w, 200, map[string]bool{"success": true})
	})
	m.HandleFunc("GET /api/staff/bookings", func(w http.ResponseWriter, r *http.Request) {
		_, ok := h.staff(w, r, false, false)
		if !ok {
			return
		}
		out, err := h.Service.StaffList(r.Context())
		if err != nil {
			h.fail(w, r, err)
			return
		}
		respond(w, 200, out)
	})
	m.HandleFunc("GET /api/staff/export.xlsx", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := h.staff(w, r, false, false); !ok {
			return
		}
		out, err := h.Service.StaffExport(r.Context())
		if err != nil {
			h.fail(w, r, err)
			return
		}
		w.Header().Set("Content-Type", excelContentType)
		w.Header().Set("Content-Disposition", `attachment; filename="SPR_Queue_Registrations_`+time.Now().UTC().Format("20060102_150405Z")+`.xlsx"`)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Length", strconv.Itoa(len(out)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(out)
	})
	m.HandleFunc("GET /api/staff/bookings/{id}/tracking-link", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := h.staff(w, r, false, false); !ok {
			return
		}
		out, err := h.Service.StaffTrackingLink(r.Context(), r.PathValue("id"))
		if err != nil {
			h.fail(w, r, err)
			return
		}
		respond(w, 200, out)
	})
	m.HandleFunc("PUT /api/staff/queue-order", func(w http.ResponseWriter, r *http.Request) {
		id, ok := h.staff(w, r, true, false)
		if !ok {
			return
		}
		var in struct {
			Scope      Scope    `json:"scope"`
			BookingIDs []string `json:"bookingIds"`
			Revision   int64    `json:"expectedRevision"`
		}
		if err := decode(w, r, &in); err != nil {
			h.fail(w, r, err)
			return
		}
		if err := h.Service.Assign(r.Context(), in.Scope, in.BookingIDs, in.Revision, id.User.Username); err != nil {
			h.fail(w, r, err)
			return
		}
		respond(w, 200, map[string]bool{"success": true})
	})
	m.HandleFunc("POST /api/staff/bookings/{id}/actions", func(w http.ResponseWriter, r *http.Request) {
		id, ok := h.staff(w, r, true, false)
		if !ok {
			return
		}
		var in struct {
			Status   string `json:"status"`
			Revision int64  `json:"expectedRevision"`
			Reason   string `json:"reason"`
		}
		if err := decode(w, r, &in); err != nil {
			h.fail(w, r, err)
			return
		}
		out, err := h.Service.Status(r.Context(), r.PathValue("id"), in.Status, in.Revision, id.User.Username, in.Reason)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		respond(w, 200, out)
	})
	m.HandleFunc("POST /api/staff/bookings/{id}/start-subject", func(w http.ResponseWriter, r *http.Request) {
		identity, ok := h.staff(w, r, true, false)
		if !ok {
			return
		}
		var in struct {
			Stream   string `json:"stream"`
			Revision int64  `json:"expectedRevision"`
		}
		if err := decode(w, r, &in); err != nil {
			h.fail(w, r, err)
			return
		}
		out, err := h.Service.ChooseStartSubject(r.Context(), r.PathValue("id"), in.Stream, in.Revision, identity.User.Username)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		respond(w, 200, out)
	})
	m.HandleFunc("POST /api/staff/bookings/{id}/recovery", func(w http.ResponseWriter, r *http.Request) {
		identity, ok := h.staff(w, r, true, true)
		if !ok {
			return
		}
		var in BookingRecovery
		if err := decode(w, r, &in); err != nil {
			h.fail(w, r, err)
			return
		}
		out, err := h.Service.RecoverBooking(r.Context(), r.PathValue("id"), in, identity.User.Username)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		respond(w, 200, out)
	})
	m.HandleFunc("PATCH /api/staff/bookings/{id}/student-details", func(w http.ResponseWriter, r *http.Request) {
		identity, ok := h.staff(w, r, true, false)
		if !ok {
			return
		}
		var in StudentDetailsEdit
		if err := decode(w, r, &in); err != nil {
			h.fail(w, r, err)
			return
		}
		out, err := h.Service.EditStudentDetails(r.Context(), r.PathValue("id"), in, identity.User.Username)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		respond(w, 200, out)
	})
	m.HandleFunc("PATCH /api/staff/bookings/{id}", func(w http.ResponseWriter, r *http.Request) {
		identity, ok := h.staff(w, r, true, false)
		if !ok {
			return
		}
		var in BookingEdit
		if err := decode(w, r, &in); err != nil {
			h.fail(w, r, err)
			return
		}
		out, err := h.Service.EditBooking(r.Context(), r.PathValue("id"), in, identity.User.Username)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		respond(w, 200, out)
	})
	m.HandleFunc("POST /api/staff/bookings/{id}/restore", func(w http.ResponseWriter, r *http.Request) {
		identity, ok := h.staff(w, r, true, false)
		if !ok {
			return
		}
		var in struct {
			Revision int64 `json:"expectedRevision"`
		}
		if err := decode(w, r, &in); err != nil {
			h.fail(w, r, err)
			return
		}
		out, err := h.Service.RestoreBooking(r.Context(), r.PathValue("id"), in.Revision, identity.User.Username)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		respond(w, 200, out)
	})

	m.HandleFunc("POST /api/staff/bookings/{id}/archive", func(w http.ResponseWriter, r *http.Request) {
		identity, ok := h.staff(w, r, true, false)
		if !ok {
			return
		}
		var in struct {
			Archived bool  `json:"archived"`
			Revision int64 `json:"expectedRevision"`
		}
		if err := decode(w, r, &in); err != nil {
			h.fail(w, r, err)
			return
		}
		out, err := h.Service.ArchiveCompleted(r.Context(), r.PathValue("id"), in.Archived, in.Revision, identity.User.Username)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		respond(w, 200, out)
	})
	m.HandleFunc("DELETE /api/staff/bookings/{id}", func(w http.ResponseWriter, r *http.Request) {
		_, ok := h.staff(w, r, true, false)
		if !ok {
			return
		}
		var in struct {
			Revision     int64  `json:"expectedRevision"`
			Confirmation string `json:"confirmation"`
		}
		if err := decode(w, r, &in); err != nil {
			h.fail(w, r, err)
			return
		}
		if err := h.Service.DeleteTestBooking(r.Context(), r.PathValue("id"), in.Revision, in.Confirmation); err != nil {
			h.fail(w, r, err)
			return
		}
		respond(w, 200, map[string]bool{"success": true})
	})
	m.HandleFunc("POST /api/staff/test-data/clear", func(w http.ResponseWriter, r *http.Request) {
		_, ok := h.staff(w, r, true, false)
		if !ok {
			return
		}
		var in struct {
			Revision     int64  `json:"expectedRevision"`
			Confirmation string `json:"confirmation"`
		}
		if err := decode(w, r, &in); err != nil {
			h.fail(w, r, err)
			return
		}
		count, err := h.Service.ClearCurrentTestEvent(r.Context(), in.Revision, in.Confirmation)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		respond(w, 200, map[string]any{"success": true, "deletedCount": count})
	})
	m.HandleFunc("POST /api/staff/bookings/{id}/reschedule", func(w http.ResponseWriter, r *http.Request) {
		id, ok := h.staff(w, r, true, true)
		if !ok {
			return
		}
		var in struct {
			Date     string `json:"date"`
			PeriodID string `json:"periodId"`
			Revision int64  `json:"expectedRevision"`
		}
		if err := decode(w, r, &in); err != nil {
			h.fail(w, r, err)
			return
		}
		out, err := h.Service.Reschedule(r.Context(), r.PathValue("id"), in.Date, in.PeriodID, in.Revision, id.User.Username)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		respond(w, 200, out)
	})
	m.HandleFunc("GET /api/staff/audit", func(w http.ResponseWriter, r *http.Request) {
		_, ok := h.staff(w, r, false, true)
		if !ok {
			return
		}
		out, err := h.Service.Audit(r.Context())
		if err != nil {
			h.fail(w, r, err)
			return
		}
		respond(w, 200, out)
	})
	m.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		h.fail(w, r, problem("NOT_FOUND", "ไม่พบ API นี้", 404))
	})
	if h.StaticDir != "" {
		fs := http.FileServer(http.Dir(h.StaticDir))
		m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" && r.Method != "HEAD" {
				h.fail(w, r, problem("NOT_FOUND", "ไม่พบหน้านี้", 404))
				return
			}
			clean := filepath.Clean(r.URL.Path)
			if strings.HasPrefix(filepath.Base(clean), ".") {
				http.NotFound(w, r)
				return
			}
			fs.ServeHTTP(w, r)
		})
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &recorded{ResponseWriter: w}
		requestID, err := randomToken()
		if err != nil {
			http.Error(w, "Service unavailable", 503)
			return
		}
		w.Header().Set("X-Request-ID", requestID)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; font-src 'self' data:; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		defer func() {
			if recover() != nil {
				h.fail(rec, r, problem("INTERNAL_ERROR", "ระบบไม่สามารถทำรายการได้", 500))
			}
			h.Logger.Info("request", "method", r.Method, "route", r.Pattern, "status", rec.status, "durationMs", time.Since(start).Milliseconds(), "correlationId", requestID)
		}()
		origin := r.Header.Get("Origin")
		if origin != "" && !h.Origins[origin] {
			h.fail(rec, r, problem("FORBIDDEN", "ไม่อนุญาตคำขอจากเว็บไซต์นี้", 403))
			return
		}
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Add("Vary", "Origin")
		}
		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Idempotency-Key,X-CSRF-Token")
			w.WriteHeader(204)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			if origin == "" { // permit CLI clients without cookies; privileged APIs still need their CSRF token
				if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
					h.fail(rec, r, problem("FORBIDDEN", "ไม่อนุญาตคำขอจากเว็บไซต์นี้", 403))
					return
				}
			}
		}
		m.ServeHTTP(rec, r)
	})
}
func Origin(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return "", errors.New("PUBLIC_URL must be an HTTP(S) origin")
	}
	return u.Scheme + "://" + u.Host, nil
}
