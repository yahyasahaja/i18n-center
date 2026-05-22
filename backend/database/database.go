// Package database owns the connection lifecycle for the shared Cloud SQL
// Postgres pool. As of Commit I the whole repository layer is sqlx-backed —
// GORM is gone. The package exports a single handle:
//
//   - SQLX (*sqlx.DB) — used by every repository under backend/repository/*.
//
// No schema mutation happens here: migrations are applied manually via the
// `i18n-center-migrate` CLI, so a fresh pod boots cleanly even if the schema
// is up-to-date (or 500s every query if it isn't).
package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // sql driver registration
)

// SQLX is the single application-wide DB handle. Repositories accept a
// `repository.Queryer` (satisfied by both *sqlx.DB and *sqlx.Tx) so they can
// run inside or outside a transaction without two code paths.
var SQLX *sqlx.DB

// InitDatabase opens the database connection and sizes the pool. It does NOT
// migrate the schema — that's the job of the `i18n-center-migrate` binary,
// run manually before each deploy that includes a schema change.
//
// If the schema is missing entirely, the server will boot fine but every
// query will fail with `relation "..." does not exist`. The fix is to exec
// into the pod and run `i18n-center-migrate up`.
func InitDatabase() error {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_SSLMODE"),
	)

	// Open with database/sql so we can size the pool before wrapping in sqlx.
	sqlDB, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Connection pool sizing. The Cloud SQL Postgres is shared with Hydra
	// (OAuth2 server in the B2C login hot path), so an unbounded pool here
	// can starve Hydra under load. Defaults are conservative; tune via env.
	//
	//   DB_MAX_OPEN_CONNS — total connections per pod. Default 20.
	//   DB_MAX_IDLE_CONNS — idle connections kept alive. Default 5.
	//   DB_CONN_MAX_LIFETIME_MIN — rotate connections every N min to align
	//       with Cloud SQL's idle-disconnect window. Default 30.
	//
	// Budget: 3 replicas × 20 = 60 of Cloud SQL's default max_connections=100.
	sqlDB.SetMaxOpenConns(envIntOr("DB_MAX_OPEN_CONNS", 20))
	sqlDB.SetMaxIdleConns(envIntOr("DB_MAX_IDLE_CONNS", 5))
	sqlDB.SetConnMaxLifetime(time.Duration(envIntOr("DB_CONN_MAX_LIFETIME_MIN", 30)) * time.Minute)

	// Confirm the pool can actually reach the server. Without this, a wrong
	// host/credentials surfaces as a per-request error later — boot-time
	// failure is loud and obvious.
	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	SQLX = sqlx.NewDb(sqlDB, "postgres")

	log.Println("Database connected. Reminder: schema is NOT migrated automatically — run `i18n-center-migrate up` in the pod before sending traffic on a fresh deploy.")
	return nil
}

// envIntOr reads an env var as int, falling back to dflt if unset or unparseable.
// Used for pool-sizing knobs that operations may want to tune per environment.
func envIntOr(key string, dflt int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return dflt
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		log.Printf("invalid %s=%q (using default %d): %v", key, raw, dflt, err)
		return dflt
	}
	return n
}
