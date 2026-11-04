package main

import (
	"testing"
	"warwick/consult/internal/consult"
)

func TestSheetsSyncConfiguration(t *testing.T) {
	t.Setenv("SHEETS_SYNC_URL", "")
	t.Setenv("SHEETS_SYNC_SECRET", "")
	if _, on, err := configuredSheetSync(true); err != nil || on {
		t.Fatal("not off by default")
	}
	t.Setenv("SHEETS_SYNC_URL", "https://script.google.com/macros/s/deployment_fixture_123/exec")
	if _, _, err := configuredSheetSync(true); err == nil {
		t.Fatal("missing secret accepted")
	}
	t.Setenv("SHEETS_SYNC_SECRET", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if _, on, err := configuredSheetSync(true); err != nil || !on {
		t.Fatal(err)
	}
	if _, _, err := configuredSheetSync(false); err == nil {
		t.Fatal("local test could sync into production sheet")
	}
	t.Setenv("SHEETS_SYNC_URL", "https://accounts.google.com/login")
	if _, _, err := configuredSheetSync(true); err == nil {
		t.Fatal("unexpected endpoint accepted")
	}
}

func TestRailwayPortAndDomain(t *testing.T) {
	t.Setenv("PORT", "9191")
	t.Setenv("HTTP_ADDR", "127.0.0.1:8080")
	if a, e := listenAddress(); e != nil || a != "0.0.0.0:9191" {
		t.Fatal(a, e)
	}
	for _, port := range []string{"0", "65536", "invalid", "80:80"} {
		t.Setenv("PORT", port)
		if _, e := listenAddress(); e == nil {
			t.Fatal("invalid port accepted")
		}
	}
	t.Setenv("PORT", "")
	if a, _ := listenAddress(); a != "127.0.0.1:8080" {
		t.Fatal("local listen changed")
	}
	t.Setenv("PUBLIC_URL", "")
	t.Setenv("RAILWAY_PUBLIC_DOMAIN", "consult.up.railway.app")
	if u, e := publicOrigin(); e != nil || u != "https://consult.up.railway.app" {
		t.Fatal(u, e)
	}
	t.Setenv("PUBLIC_URL", "https://consult.example")
	if u, _ := publicOrigin(); u != "https://consult.example" {
		t.Fatal("explicit URL ignored")
	}
}
func TestEmailProviderConfiguration(t *testing.T) {
	for _, name := range []string{"EMAIL_PROVIDER", "SMTP_HOST", "SMTP_FROM", "RESEND_API_KEY", "EMAIL_FROM", "APPS_SCRIPT_URL", "APPS_SCRIPT_SECRET"} {
		t.Setenv(name, "")
	}
	cfg := consult.Config{}
	if _, enabled, e := configuredMailer(cfg); e != nil || enabled {
		t.Fatal("mail not off by default")
	}
	t.Setenv("EMAIL_PROVIDER", "apps-script")
	if _, _, e := configuredMailer(cfg); e == nil {
		t.Fatal("incomplete Apps Script config accepted")
	}
	t.Setenv("APPS_SCRIPT_URL", "https://script.google.com/macros/s/deployment_fixture_123/exec")
	t.Setenv("APPS_SCRIPT_SECRET", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if mailer, on, e := configuredMailer(cfg); e != nil || !on {
		t.Fatal("Apps Script not enabled", e)
	} else if _, ok := mailer.(consult.AppsScriptMailer); !ok {
		t.Fatal("wrong Apps Script adapter")
	}
	t.Setenv("EMAIL_PROVIDER", "resend")
	if _, _, e := configuredMailer(cfg); e == nil {
		t.Fatal("missing key accepted")
	}
	t.Setenv("RESEND_API_KEY", "fixture-key")
	t.Setenv("EMAIL_FROM", "consult@example.com")
	if _, on, e := configuredMailer(cfg); e != nil || !on {
		t.Fatal(e)
	}
	t.Setenv("EMAIL_PROVIDER", "off")
	if _, on, e := configuredMailer(cfg); e != nil || on {
		t.Fatal("explicit off ignored")
	}
	t.Setenv("EMAIL_PROVIDER", "")
	t.Setenv("SMTP_HOST", "smtp.example.com")
	if _, _, e := configuredMailer(cfg); e == nil {
		t.Fatal("partial SMTP config")
	}
	t.Setenv("SMTP_FROM", "consult@example.com")
	if _, on, e := configuredMailer(cfg); e != nil || !on {
		t.Fatal("legacy SMTP config broken")
	}
}
