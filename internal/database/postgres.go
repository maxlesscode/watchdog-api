package database

import (
	"database/sql"
	"fmt"
	"log"
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

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func configDB() Config {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file, reading from environment")
	}

	cfg := Config{
		host:     os.Getenv("DB_HOST"),
		port:     os.Getenv("DB_PORT"),
		user:     os.Getenv("DB_USER"),
		password: os.Getenv("DB_PASSWORD"),
		dbname:   os.Getenv("DB_NAME"),
	}

	if cfg.host == "" || cfg.port == "" || cfg.user == "" || cfg.dbname == "" {
		log.Fatal("DB_HOST, DB_PORT, DB_USER, DB_NAME must be set")
	}

	return cfg
}

func StartDB() *sql.DB {
	cfg := configDB()

	sqlInfo := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.host, cfg.port, cfg.user, cfg.password, cfg.dbname)

	db, err := sql.Open("postgres", sqlInfo)
	if err != nil {
		log.Fatal("failed to open db: ", err)
	}

	if err = db.Ping(); err != nil {
		log.Fatal("database not alive: ", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS products (
		id           SERIAL PRIMARY KEY,
		name         TEXT NOT NULL,
		url          TEXT NOT NULL,
		actual_price NUMERIC,
		target_price NUMERIC
	)`)
	if err != nil {
		log.Fatal("failed to create products table: ", err)
	}

	_, err = db.Exec(`ALTER TABLE products ADD COLUMN IF NOT EXISTS price_selector TEXT`)
	if err != nil {
		log.Fatal("failed to add price_selector column: ", err)
	}

	_, err = db.Exec(`ALTER TABLE products ADD COLUMN IF NOT EXISTS last_checked_at TIMESTAMPTZ`)
	if err != nil {
		log.Fatal("failed to add last_checked_at column: ", err)
	}

	_, err = db.Exec(`ALTER TABLE products ADD COLUMN IF NOT EXISTS last_alerted_at TIMESTAMPTZ`)
	if err != nil {
		log.Fatal("failed to add last_alerted_at column: ", err)
	}

	// Ensure products.id has a PRIMARY KEY — may be missing if table was created by an older schema.
	_, err = db.Exec(`DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint
				WHERE conrelid = 'products'::regclass AND contype = 'p'
			) THEN
				ALTER TABLE products ADD PRIMARY KEY (id);
			END IF;
		END;
	$$`)
	if err != nil {
		log.Fatal("failed to ensure products primary key: ", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS price_history (
		id         SERIAL PRIMARY KEY,
		product_id INTEGER NOT NULL REFERENCES products(id) ON DELETE CASCADE,
		price      NUMERIC NOT NULL,
		checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	if err != nil {
		log.Fatal("failed to create price_history table: ", err)
	}

	return db
}
