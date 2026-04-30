package database

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"net/url"

	"github.com/golang-migrate/migrate/v4"
	migratepostgres "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/lib/pq"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Config holds database connection settings. SSLMode defaults to "require" when empty.
type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// PostgresStore wraps a *sql.DB and implements ProductStore.
type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func runMigrations(db *sql.DB, dbname string) error {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("load migration source: %w", err)
	}

	driver, err := migratepostgres.WithInstance(db, &migratepostgres.Config{DatabaseName: dbname})
	if err != nil {
		return fmt.Errorf("create migration driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, dbname, driver)
	if err != nil {
		return fmt.Errorf("init migrator: %w", err)
	}

	if err = m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

// MigrateDB applies all pending migrations to db. Exported for integration tests.
func MigrateDB(db *sql.DB) error {
	var dbname string
	if err := db.QueryRow("SELECT current_database()").Scan(&dbname); err != nil {
		return fmt.Errorf("get db name: %w", err)
	}
	return runMigrations(db, dbname)
}

// Connect opens a Postgres connection using cfg, pings it, and runs pending migrations.
func Connect(cfg Config) (*sql.DB, error) {
	if cfg.SSLMode == "" {
		cfg.SSLMode = "require"
	}

	dsn := &url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(cfg.User, cfg.Password),
		Host:     cfg.Host + ":" + cfg.Port,
		Path:     "/" + cfg.DBName,
		RawQuery: "sslmode=" + url.QueryEscape(cfg.SSLMode),
	}

	db, err := sql.Open("postgres", dsn.String())
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err = db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}

	if err = runMigrations(db, cfg.DBName); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrations: %w", err)
	}

	return db, nil
}
