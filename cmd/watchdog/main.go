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
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/maxlesscode/watchdog/internal/database"
	"github.com/maxlesscode/watchdog/internal/handlers"
	"github.com/maxlesscode/watchdog/internal/logger"
	_ "github.com/maxlesscode/watchdog/internal/metrics" // register expvar counters
	m "github.com/maxlesscode/watchdog/internal/middleware"
	"github.com/maxlesscode/watchdog/internal/notifier"
	"github.com/maxlesscode/watchdog/internal/scheduler"
	"github.com/maxlesscode/watchdog/internal/scraper"
)

func main() {
	isDev := os.Getenv("IS_DEV")

	appLogger, cleanup, err := logger.New("watchdog.log", slog.LevelInfo, isDev)
	if err != nil {
		log.Fatal("failed to initialize logger: ", err)
	}
	defer cleanup()
	slog.SetDefault(appLogger)

	db := database.StartDB()
	defer db.Close()

	if os.Getenv("API_KEY") == "" {
		log.Fatal("API_KEY must be set")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	store := database.NewPostgresStore(db)
	htmlScraper := scraper.NewHTMLScraper(&http.Client{Timeout: 5 * time.Second})
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
	mux.HandleFunc("PATCH /products/{id}", env.UpdateProduct)
	mux.HandleFunc("DELETE /products/{id}", env.DeleteProduct)
	mux.HandleFunc("GET /health", env.HealthCheck)
	mux.HandleFunc("POST /admin/scrape", env.AdminScrape)

	rateLimitRPS := envFloat("RATE_LIMIT", 10.0)
	rateBurst := envInt("RATE_BURST", 20)

	srv := http.Server{
		Addr:         ":9999",
		Handler:      m.CORSMiddleware(m.LoggingMiddleware(m.RateLimitMiddleware(rateLimitRPS, rateBurst)(m.APIKeyMiddleware(mux)))),
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

	slog.Info("HTTP Server started")
	if err = srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server stopped", "err", err)
	}
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
	return &http.Server{Addr: addr, Handler: mux}
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
		log.Fatal(err)
	}
	return notifier.NewSMTPNotifier(cfg)
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
