package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"github.com/jackc/pgx/v5"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	_ "time/tzdata"
	"warwick/consult/internal/consult"
)

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
func loadEnv(path string) error {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("invalid environment file")
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if strings.HasPrefix(v, "\"") {
			v, err = strconv.Unquote(v)
			if err != nil {
				return err
			}
		}
		if _, exists := os.LookupEnv(k); !exists {
			if err = os.Setenv(k, v); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}
func initEnv(path string) error {
	secret := make([]byte, 32)
	password := make([]byte, 24)
	databasePassword := make([]byte, 24)
	if _, err := rand.Read(secret); err != nil {
		return err
	}
	if _, err := rand.Read(password); err != nil {
		return err
	}
	if _, err := rand.Read(databasePassword); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "# Generated locally. Keep this file private and back it up with the database.\nAPP_ENV=development\nAPP_SECRET=%s\nSTAFF_USERNAME=staff\nSTAFF_PASSWORD=%s\nHTTP_ADDR=127.0.0.1:8080\nPUBLIC_URL=http://127.0.0.1:8080\nDEV_FRONTEND_ORIGIN=http://127.0.0.1:5173\nPOSTGRES_USER=warwick\nPOSTGRES_DB=warwick_consult\nPOSTGRES_PASSWORD=%s\nDATABASE_URL=postgres://warwick:%s@127.0.0.1:5432/warwick_consult?sslmode=disable\nEVENT_CONFIG=config/event.json\nFRONTEND_DIST=../frontend/dist\n# Optional SMTP: port 587 with STARTTLS\nSMTP_HOST=\nSMTP_PORT=587\nSMTP_USERNAME=\nSMTP_PASSWORD=\nSMTP_FROM=\n", base64.StdEncoding.EncodeToString(secret), base64.RawURLEncoding.EncodeToString(password), base64.RawURLEncoding.EncodeToString(databasePassword), base64.RawURLEncoding.EncodeToString(databasePassword))
	return err
}
func checkHealth(url string) error {
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("local API is not ready")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("local API health status: %d", response.StatusCode)
	}
	return nil
}
func validateLocalDataReset(mode string, cfg consult.Config, databaseURL string) error {
	parsed, err := pgx.ParseConfig(databaseURL)
	if err != nil || mode != "local-test" || !cfg.Event.Sample || !strings.HasPrefix(cfg.Event.ID, "local-test-") {
		return fmt.Errorf("Data reset requires a local-test sample event")
	}
	if parsed.Database != "warwick_consult_local_test" || parsed.User != "warwick_local_test" || (parsed.Host != "db" && parsed.Host != "127.0.0.1" && parsed.Host != "localhost") {
		return fmt.Errorf("Data reset requires the dedicated local-test database/account on a local host")
	}
	return nil
}
func main() {
	healthcheck := flag.Bool("healthcheck", false, "check the local API for container readiness")
	envPath := flag.String("env-file", ".env", "local environment file")
	initOnly := flag.Bool("init", false, "generate private .env once")
	resetData := flag.Bool("reset-local-test-data", false, "clear all local-test bookings, queues, history and sessions; preserve Staff credentials")
	testEmail := flag.String("test-email", "", "send one SMTP delivery test to the specified recipient without booking")
	provision := flag.Bool("provision-staff", false, "explicitly apply the seven production Staff accounts and exit")
	reset := flag.Bool("reset-staff-password", false, "explicitly replace configured staff password and revoke sessions")
	flag.Parse()
	if *healthcheck {
		addr, e := listenAddress()
		if e != nil {
			log.Fatal(e)
		}
		_, port, e := net.SplitHostPort(addr)
		if e != nil {
			log.Fatal("invalid HTTP_ADDR")
		}
		if err := checkHealth("http://127.0.0.1:" + port + "/api/health"); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *initOnly {
		if err := initEnv(*envPath); err != nil {
			log.Fatal(err)
		}
		fmt.Println("Created private environment file. Open it locally to see your Staff login.")
		return
	}
	if err := loadEnv(*envPath); err != nil {
		log.Fatal(err)
	}
	key, err := base64.StdEncoding.DecodeString(os.Getenv("APP_SECRET"))
	if err != nil || len(key) != 32 {
		log.Fatal("APP_SECRET is required: run go run ./cmd/server -init first")
	}
	cfg, err := consult.LoadConfig(env("EVENT_CONFIG", "config/event.json"))
	if err != nil {
		log.Fatal(err)
	}
	mode := env("APP_ENV", "development")
	if mode != "development" && mode != "local-test" && mode != "production" {
		log.Fatal("APP_ENV must be development, local-test or production")
	}
	if mode == "local-test" && (!cfg.Event.Sample || !strings.HasPrefix(cfg.Event.ID, "local-test-")) {
		log.Fatal("Local testing requires a sample event with a local-test ID")
	}
	production := mode == "production"
	if *provision && !production {
		log.Fatal("-provision-staff is only for production")
	}
	if *reset && production {
		log.Fatal("use -provision-staff for production credential changes")
	}
	publicURL, err := publicOrigin()
	if err != nil {
		log.Fatal(err)
	}
	if production && (cfg.Event.Sample || !strings.HasPrefix(publicURL, "https://")) {
		log.Fatal("Production requires HTTPS PUBLIC_URL and a confirmed event with sample=false")
	}
	if os.Getenv("DATABASE_URL") == "" {
		log.Fatal("DATABASE_URL is required for PostgreSQL")
	}
	if *resetData {
		if err = validateLocalDataReset(mode, cfg, os.Getenv("DATABASE_URL")); err != nil {
			log.Fatal(err)
		}
	}
	service, err := consult.Open(os.Getenv("DATABASE_URL"), cfg, key, publicURL)
	if err != nil {
		log.Fatal(err)
	}
	defer service.DB.Close()
	service.LocalTestDataManagement = validateLocalDataReset(mode, cfg, os.Getenv("DATABASE_URL")) == nil
	if *resetData {
		resetCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err = service.ResetLocalTestData(resetCtx); err != nil {
			log.Fatal(err)
		}
		fmt.Println("All local-test bookings, queues, history and sessions cleared. Staff credentials preserved; current schedule is ready.")
		return
	}
	auth, err := consult.NewAuth(service, production || strings.HasPrefix(publicURL, "https://"))
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if mode == "local-test" {
		err = auth.SyncLocalStaff(ctx)
	} else if production {
		err = auth.ProvisionProductionStaff(ctx, os.Getenv("STAFF_ACCOUNTS_JSON"), *provision)
	} else {
		err = auth.Bootstrap(ctx, env("STAFF_USERNAME", "staff"), os.Getenv("STAFF_PASSWORD"), *reset)
	}
	if err != nil {
		log.Fatal(err)
	}
	if *provision {
		if !production {
			log.Fatal("-provision-staff is only for production")
		}
		fmt.Println("Production Staff roster applied; bookings and queue preserved.")
		return
	}
	origins := []string{publicURL}
	if !production {
		if dev := os.Getenv("DEV_FRONTEND_ORIGIN"); dev != "" {
			o, e := consult.Origin(dev)
			if e != nil {
				log.Fatal(e)
			}
			origins = append(origins, o)
		}
	}
	mailer, enabled, err := configuredMailer(cfg)
	if err != nil {
		log.Fatal(err)
	}
	service.MailEnabled = enabled
	if *testEmail != "" {
		if !service.MailEnabled {
			log.Fatal("Configure an email provider before sending a test email")
		}
		rule := cfg.Rules[0]
		period := rule.Periods[0]
		booking := consult.Booking{ID: "smtp-test", Input: consult.Input{Email: *testEmail, Nickname: "ทดสอบอีเมล (ไม่ใช่การจอง)", Level: rule.Level, Subject: rule.Subject, Date: rule.Date, PeriodID: period.ID}, Mode: rule.Mode}
		testCtx, stop := context.WithTimeout(ctx, 30*time.Second)
		defer stop()
		if e := mailer.Send(testCtx, booking, publicURL+"/#/book"); e != nil {
			log.Fatal("Email test failed. Check provider settings and sender verification.")
		}
		fmt.Println("Email provider accepted the test message. Check the recipient inbox and spam folder.")
		return
	}
	if service.MailEnabled {
		log.Print("Email delivery enabled; confirmation messages are processed by the outbox worker")
	} else {
		log.Print("Email delivery not configured")
	}
	handler := consult.NewHTTP(service, auth, origins, env("FRONTEND_DIST", "../frontend/dist"))
	addr, err := listenAddress()
	if err != nil {
		log.Fatal(err)
	}
	server := &http.Server{Addr: addr, Handler: handler.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	var worker sync.WaitGroup
	defer worker.Wait()
	if service.MailEnabled {
		worker.Add(1)
		go func() {
			defer worker.Done()
			ticker := time.NewTicker(3 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if _, e := service.ProcessMail(ctx, mailer); e != nil {
						handler.Logger.Error("notification_worker_failure", "category", "INTERNAL_ERROR")
					}
				}
			}
		}()
	}
	go func() {
		<-ctx.Done()
		shutdown, c := context.WithTimeout(context.Background(), 10*time.Second)
		defer c()
		_ = server.Shutdown(shutdown)
	}()
	log.Printf("Warwick Go API listening at %s (sample event=%t)", server.Addr, cfg.Event.Sample)
	if err = server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
