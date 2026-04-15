package database

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type Config struct {
	host     string
	port     string
	user     string
	password string
	dbname   string
}

func configDB() Config {
	var newConfig Config

	err := godotenv.Load()
	if err != nil {
		slog.Error("failed to load .env", "err", err)
	}

	newConfig.host = os.Getenv("DB_HOST")
	newConfig.port = os.Getenv("DB_PORT")
	newConfig.user = os.Getenv("DB_USER")
	newConfig.password = os.Getenv("DB_PASSWORD")
	newConfig.dbname = os.Getenv("DB_NAME")

	return newConfig
}

func StartDB() *sql.DB {
	cfg := configDB()

	sqlInfo := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.host, cfg.port, cfg.user, cfg.password, cfg.dbname)

	db, err := sql.Open("postgres", sqlInfo)
	if err != nil {
		slog.Error("failed to open db", "err", err)
	}

	if err = db.Ping(); err != nil {
		slog.Error("failed to ping db", "err", err)
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS products (id SERIAL, name TEXT, url TEXT, actual_price NUMERIC, target_price NUMERIC)")
	if err != nil {
		slog.Error("failed to create main table", "err", err)
	}

	return db
}
