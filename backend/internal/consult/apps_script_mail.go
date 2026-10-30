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
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var appsScriptPath = regexp.MustCompile(`^/macros/s/[A-Za-z0-9_-]{10,}/exec$`)
var appsScriptSecret = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)

func ValidateAppsScriptSettings(endpoint, secret string) error {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Host != "script.google.com" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" || !appsScriptPath.MatchString(u.Path) {
		return fmt.Errorf("APPS_SCRIPT_URL must be the HTTPS script.google.com /macros/s/.../exec deployment URL")
	}
	if !appsScriptSecret.MatchString(secret) {
		return fmt.Errorf("APPS_SCRIPT_SECRET must be 64 hexadecimal characters")
	}
	return nil
}

type AppsScriptMailer struct {
	URL, Secret string
	Config      Config
	client      *http.Client
	endpoint    string // Isolated HTTP fixtures only; production uses URL.
	now         func() time.Time
}

type appsScriptMessage struct {
	RequestID string `json:"requestId"`
	To        string `json:"to"`
	Subject   string `json:"subject"`
	HTML      string `json:"html"`
	Text      string `json:"text"`
}

type appsScriptEnvelope struct {
	Timestamp int64  `json:"timestamp"`
	Payload   string `json:"payload"`
	Signature string `json:"signature"`
}

// ContentService redirects only the response to Google's one-time GET URL.
// Never replay the signed POST body to a redirect or follow a Google login page.
func appsScriptRedirect(req *http.Request, via []*http.Request) error {
	u := req.URL
	if len(via) > 2 || req.Method != http.MethodGet || u.Scheme != "https" || u.Host != "script.googleusercontent.com" || u.User != nil || u.Path != "/macros/echo" {
		return fmt.Errorf("unexpected Apps Script redirect")
	}
	return nil
}

func (m AppsScriptMailer) Send(ctx context.Context, b Booking, tracking string) error {
	if err := ValidateAppsScriptSettings(m.URL, m.Secret); err != nil {
		return err
	}
	recipient, err := mail.ParseAddress(b.Email)
	if err != nil || recipient.Address != b.Email || strings.ContainsAny(b.Email, "\r\n") {
		return fmt.Errorf("invalid email recipient")
	}
	content, err := RenderConfirmation(m.Config, b, tracking)
	if err != nil {
		return err
	}
	message := appsScriptMessage{To: b.Email, Subject: "Warwick Private Consult - Booking Confirmation", HTML: content.HTML, Text: content.Text}
	if b.ID == "smtp-test" {
		message.Subject = "Warwick - Email Delivery Test"
	}
	unsigned, err := json.Marshal(message)
	if err != nil {
		return err
	}
	message.RequestID = digest(b.EventID + "/" + b.ID + "/" + string(unsigned))
	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}
	if len(payload) > 120000 {
		return fmt.Errorf("Apps Script email payload is too large")
	}
	now := time.Now()
	if m.now != nil {
		now = m.now()
	}
	envelope := appsScriptEnvelope{Timestamp: now.Unix(), Payload: base64.StdEncoding.EncodeToString(payload)}
	mac := hmac.New(sha256.New, []byte(m.Secret))
	mac.Write([]byte(strconv.FormatInt(envelope.Timestamp, 10) + "\n" + envelope.Payload))
	envelope.Signature = hex.EncodeToString(mac.Sum(nil))
	body, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	endpoint := m.URL
	if m.endpoint != "" {
		endpoint = m.endpoint
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("cannot create Apps Script request")
	}
	req.Header.Set("Content-Type", "application/json")
	client := http.Client{Timeout: 45 * time.Second}
	if m.client != nil {
		client = *m.client
	}
	client.CheckRedirect = appsScriptRedirect
	response, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Apps Script email request failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("Apps Script email rejected (status %d)", response.StatusCode)
	}
	var result struct {
		OK        bool   `json:"ok"`
		RequestID string `json:"requestId"`
		Status    string `json:"status"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 4096))
	if decoder.Decode(&result) != nil || !result.OK || result.RequestID != message.RequestID || (result.Status != "sent" && result.Status != "duplicate") {
		return fmt.Errorf("Apps Script did not confirm email acceptance")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return fmt.Errorf("invalid Apps Script email response")
	}
	return nil
}
