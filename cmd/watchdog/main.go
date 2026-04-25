package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"
	"github.com/maxlesscode/watchdog/internal/database"
	"github.com/maxlesscode/watchdog/internal/handlers"
	"github.com/maxlesscode/watchdog/internal/logger"
	m "github.com/maxlesscode/watchdog/internal/middleware"
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
	mux := http.NewServeMux()

	srv := http.Server{
		Addr:         ":9999",
		Handler:      m.LoggingMiddleware(m.APIKeyMiddleware(mux)),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  90 * time.Second,
	}

	store := database.NewPostgresStore(db)
	env := &handlers.Env{DB: store}

	mux.HandleFunc("GET /products", env.GetAllProducts)
	mux.HandleFunc("GET /products/{id}", env.GetProductByID)
	mux.HandleFunc("POST /products", env.CreateProduct)
	mux.HandleFunc("PATCH /products/{id}", env.UpdateProduct)
	mux.HandleFunc("DELETE /products/{id}", env.DeleteProduct)
	mux.HandleFunc("GET /health", env.HealthCheck)

	slog.Info("HTTP Server started")
	if err = srv.ListenAndServe(); err != nil {
		slog.Error("server stopped", "err", err)
	}
}
