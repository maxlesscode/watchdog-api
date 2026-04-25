package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/maxlesscode/watchdog/internal/database"
	"github.com/maxlesscode/watchdog/internal/handlers"
	"github.com/maxlesscode/watchdog/internal/logger"
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

	sched := scheduler.New(scheduler.Config{
		Store:    store,
		Scraper:  htmlScraper,
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

	srv := http.Server{
		Addr:         ":9999",
		Handler:      m.LoggingMiddleware(m.APIKeyMiddleware(mux)),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  90 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("server shutdown error", "err", err)
		}
	}()

	slog.Info("HTTP Server started")
	if err = srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server stopped", "err", err)
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
		log.Fatal(err)
	}
	return notifier.NewSMTPNotifier(cfg)
}
