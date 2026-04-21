package main

import (
	"log/slog"
	"net/http"
	"time"

	_ "github.com/lib/pq"
	"github.com/maxlesscode/watchdog/internal/database"
	"github.com/maxlesscode/watchdog/internal/handlers"
	"github.com/maxlesscode/watchdog/internal/logger"
	m "github.com/maxlesscode/watchdog/internal/middleware"
)

const (
	isDev = true
)

func main() {
	db := database.StartDB()
	defer db.Close()

	mux := http.NewServeMux()

	srv := http.Server{
		Addr:         ":9999",
		Handler:      m.LoggingMiddleware(m.APIKeyMiddleware(mux)),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  90 * time.Second,
	}

	store := &database.PostgresStore{}
	env := &handlers.Env{DB: store}

	logger, cleanup, err := logger.New("watchdog.log", slog.LevelInfo, isDev)
	if err != nil {
		slog.Error("Can't load logger", "err", err)
	}
	defer cleanup()
	slog.SetDefault(logger)

	mux.HandleFunc("GET /products", env.GetAllProducts)
	mux.HandleFunc("GET /products/{id}", env.GetProductByID)
	mux.HandleFunc("POST /products", env.CreateProduct)
	mux.HandleFunc("PATCH /products/{id}", env.UpdateProduct)
	mux.HandleFunc("DELETE /products/{id}", env.DeleteProduct)
	mux.HandleFunc("GET /health", env.HealthCheck)

	slog.Info("HTTP Server started")
	srv.ListenAndServe()
}
