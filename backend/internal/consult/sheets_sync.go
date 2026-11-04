package consult

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"time"
)

type SheetSnapshot struct {
	Action    string     `json:"action"`
	EventID   string     `json:"eventId"`
	Revision  int64      `json:"revision"`
	Headers   []string   `json:"headers"`
	Rows      [][]string `json:"rows"`
	RequestID string     `json:"requestId"`
}

func (s *Service) sheetSnapshot(ctx context.Context) (SheetSnapshot, error) {
	v, err := s.StaffList(ctx)
	if err != nil {
		return SheetSnapshot{}, err
	}
	sheets, err := registrationSheets(s.Config, v.Bookings)
	if err != nil {
		return SheetSnapshot{}, err
	}
	created := map[string]string{}
	for _, b := range v.Bookings {
		created[b.ID] = b.CreatedAt
	}
	rows := sheets[0].rows[1:]
	for _, row := range rows {
		for n, value := range row {
			row[n] = cleanExportText(value)
		}
	}
	// Existing rows stay in place remotely. New records are appended oldest first.
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i][0], rows[j][0]
		if created[a] != created[b] {
			return created[a] < created[b]
		}
		return a < b
	})
	if rows == nil {
		rows = [][]string{}
	}
	p := SheetSnapshot{Action: "sync-registrations", EventID: s.Config.Event.ID, Revision: v.Revision, Headers: sheets[0].rows[0], Rows: rows}
	raw, err := json.Marshal(p)
	if err != nil {
		return p, err
	}
	p.RequestID = digest(string(raw))
	return p, nil
}

type SheetSink interface {
	Sync(context.Context, SheetSnapshot) error
}

type AppsScriptSheetSink struct {
	URL, Secret string
	client      *http.Client
	endpoint    string // HTTP fixtures only; production always uses the validated URL.
	now         func() time.Time
}

func (m AppsScriptSheetSink) Sync(ctx context.Context, p SheetSnapshot) error {
	if err := ValidateAppsScriptSettings(m.URL, m.Secret); err != nil {
		return fmt.Errorf("invalid Google Sheets sync settings")
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	if len(raw) > 2000000 {
		return fmt.Errorf("Google Sheets snapshot exceeds supported size")
	}
	now := time.Now()
	if m.now != nil {
		now = m.now()
	}
	e := appsScriptEnvelope{Timestamp: now.Unix(), Payload: base64.StdEncoding.EncodeToString(raw)}
	mac := hmac.New(sha256.New, []byte(m.Secret))
	mac.Write([]byte(strconv.FormatInt(e.Timestamp, 10) + "\n" + e.Payload))
	e.Signature = hex.EncodeToString(mac.Sum(nil))
	body, err := json.Marshal(e)
	if err != nil {
		return err
	}
	endpoint := m.URL
	if m.endpoint != "" {
		endpoint = m.endpoint
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("cannot create Sheets sync request")
	}
	req.Header.Set("Content-Type", "application/json")
	client := http.Client{Timeout: 45 * time.Second}
	if m.client != nil {
		client = *m.client
	}
	client.CheckRedirect = appsScriptRedirect
	r, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Google Sheets sync request failed")
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		return fmt.Errorf("Google Sheets sync rejected (status %d)", r.StatusCode)
	}
	var result struct {
		OK        bool   `json:"ok"`
		RequestID string `json:"requestId"`
		Revision  *int64 `json:"revision"`
		Status    string `json:"status"`
	}
	d := json.NewDecoder(io.LimitReader(r.Body, 4096))
	if d.Decode(&result) != nil || !result.OK || result.RequestID != p.RequestID || result.Revision == nil || *result.Revision < p.Revision || (result.Status != "synced" && result.Status != "stale") {
		return fmt.Errorf("Google Sheets did not confirm synchronization")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return fmt.Errorf("invalid Google Sheets sync response")
	}
	return nil
}

type sheetSyncProgress struct {
	revision  int64
	sent      bool
	successAt time.Time
}

func (p *sheetSyncProgress) once(ctx context.Context, s *Service, sink SheetSink, now time.Time) (bool, error) {
	v, err := s.sheetSnapshot(ctx)
	if err != nil {
		return false, err
	}
	if p.sent && v.Revision == p.revision && now.Sub(p.successAt) < 5*time.Minute {
		return false, nil
	}
	if err = sink.Sync(ctx, v); err != nil {
		return false, err
	}
	p.revision = v.Revision
	p.sent = true
	p.successAt = now
	return true, nil
}

// PostgreSQL remains the durable source. Each retry reads the latest full
// snapshot, so downtime/restarts cannot lose intermediate registrations.
// Remote booking IDs make retries idempotent, and revisions reject stale writes.
func RunSheetSync(ctx context.Context, s *Service, sink SheetSink, logger *slog.Logger) {
	progress := sheetSyncProgress{}
	delay := 15 * time.Second
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			sent, err := progress.once(ctx, s, sink, time.Now())
			if ctx.Err() != nil {
				return
			}
			if err != nil {
				logger.Error("google_sheets_sync_failed", "category", "SHEETS_SYNC_FAILED")
				delay *= 2
				if delay > 5*time.Minute {
					delay = 5 * time.Minute
				}
			} else {
				delay = 15 * time.Second
				if sent {
					logger.Info("google_sheets_sync_complete", "revision", progress.revision)
				}
			}
			timer.Reset(delay)
		}
	}
}
