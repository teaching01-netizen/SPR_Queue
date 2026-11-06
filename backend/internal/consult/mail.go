package consult

import (
	"bytes"
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/base64"
	"fmt"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"
)

type Mailer interface {
	Send(context.Context, Booking, string) error
}
type SMTPMailer struct {
	Host, Port, Username, Password, From string
	Security                             string
	tlsConfig                            *tls.Config
	Config                               Config
}

func (m SMTPMailer) Send(ctx context.Context, b Booking, tracking string) error {
	from, err := mail.ParseAddress(m.From)
	if err != nil || strings.ContainsAny(m.From, "\r\n") {
		return fmt.Errorf("invalid SMTP_FROM")
	}
	recipient, err := mail.ParseAddress(b.Email)
	if err != nil || recipient.Address != b.Email || strings.ContainsAny(b.Email, "\r\n") {
		return fmt.Errorf("invalid recipient")
	}
	content, err := RenderConfirmation(m.Config, b, tracking)
	if err != nil {
		return err
	}
	security := m.Security
	if security == "" {
		security = "starttls"
	}
	if security != "starttls" && security != "tls" {
		return fmt.Errorf("SMTP_SECURITY must be starttls or tls")
	}
	tlsConfig := &tls.Config{ServerName: m.Host, MinVersion: tls.VersionTLS12}
	if m.tlsConfig != nil {
		tlsConfig = m.tlsConfig.Clone()
	}
	dialer := net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(m.Host, m.Port))
	if err != nil {
		return err
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	_ = conn.SetDeadline(time.Now().Add(20 * time.Second))
	if security == "tls" {
		secure := tls.Client(conn, tlsConfig)
		if err = secure.HandshakeContext(ctx); err != nil {
			return err
		}
		conn = secure
	}
	client, err := smtp.NewClient(conn, m.Host)
	if err != nil {
		return err
	}
	defer client.Close()
	if security == "starttls" {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return fmt.Errorf("SMTP server requires STARTTLS support")
		}
		if err = client.StartTLS(tlsConfig); err != nil {
			return err
		}
	}
	if m.Username != "" {
		if err = client.Auth(smtp.PlainAuth("", m.Username, m.Password, m.Host)); err != nil {
			return err
		}
	}
	if err = client.Mail(from.Address); err != nil {
		return err
	}
	if err = client.Rcpt(b.Email); err != nil {
		return err
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	message, err := m.message(b, content)
	if err != nil {
		return err
	}
	if _, err = writer.Write([]byte(message)); err != nil {
		return err
	}
	if err = writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func (m SMTPMailer) message(b Booking, content EmailContent) ([]byte, error) {
	var body bytes.Buffer
	parts := multipart.NewWriter(&body)
	for _, p := range []struct{ kind, body string }{{"text/plain", content.Text}, {"text/html", content.HTML}} {
		h := textproto.MIMEHeader{}
		h.Set("Content-Type", p.kind+"; charset=UTF-8")
		h.Set("Content-Transfer-Encoding", "quoted-printable")
		w, err := parts.CreatePart(h)
		if err != nil {
			return nil, err
		}
		q := quotedprintable.NewWriter(w)
		if _, err = q.Write([]byte(p.body)); err != nil {
			return nil, err
		}
		if err = q.Close(); err != nil {
			return nil, err
		}
	}
	if err := parts.Close(); err != nil {
		return nil, err
	}
	subject := "Warwick Private Consult - Booking Confirmation"
	if b.ID == "smtp-test" {
		subject = "Warwick - Email Delivery Test"
	}
	random, err := randomToken()
	if err != nil {
		return nil, err
	}
	// Encode From for Thai display names; don't put caller-provided IDs into headers.
	from, err := mail.ParseAddress(m.From)
	if err != nil || strings.ContainsAny(m.From, "\r\n") {
		return nil, fmt.Errorf("invalid SMTP_FROM")
	}
	headers := "From: " + from.String() + "\r\nTo: " + b.Email + "\r\nSubject: " + subject + "\r\nDate: " + time.Now().Format(time.RFC1123Z) + "\r\nMIME-Version: 1.0\r\nContent-Type: multipart/alternative; boundary=\"" + parts.Boundary() + "\"\r\nMessage-ID: <" + base64.RawURLEncoding.EncodeToString([]byte(random)) + "." + strconv.FormatInt(time.Now().UnixNano(), 10) + "@warwick-mail>\r\n\r\n"
	return append([]byte(headers), body.Bytes()...), nil
}

// Durable outbox: claim in a DB transaction, send after commit. Delivery is at-least-once.
func (s *Service) ProcessMail(ctx context.Context, mailer Mailer) (bool, error) {
	tx, err := s.beginWrite(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	now := time.Now().Unix()
	_, err = tx.ExecContext(ctx, `UPDATE outbox SET state='Pending' WHERE state='Sending' AND lease_at<$1`, now-300)
	if err != nil {
		return false, err
	}
	var id string
	var attempts int
	err = tx.QueryRowContext(ctx, `SELECT o.booking_id,o.attempts FROM outbox o JOIN bookings b ON o.booking_id=b.id WHERE b.event_id=$1 AND o.state='Pending' AND o.next_at<=$2 ORDER BY o.next_at,o.booking_id LIMIT 1`, s.Config.Event.ID, now).Scan(&id, &attempts)
	if err == sql.ErrNoRows {
		return false, tx.Commit()
	}
	if err != nil {
		return false, err
	}
	b, err := getBooking(ctx, tx, id)
	if err != nil {
		return false, err
	}
	if b.Status == "Cancelled" {
		_, err = tx.ExecContext(ctx, `UPDATE outbox SET state='Cancelled',last_error='BOOKING_CANCELLED' WHERE booking_id=$1`, id)
		if err != nil {
			return false, err
		}
		// The booking was cancelled before this worker could send: report the
		// suppressed delivery honestly instead of claiming the email failed.
		b.EmailStatus = "Cancelled"
		if err = updateBooking(ctx, tx, &b); err != nil {
			return false, err
		}
		if err = bump(ctx, tx); err != nil {
			return false, err
		}
		return true, tx.Commit()
	}
	var encrypted string
	if err = tx.QueryRowContext(ctx, `SELECT token_cipher FROM bookings WHERE id=$1`, id).Scan(&encrypted); err != nil {
		return false, err
	}
	token, err := unseal(s.Key, encrypted)
	if err != nil {
		return false, err
	}
	attempts++
	if _, err = tx.ExecContext(ctx, `UPDATE outbox SET state='Sending',attempts=$1,lease_at=$2 WHERE booking_id=$3`, attempts, now, id); err != nil {
		return false, err
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	sendErr := mailer.Send(ctx, b, s.confirm(b, token).TrackingURL)
	finishCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	finish, err := s.beginWrite(finishCtx)
	if err != nil {
		return true, err
	}
	defer finish.Rollback()
	fresh, err := getBooking(finishCtx, finish, id)
	if err != nil {
		return true, err
	}
	stateName := "Sent"
	code := ""
	nextAt := int64(0)
	fresh.EmailStatus = "Sent"
	if sendErr != nil {
		stateName = "Pending"
		code = "EMAIL_DELIVERY_FAILED"
		fresh.EmailStatus = "Failed"
		nextAt = time.Now().Add(time.Duration(1<<uint(attempts-1)) * time.Minute).Unix()
		if attempts >= 5 {
			stateName = "Failed"
		}
	}
	if _, err = finish.ExecContext(finishCtx, `UPDATE outbox SET state=$1,last_error=$2,next_at=$3,lease_at=0 WHERE booking_id=$4`, stateName, code, nextAt, id); err != nil {
		return true, err
	}
	if err = updateBooking(finishCtx, finish, &fresh); err != nil {
		return true, err
	}
	if err = addAudit(finishCtx, finish, fresh, "EMAIL_"+fresh.EmailStatus, state(fresh), "NotificationWorker"); err != nil {
		return true, err
	}
	if err = bump(finishCtx, finish); err != nil {
		return true, err
	}
	return true, finish.Commit()
}
