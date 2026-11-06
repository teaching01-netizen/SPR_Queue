// Command importregistrations loads a Staff Excel export (converted to JSON by
// scripts/import-export/xlsx_to_rows.py) into a local PostgreSQL database.
//
// It keeps the exported booking IDs, statuses, queue positions, check-in and
// stage timestamps, and generates a fresh valid tracking token per booking so
// parent links work locally. It refuses to run against a non-local database.
//
// Local only, for testing/demo data. Never point it at production.
package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "time/tzdata"

	_ "github.com/jackc/pgx/v5/stdlib"
	"warwick/consult/internal/consult"
)

type Row struct {
	ID                 string              `json:"id"`
	RequestKey         string              `json:"requestKey"`
	Email              string              `json:"email"`
	Wcode              string              `json:"wcode"`
	Nickname           string              `json:"nickname"`
	School             string              `json:"school"`
	Level              string              `json:"level"`
	Subject            string              `json:"subject"`
	Date               string              `json:"date"`
	PeriodID           string              `json:"periodId"`
	Questions          []string            `json:"questions"`
	SeminarConfirmed   bool                `json:"seminarConfirmed"`
	Status             string              `json:"status"`
	CurrentSubject     string              `json:"currentSubject"`
	Orders             map[string]int      `json:"orders"`
	QueueNo            string              `json:"queueNo"`
	CheckedInAt        string              `json:"checkedInAt"`
	Stages             map[string]StageRow `json:"stages"`
	Mode               string              `json:"mode"`
	Counter            string              `json:"counter"`
	Staff              string              `json:"staff"`
	CreatedAt          string              `json:"createdAt"`
	UpdatedAt          string              `json:"updatedAt"`
	CompletedAt        string              `json:"completedAt"`
	CancelledAt        string              `json:"cancelledAt"`
	CancellationReason string              `json:"cancellationReason"`
	ArchivedAt         string              `json:"archivedAt"`
	ArchivedBy         string              `json:"archivedBy"`
	EmailStatus        string              `json:"emailStatus"`
}

type StageRow struct {
	CalledAt  string `json:"calledAt"`
	StartedAt string `json:"startedAt"`
	EndedAt   string `json:"endedAt"`
}

type trackingLink struct{ id, nickname, link string }

type summary struct {
	inserted  int
	byStatus  map[string]int
	bySubject map[string]int
}

func main() {
	rowsPath := flag.String("rows", "", "JSON rows produced by scripts/import-export/xlsx_to_rows.py")
	configPath := flag.String("config", "config/event.production.json", "event configuration JSON")
	outPath := flag.String("out", "/tmp/warwick-import/tracking-links.tsv", "where to write local tracking links")
	replace := flag.Bool("replace", false, "delete existing bookings of this event first")
	confirm := flag.Bool("confirm-local", false, "required: confirm the target database is a local scratch database")
	flag.Parse()
	if *rowsPath == "" {
		fail("-rows is required")
	}
	if !*confirm {
		fail("refusing to import without -confirm-local (local scratch database only)")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	rawSecret := os.Getenv("APP_SECRET")
	publicURL := os.Getenv("PUBLIC_URL")
	if databaseURL == "" || rawSecret == "" {
		fail("DATABASE_URL and APP_SECRET must be set (see backend/.env)")
	}
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		fail("invalid DATABASE_URL: %v", err)
	}
	if host := parsed.Hostname(); host != "localhost" && host != "127.0.0.1" {
		fail("refusing to import into non-local host %q", host)
	}
	key, err := base64.StdEncoding.DecodeString(rawSecret)
	if err != nil || len(key) != 32 {
		fail("APP_SECRET must be base64 that decodes to 32 bytes")
	}
	cfg, err := consult.LoadConfig(*configPath)
	if err != nil {
		fail("load config: %v", err)
	}
	if publicURL == "" {
		publicURL = "http://127.0.0.1:8080"
	}

	raw, err := os.ReadFile(*rowsPath)
	if err != nil {
		fail("read rows: %v", err)
	}
	var rows []Row
	if err = json.Unmarshal(raw, &rows); err != nil {
		fail("decode rows: %v", err)
	}
	if err = validateRows(cfg, rows); err != nil {
		fail("invalid export rows: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	service, err := consult.Open(databaseURL, cfg, key, publicURL)
	if err != nil {
		fail("open database: %v", err)
	}
	defer service.DB.Close()

	var existing int
	if err = service.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM bookings WHERE event_id=$1`, cfg.Event.ID).Scan(&existing); err != nil {
		fail("count existing bookings: %v", err)
	}
	if existing > 0 && !*replace {
		fail("database already has %d bookings for %s; pass -replace to overwrite", existing, cfg.Event.ID)
	}

	tx, err := service.DB.BeginTx(ctx, nil)
	if err != nil {
		fail("begin: %v", err)
	}
	defer tx.Rollback()
	if existing > 0 {
		for _, statement := range []string{
			`DELETE FROM outbox WHERE booking_id IN (SELECT id FROM bookings WHERE event_id=$1)`,
			`DELETE FROM audit WHERE booking_id IN (SELECT id FROM bookings WHERE event_id=$1)`,
			`DELETE FROM bookings WHERE event_id=$1`,
		} {
			if _, err = tx.ExecContext(ctx, statement, cfg.Event.ID); err != nil {
				fail("clear existing bookings: %v", err)
			}
		}
	}

	loc, err := time.LoadLocation(cfg.Event.Timezone)
	if err != nil {
		fail("event timezone: %v", err)
	}
	result := summary{byStatus: map[string]int{}, bySubject: map[string]int{}}
	links := make([]trackingLink, 0, len(rows))
	for _, row := range rows {
		token, ciphertext, tokenHash, err := newToken(key)
		if err != nil {
			fail("token: %v", err)
		}
		booking := consult.Booking{
			Input: consult.Input{
				Email:            strings.ToLower(strings.TrimSpace(row.Email)),
				Wcode:            strings.TrimSpace(row.Wcode),
				Nickname:         strings.TrimSpace(row.Nickname),
				School:           strings.TrimSpace(row.School),
				Level:            row.Level,
				Subject:          row.Subject,
				Date:             row.Date,
				PeriodID:         row.PeriodID,
				Questions:        trimmed(row.Questions),
				SeminarConfirmed: row.SeminarConfirmed,
			},
			ID:                 row.ID,
			EventID:            cfg.Event.ID,
			RequestID:          row.RequestKey,
			Orders:             row.Orders,
			CurrentSubject:     row.CurrentSubject,
			Status:             row.Status,
			Stages:             stages(loc, row.Stages),
			Mode:               row.Mode,
			Counter:            row.Counter,
			Staff:              row.Staff,
			CreatedAt:          stampAt(loc, row.CreatedAt),
			UpdatedAt:          stampAt(loc, row.UpdatedAt),
			EmailStatus:        row.EmailStatus,
			CompletedAt:        stampAt(loc, row.CompletedAt),
			CancelledAt:        stampAt(loc, row.CancelledAt),
			CancellationReason: strings.TrimSpace(row.CancellationReason),
			ArchivedAt:         stampAt(loc, row.ArchivedAt),
			ArchivedBy:         strings.TrimSpace(row.ArchivedBy),
			Revision:           1,
		}
		if booking.Orders == nil {
			booking.Orders = map[string]int{}
		}
		if booking.CurrentSubject == "" {
			booking.CurrentSubject = "Math"
		}
		if queueNo := strings.TrimSpace(row.QueueNo); queueNo != "" {
			booking.QueueNo = &queueNo
		}
		if checkedIn := stampAt(loc, row.CheckedInAt); checkedIn != "" {
			booking.CheckedInAt = &checkedIn
		}
		if booking.EmailStatus == "" {
			booking.EmailStatus = "Sent"
		}
		data, err := json.Marshal(booking)
		if err != nil {
			fail("encode booking %s: %v", row.ID, err)
		}
		payloadHash := sha256Hex(mustJSON(booking.Input))
		var queueArg any
		if booking.QueueNo != nil {
			queueArg = *booking.QueueNo
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO bookings(id,event_id,level,subject,date,period_id,current_subject,status,queue_no,request_key,payload_hash,token_hash,token_cipher,data)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
			booking.ID, booking.EventID, booking.Level, booking.Subject, booking.Date, booking.PeriodID,
			booking.CurrentSubject, booking.Status, queueArg, booking.RequestID, payloadHash, tokenHash, ciphertext, data); err != nil {
			fail("insert %s: %v", row.ID, err)
		}
		state := booking.CurrentSubject + "/" + booking.Status
		auditAt := time.Now().UTC().Format(time.RFC3339Nano)
		if _, err = tx.ExecContext(ctx, `INSERT INTO audit(booking_id,action,actor,previous,next,at) VALUES($1,'IMPORT','local-import','',$2,$3)`, booking.ID, state, auditAt); err != nil {
			fail("audit %s: %v", row.ID, err)
		}
		result.inserted++
		result.byStatus[booking.Status]++
		result.bySubject[booking.Subject]++
		links = append(links, trackingLink{booking.ID, booking.Nickname, strings.TrimRight(publicURL, "/") + "/#/track/" + url.PathEscape(token)})
	}
	if _, err = tx.ExecContext(ctx, `UPDATE meta SET revision=revision+1 WHERE id=1`); err != nil {
		fail("bump revision: %v", err)
	}
	if err = tx.Commit(); err != nil {
		fail("commit: %v", err)
	}

	if err = writeLinks(*outPath, links); err != nil {
		fail("write tracking links: %v", err)
	}
	fmt.Printf("imported %d registrations into %q\n", result.inserted, cfg.Event.ID)
	printMap("status", result.byStatus)
	printMap("subject", result.bySubject)
	fmt.Printf("tracking links: %s (contains private demo tokens, keep it local)\n", *outPath)
}

func validateRows(cfg consult.Config, rows []Row) error {
	known := map[string]bool{}
	for _, rule := range cfg.Rules {
		for _, period := range rule.Periods {
			known[rule.Level+"|"+rule.Subject+"|"+rule.Date+"|"+period.ID] = true
		}
	}
	seen := map[string]bool{}
	for index, row := range rows {
		if row.ID == "" || seen[row.ID] {
			return fmt.Errorf("row %d: missing or duplicate booking id", index+2)
		}
		seen[row.ID] = true
		if !known[row.Level+"|"+row.Subject+"|"+row.Date+"|"+row.PeriodID] {
			return fmt.Errorf("row %d (%s): unknown level/subject/date/period", index+2, row.ID)
		}
		switch row.Status {
		case "Booked", "Waiting", "Calling", "Serving", "Completed", "No-show", "Cancelled":
		default:
			return fmt.Errorf("row %d (%s): unknown status %q", index+2, row.ID, row.Status)
		}
		if row.CurrentSubject != "Math" && row.CurrentSubject != "Verbal" {
			return fmt.Errorf("row %d (%s): unknown current subject %q", index+2, row.ID, row.CurrentSubject)
		}
		if row.Nickname == "" || row.School == "" || !strings.Contains(row.Email, "@") {
			return fmt.Errorf("row %d (%s): missing nickname, school or email", index+2, row.ID)
		}
	}
	return nil
}

func newToken(key []byte) (token, ciphertext, hash string, err error) {
	raw := make([]byte, 32)
	if _, err = io.ReadFull(rand.Reader, raw); err != nil {
		return "", "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", "", "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", "", "", err
	}
	ciphertext = base64.RawURLEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte(token), nil))
	return token, ciphertext, sha256Hex([]byte(token)), nil
}

func sha256Hex(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func mustJSON(value any) []byte {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return raw
}

func trimmed(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, strings.TrimSpace(value))
	}
	return out
}

func stages(loc *time.Location, rows map[string]StageRow) map[string]consult.Stage {
	out := map[string]consult.Stage{}
	for stream, row := range rows {
		stage := consult.Stage{
			CalledAt:  stampAt(loc, row.CalledAt),
			StartedAt: stampAt(loc, row.StartedAt),
			EndedAt:   stampAt(loc, row.EndedAt),
		}
		if stage.CalledAt != "" || stage.StartedAt != "" || stage.EndedAt != "" {
			out[stream] = stage
		}
	}
	return out
}

func stampAt(loc *time.Location, value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "-" {
		return ""
	}
	at, err := time.ParseInLocation("2006-01-02 15:04:05", value, loc)
	if err != nil {
		return ""
	}
	return at.UTC().Format(time.RFC3339Nano)
}

func writeLinks(path string, links []trackingLink) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	sort.Slice(links, func(i, j int) bool { return links[i].nickname < links[j].nickname })
	var builder strings.Builder
	builder.WriteString("booking_id\tnickname\ttracking_url\n")
	for _, link := range links {
		builder.WriteString(link.id + "\t" + link.nickname + "\t" + link.link + "\n")
	}
	return os.WriteFile(path, []byte(builder.String()), 0o600)
}

func printMap(label string, values map[string]int) {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", key, values[key]))
	}
	fmt.Printf("%s: %s\n", label, strings.Join(parts, ", "))
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
