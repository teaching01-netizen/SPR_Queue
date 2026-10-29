package consult

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"io"
	"math/big"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"os"
	"strings"
	"testing"
	"time"
)

func readEmail(t *testing.T, raw string) map[string]string {
	t.Helper()
	m, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	kind, params, err := mime.ParseMediaType(m.Header.Get("Content-Type"))
	if err != nil || kind != "multipart/alternative" {
		t.Fatal("missing alternative MIME", err)
	}
	out := map[string]string{}
	r := multipart.NewReader(m.Body, params["boundary"])
	for {
		p, err := r.NextRawPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		kind, _, err = mime.ParseMediaType(p.Header.Get("Content-Type"))
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(quotedprintable.NewReader(p))
		if err != nil {
			t.Fatal(err)
		}
		out[kind] = string(b)
	}
	return out
}
func TestConfirmationTemplateAndMIME(t *testing.T) {
	cfg, err := LoadConfig("../../config/event.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range cfg.Rules {
		b := Booking{Input: Input{Email: "parent@example.com", Nickname: `<img src=x onerror="alert(1)">`, Level: r.Level, Subject: r.Subject, Date: r.Date, PeriodID: r.Periods[0].ID}, Mode: r.Mode}
		link := "https://consult.example.com/#/track/private-booking-token-0123456789012345"
		c, err := RenderConfirmation(cfg, b, link)
		if err != nil {
			t.Fatal(err)
		}
		for _, body := range []string{c.Text, c.HTML} {
			if !strings.Contains(body, "ช่วงเวลา Consult:") || !strings.Contains(body, r.Periods[0].Start+" - "+r.Periods[0].End) || !strings.Contains(body, link) {
				t.Fatal("missing period or tracking")
			}
		}
		if strings.Contains(c.HTML, `<img src=x`) {
			t.Fatal("unescaped nickname")
		}
		if strings.Contains(c.HTML, "สัมมนารวม") != (r.Level == "Beginner") {
			t.Fatal("incorrect seminar scope")
		}
		if r.Subject == "Combined" && r.Level == "Beginner" && (!strings.Contains(c.HTML, "08:30 - 09:20") || !strings.Contains(c.HTML, "09:30 - 10:20")) {
			t.Fatal("missing either seminar")
		}
		if !strings.Contains(c.HTML, ">Check Queue Status</a>") {
			t.Fatal("CTA missing")
		}
		raw, err := (SMTPMailer{From: "Warwick Institute <sender@example.com>"}).message(b, c)
		if err != nil {
			t.Fatal(err)
		}
		parts := readEmail(t, string(raw))
		if strings.ReplaceAll(parts["text/html"], "\r\n", "\n") != c.HTML || strings.ReplaceAll(parts["text/plain"], "\r\n", "\n") != c.Text {
			t.Fatal("MIME not reversible")
		}
	}
	if _, err = RenderConfirmation(cfg, Booking{Input: Input{Level: cfg.Rules[0].Level, Subject: cfg.Rules[0].Subject, Date: cfg.Rules[0].Date, PeriodID: cfg.Rules[0].Periods[0].ID}}, "javascript:alert(1)"); err == nil {
		t.Fatal("unsafe link accepted")
	}
	if dest := os.Getenv("EMAIL_PREVIEW_PATH"); dest != "" {
		r := cfg.Rules[2]
		b := Booking{Input: Input{Nickname: "นักเรียนตัวอย่าง", Level: r.Level, Subject: r.Subject, Date: r.Date, PeriodID: r.Periods[0].ID}, Mode: r.Mode}
		c, err := RenderConfirmation(cfg, b, "https://example.com/#/track/preview-only-not-a-real-booking")
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(dest, []byte(c.HTML), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

// Real SMTP client delivery into an isolated TLS receiver, with certificates checked.
func smtpFixture(t *testing.T, implicit bool) (string, string, *tls.Config, <-chan string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"}, DNSNames: []string{"localhost"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	pk, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	pair, err := tls.X509KeyPair(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pk}))
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	serverConfig := &tls.Config{Certificates: []tls.Certificate{pair}, MinVersion: tls.VersionTLS12}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	messages := make(chan string, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(15 * time.Second))
		secure := implicit
		if implicit {
			conn = tls.Server(conn, serverConfig)
		}
		reader := bufio.NewReader(conn)
		write := func(s string) { io.WriteString(conn, s+"\r\n") }
		write("220 localhost SMTP")
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(line, "EHLO"):
				if secure {
					write("250-localhost\r\n250 AUTH PLAIN")
				} else {
					write("250-localhost\r\n250 STARTTLS")
				}
			case line == "STARTTLS":
				write("220 Ready")
				conn = tls.Server(conn, serverConfig)
				reader = bufio.NewReader(conn)
				secure = true
			case strings.HasPrefix(line, "AUTH PLAIN "):
				b, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(line, "AUTH PLAIN "))
				if err != nil || string(b) != "\x00smtp-user\x00smtp-password" || !secure {
					write("535 Authentication failed")
				} else {
					write("235 Authenticated")
				}
			case strings.HasPrefix(line, "MAIL FROM:"), strings.HasPrefix(line, "RCPT TO:"):
				write("250 OK")
			case line == "DATA":
				write("354 Send")
				var message strings.Builder
				for {
					l, err := reader.ReadString('\n')
					if err != nil {
						return
					}
					if l == ".\r\n" {
						break
					}
					message.WriteString(strings.TrimPrefix(l, "."))
				}
				messages <- message.String()
				write("250 Accepted")
			case line == "QUIT":
				write("221 Bye")
				return
			default:
				write("500 Unknown")
			}
		}
	}()
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	return "localhost", port, &tls.Config{ServerName: "localhost", RootCAs: pool, MinVersion: tls.VersionTLS12}, messages
}
func TestSMTPConfirmationDelivery(t *testing.T) {
	for _, security := range []string{"starttls", "tls"} {
		t.Run(security, func(t *testing.T) {
			s := novemberService(t)
			s.MailEnabled = true
			r := s.Config.Rules[2]
			b, err := s.Book(context.Background(), "smtp-"+security, Input{Email: "parent@example.com", Nickname: "SMTP test", School: "School", Level: r.Level, Subject: r.Subject, Date: r.Date, PeriodID: r.Periods[0].ID, Questions: []string{"one", "two"}, SeminarConfirmed: true})
			if err != nil {
				t.Fatal(err)
			}
			host, port, trust, messages := smtpFixture(t, security == "tls")
			mailer := SMTPMailer{Host: host, Port: port, Username: "smtp-user", Password: "smtp-password", From: "Warwick Institute <sender@example.com>", Security: security, Config: s.Config, tlsConfig: trust}
			done, err := s.ProcessMail(context.Background(), mailer)
			if err != nil || !done {
				t.Fatal("delivery failed", err)
			}
			fresh := latest(t, s, b.Booking.ID)
			if fresh.EmailStatus != "Sent" {
				t.Fatal("not marked Sent", fresh.EmailStatus)
			}
			raw := <-messages
			parts := readEmail(t, raw)
			for _, body := range parts {
				if !strings.Contains(body, b.TrackingURL) {
					t.Fatal("wrong recipient tracking link")
				}
			}
			if !strings.Contains(parts["text/html"], "ช่วงเวลา Consult:") {
				t.Fatal("wrong template")
			}
		})
	}
}
