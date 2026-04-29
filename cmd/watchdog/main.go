package main

import (
	"context"
	"errors"
	"expvar"
	"log"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/maxlesscode/watchdog/internal/database"
	"github.com/maxlesscode/watchdog/internal/handlers"
	"github.com/maxlesscode/watchdog/internal/logger"
	_ "github.com/maxlesscode/watchdog/internal/metrics" // register expvar counters
	m "github.com/maxlesscode/watchdog/internal/middleware"
	"github.com/maxlesscode/watchdog/internal/notifier"
	"github.com/maxlesscode/watchdog/internal/scheduler"
	"github.com/maxlesscode/watchdog/internal/scraper"
)

var version = "dev"

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file, reading from environment")
	}

	isDev := os.Getenv("IS_DEV") == "true"

	logPath := os.Getenv("LOG_PATH")
	if logPath == "" {
		logPath = "watchdog.log"
	}

	appLogger, cleanup, err := logger.New(logPath, slog.LevelInfo, isDev)
	if err != nil {
		log.Fatal("failed to initialize logger: ", err)
	}
	defer cleanup()
	slog.SetDefault(appLogger)

	slog.Info("starting watchdog", "version", version)

	if os.Getenv("API_KEY") == "" {
		slog.Error("API_KEY must be set")
		os.Exit(1)
	}

	dbCfg := database.Config{
		Host:     os.Getenv("DB_HOST"),
		Port:     os.Getenv("DB_PORT"),
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		DBName:   os.Getenv("DB_NAME"),
		SSLMode:  os.Getenv("DB_SSL_MODE"),
	}
	if dbCfg.Host == "" || dbCfg.Port == "" || dbCfg.User == "" || dbCfg.DBName == "" {
		slog.Error("required DB env vars not set", "required", "DB_HOST, DB_PORT, DB_USER, DB_NAME")
		os.Exit(1)
	}

	db, err := database.Connect(dbCfg)
	if err != nil {
		slog.Error("database connect failed", "err", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	store := database.NewPostgresStore(db)
	htmlScraper := scraper.NewHTMLScraper(scraper.NewSafeHTTPClient(5 * time.Second))
	resilientScraper := scraper.NewResilientScraper(htmlScraper, scraper.ResilientConfig{
		MaxAttempts:      3,
		InitialDelay:     time.Second,
		CircuitThreshold: 5,
		CircuitCooldown:  5 * time.Minute,
	})

	sched := scheduler.New(scheduler.Config{
		Store:    store,
		Scraper:  resilientScraper,
		Notifier: loadNotifier(),
	})
	go sched.Run(ctx)

	mux := http.NewServeMux()
	env := &handlers.Env{
		DB:            store,
		TriggerScrape: sched.RunCycle,
	}

	mux.HandleFunc("GET /products", env.GetAllProducts)
	mux.HandleFunc("GET /products/{id}", env.GetProductByID)
	mux.HandleFunc("POST /products", env.CreateProduct)
	mux.HandleFunc("PUT /products/{id}", env.UpdateProduct)
	mux.HandleFunc("DELETE /products/{id}", env.DeleteProduct)
	mux.HandleFunc("GET /health", env.HealthCheck)
	mux.HandleFunc("POST /admin/scrape", env.AdminScrape)
	mux.HandleFunc("GET /products/{id}/history", env.GetPriceHistory)

	rateLimitRPS := envFloat("RATE_LIMIT", 10.0)
	rateBurst := envInt("RATE_BURST", 20)

	corsOrigins := parseCORSOrigins(os.Getenv("CORS_ORIGINS"))

	addr := os.Getenv("SERVER_ADDR")
	if addr == "" {
		addr = ":9999"
	}

	srv := http.Server{
		Addr: addr,
		Handler: chain(mux,
			m.CORSMiddleware(corsOrigins),
			m.LoggingMiddleware,
			m.RateLimitMiddleware(ctx, rateLimitRPS, rateBurst),
			m.APIKeyMiddleware,
		),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  90 * time.Second,
	}

	pprofSrv := buildPprofServer(os.Getenv("PPROF_ADDR"))
	go func() {
		slog.Info("pprof server started", "addr", pprofSrv.Addr)
		if err := pprofSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("pprof server error", "err", err)
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("server shutdown error", "err", err)
		}
		if err := pprofSrv.Shutdown(shutdownCtx); err != nil {
			slog.Error("pprof shutdown error", "err", err)
		}
	}()

	slog.Info("HTTP server started", "addr", addr)
	if err = srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server stopped", "err", err)
	}
}

// chain applies middlewares in order: the first middleware is outermost (runs first on each request).
func chain(h http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

func buildPprofServer(addr string) *http.Server {
	if addr == "" {
		addr = "127.0.0.1:9998"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	mux.Handle("/debug/vars", expvar.Handler())
	return &http.Server{
		Addr:        addr,
		Handler:     mux,
		ReadTimeout: 30 * time.Second,
		IdleTimeout: 60 * time.Second,
	}
}

// loadNotifier returns an SMTPNotifier if SMTP_HOST is set, nil otherwise.
func loadNotifier() notifier.Notifier {
	cfg := notifier.SMTPConfig{
		Host:       os.Getenv("SMTP_HOST"),
		Port:       os.Getenv("SMTP_PORT"),
		User:       os.Getenv("SMTP_USER"),
		Pass:       os.Getenv("SMTP_PASS"),
		AlertEmail: os.Getenv("ALERT_EMAIL"),
	}
	if cfg.Host == "" {
		slog.Warn("SMTP_HOST not set — email notifications disabled")
		return nil
	}
	if err := cfg.Validate(); err != nil {
		slog.Error("invalid SMTP config", "err", err)
		os.Exit(1)
	}
	return notifier.NewSMTPNotifier(cfg)
}

// parseCORSOrigins splits a comma-separated CORS_ORIGINS value into a slice.
func parseCORSOrigins(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func envFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}
