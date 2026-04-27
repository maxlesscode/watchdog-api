package database

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/golang-migrate/migrate/v4"
	migratepostgres "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

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

func runMigrations(db *sql.DB, dbname string) {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		log.Fatal("failed to load migration source: ", err)
	}

	driver, err := migratepostgres.WithInstance(db, &migratepostgres.Config{DatabaseName: dbname})
	if err != nil {
		log.Fatal("failed to create migration driver: ", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, dbname, driver)
	if err != nil {
		log.Fatal("failed to init migrator: ", err)
	}

	if err = m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Fatal("migration failed: ", err)
	}
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

	runMigrations(db, cfg.dbname)

	return db
}
