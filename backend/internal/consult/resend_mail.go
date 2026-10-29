package consult

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"strings"
	"time"
)

// HTTPS email delivery for hosts that block outbound SMTP (including Railway Hobby).
type ResendMailer struct {
	APIKey   string
	From     string
	Config   Config
	client   *http.Client
	endpoint string
}

func (m ResendMailer) Send(ctx context.Context, b Booking, tracking string) error {
	from, err := mail.ParseAddress(m.From)
	if err != nil || strings.ContainsAny(m.From, "\r\n") || from.Address == "" {
		return fmt.Errorf("invalid EMAIL_FROM")
	}
	recipient, err := mail.ParseAddress(b.Email)
	if err != nil || recipient.Address != b.Email || strings.ContainsAny(b.Email, "\r\n") {
		return fmt.Errorf("invalid email recipient")
	}
	if m.APIKey == "" {
		return fmt.Errorf("RESEND_API_KEY is required")
	}
	content, err := RenderConfirmation(m.Config, b, tracking)
	if err != nil {
		return err
	}
	subject := "Warwick Private Consult - Booking Confirmation"
	if b.ID == "smtp-test" {
		subject = "Warwick - Email Delivery Test"
	}
	payload, err := json.Marshal(map[string]any{"from": from.String(), "to": []string{b.Email}, "subject": subject, "html": content.HTML, "text": content.Text})
	if err != nil {
		return err
	}
	endpoint := m.endpoint
	if endpoint == "" {
		endpoint = "https://api.resend.com/emails"
	}
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("cannot create email request")
	}
	req.Header.Set("Authorization", "Bearer "+m.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "warwick-"+digest(b.EventID+"/"+b.ID+"/"+string(payload)))
	client := m.client
	if client == nil {
		client = &http.Client{Timeout: 25 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	response, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("email provider request failed")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("email provider rejected request (status %d)", response.StatusCode)
	}
	var result struct {
		ID string `json:"id"`
	}
	if err = json.NewDecoder(io.LimitReader(response.Body, 65536)).Decode(&result); err != nil || result.ID == "" {
		return fmt.Errorf("invalid email provider response")
	}
	return nil
}
