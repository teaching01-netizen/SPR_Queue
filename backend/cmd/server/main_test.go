package main

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"warwick/consult/internal/consult"
)

func TestInitCreatesPrivateSecretsWithoutOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.env")
	if e := initEnv(path); e != nil {
		t.Fatal(e)
	}
	bytes, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	values := map[string]string{}
	for _, line := range strings.Split(string(bytes), "\n") {
		k, v, ok := strings.Cut(line, "=")
		if ok {
			values[k] = v
		}
	}
	key, e := base64.StdEncoding.DecodeString(values["APP_SECRET"])
	if e != nil || len(key) != 32 || len(values["STAFF_PASSWORD"]) < 24 {
		t.Fatal("invalid generated secrets")
	}
	databaseURL, e := url.Parse(values["DATABASE_URL"])
	if e != nil || databaseURL.User == nil {
		t.Fatal("invalid database URL")
	}
	databasePassword, present := databaseURL.User.Password()
	if !present || len(databasePassword) < 24 || databasePassword != values["POSTGRES_PASSWORD"] || databasePassword == values["STAFF_PASSWORD"] {
		t.Fatal("database credentials not generated independently")
	}
	info, e := os.Stat(path)
	if e != nil || info.Mode().Perm() != 0600 {
		t.Fatal("configuration is not private")
	}
	if e := initEnv(path); e == nil {
		t.Fatal("existing settings overwritten")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(bytes) {
		t.Fatal("settings changed")
	}
}
func TestEnvironmentFileIsDataAndKeepsProcessOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.env")
	key := "WARWICK_TEST_ENV_OVERRIDE"
	t.Setenv(key, "process-value")
	if e := os.WriteFile(path, []byte(key+"=file-value\n"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := loadEnv(path); e != nil {
		t.Fatal(e)
	}
	if os.Getenv(key) != "process-value" {
		t.Fatal("process environment overwritten")
	}
}

func TestLocalContainerHealthcheck(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusServiceUnavailable} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }))
		err := checkHealth(server.URL)
		server.Close()
		if (status == http.StatusOK) != (err == nil) {
			t.Fatalf("status %d: %v", status, err)
		}
	}
	closed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := closed.URL
	closed.Close()
	if err := checkHealth(url); err == nil {
		t.Fatal("unreachable API accepted")
	}
}

func TestLocalDataResetBoundary(t *testing.T) {
	cfg := consult.Config{Event: consult.Event{ID: "local-test-nov", Sample: true}}
	dsn := "postgres://warwick_local_test:secret@db:5432/warwick_consult_local_test?sslmode=disable"
	if err := validateLocalDataReset("local-test", cfg, dsn); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"production", "development"} {
		if err := validateLocalDataReset(mode, cfg, dsn); err == nil {
			t.Fatal("reset outside local mode")
		}
	}
	for _, url := range []string{"postgres://warwick_local_test:secret@remote.example/warwick_consult_local_test", "postgres://warwick_local_test:secret@db/production", "postgres://admin:secret@db/warwick_consult_local_test", "not a valid URL"} {
		if err := validateLocalDataReset("local-test", cfg, url); err == nil {
			t.Fatal("accepted unsafe reset target")
		}
	}
	cfg.Event.Sample = false
	if err := validateLocalDataReset("local-test", cfg, dsn); err == nil {
		t.Fatal("non-sample reset")
	}
	cfg.Event.Sample = true
	cfg.Event.ID = "production-nov"
	if err := validateLocalDataReset("local-test", cfg, dsn); err == nil {
		t.Fatal("non-test event reset")
	}
}
