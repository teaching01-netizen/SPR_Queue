package main

import (
	"fmt"
	"net/mail"
	"os"
	"strconv"
	"strings"
	"warwick/consult/internal/consult"
)

func listenAddress() (string, error) {
	if port := os.Getenv("PORT"); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return "", fmt.Errorf("invalid PORT")
		}
		return "0.0.0.0:" + strconv.Itoa(n), nil
	}
	return env("HTTP_ADDR", "127.0.0.1:8080"), nil
}
func publicOrigin() (string, error) {
	raw := os.Getenv("PUBLIC_URL")
	if raw == "" && os.Getenv("RAILWAY_PUBLIC_DOMAIN") != "" {
		raw = "https://" + os.Getenv("RAILWAY_PUBLIC_DOMAIN")
	}
	if raw == "" {
		raw = "http://127.0.0.1:8080"
	}
	return consult.Origin(raw)
}
func configuredMailer(cfg consult.Config) (consult.Mailer, bool, error) {
	provider := os.Getenv("EMAIL_PROVIDER")
	if provider == "" {
		if os.Getenv("SMTP_HOST") != "" || os.Getenv("SMTP_FROM") != "" {
			provider = "smtp"
		} else {
			provider = "off"
		}
	}
	switch provider {
	case "off":
		return nil, false, nil
	case "apps-script":
		endpoint, secret := os.Getenv("APPS_SCRIPT_URL"), os.Getenv("APPS_SCRIPT_SECRET")
		if err := consult.ValidateAppsScriptSettings(endpoint, secret); err != nil {
			return nil, false, err
		}
		return consult.AppsScriptMailer{URL: endpoint, Secret: secret, Config: cfg}, true, nil
	case "resend":
		from := os.Getenv("EMAIL_FROM")
		if _, err := mail.ParseAddress(from); err != nil || strings.ContainsAny(from, "\r\n") || os.Getenv("RESEND_API_KEY") == "" {
			return nil, false, fmt.Errorf("set EMAIL_FROM and RESEND_API_KEY for HTTPS email delivery")
		}
		return consult.ResendMailer{APIKey: os.Getenv("RESEND_API_KEY"), From: from, Config: cfg}, true, nil
	case "smtp":
		if os.Getenv("SMTP_HOST") == "" || os.Getenv("SMTP_FROM") == "" {
			return nil, false, fmt.Errorf("set both SMTP_HOST and SMTP_FROM")
		}
		return consult.SMTPMailer{Host: os.Getenv("SMTP_HOST"), Port: env("SMTP_PORT", "587"), Username: os.Getenv("SMTP_USERNAME"), Password: os.Getenv("SMTP_PASSWORD"), From: os.Getenv("SMTP_FROM"), Config: cfg, Security: env("SMTP_SECURITY", "starttls")}, true, nil
	default:
		return nil, false, fmt.Errorf("EMAIL_PROVIDER must be off, apps-script, smtp or resend")
	}
}
