package main

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"

	_ "github.com/lib/pq"
	"github.com/maxlesscode/watchdog/internal/database"
	"github.com/maxlesscode/watchdog/internal/handlers"
	"github.com/maxlesscode/watchdog/internal/logger"
)

const (
	isDev = true
)

func main() {
	database := database.StartDB()
	defer database.Close()

	env := &handlers.Env{Db: database}

	logger, cleanup, err := logger.New("watchdog.log", slog.LevelInfo, isDev)
	if err != nil {
		log.Fatal("Can't load logger - ", err)
	}
	defer cleanup()
	slog.SetDefault(logger)

	http.HandleFunc("GET /products", env.GetAllProducts)
	http.HandleFunc("GET /products/{id}", env.GetProductByID)
	http.HandleFunc("POST /products", env.CreateProduct)
	http.HandleFunc("PATCH /products/{id}", env.UpdateProduct)
	http.HandleFunc("DELETE /products/{id}", env.DeleteProduct)

	fmt.Println("HTTP Server up at 'localhost:9999'")
	http.ListenAndServe(":9999", nil)
}
