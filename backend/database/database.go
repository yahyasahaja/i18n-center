// Package database owns the connection lifecycle for the shared Cloud SQL
// MySQL pool. The whole repository layer is sqlx-backed; the package exports a
// single handle:
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
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql" // sql driver registration
	"github.com/jmoiron/sqlx"
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
// query will fail with `Table '...' doesn't exist`. The fix is to exec into
// the pod and run `i18n-center-migrate up`.
func InitDatabase() error {
	dsn := buildDSN(
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_SSLMODE"),
	)

	sqlDB, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	//   DB_MAX_OPEN_CONNS — total connections per pod. Default 20.
	//   DB_MAX_IDLE_CONNS — idle connections kept alive. Default 5.
	//   DB_CONN_MAX_LIFETIME_MIN — rotate connections every N min to align
	//       with Cloud SQL's idle-disconnect window. Default 30.
	sqlDB.SetMaxOpenConns(envIntOr("DB_MAX_OPEN_CONNS", 20))
	sqlDB.SetMaxIdleConns(envIntOr("DB_MAX_IDLE_CONNS", 5))
	sqlDB.SetConnMaxLifetime(time.Duration(envIntOr("DB_CONN_MAX_LIFETIME_MIN", 30)) * time.Minute)

	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	SQLX = sqlx.NewDb(sqlDB, "mysql")

	log.Println("Database connected. Reminder: schema is NOT migrated automatically — run `i18n-center-migrate up` in the pod before sending traffic on a fresh deploy.")
	return nil
}

// buildDSN assembles the go-sql-driver/mysql DSN. Shape:
//
//	user:pass@tcp(host:port)/dbname?parseTime=true&loc=UTC&multiStatements=true[&tls=...]
//
// parseTime=true is required for time.Time scanning; loc=UTC keeps timestamps
// tz-consistent across pods. tls maps from a Postgres-shaped DB_SSLMODE for
// backwards compatibility with existing k8s secrets:
//
//	disable/"" → no TLS (default; internal VPC to Cloud SQL private IP)
//	require    → tls=true (encrypted, no CA verify)
//	verify-ca  → tls=skip-verify (encrypted, hostname skipped)
//	verify-full → tls=true and rely on the driver's built-in verification
//
// Anything else is passed through verbatim as the tls param.
func buildDSN(host, port, user, password, name, sslmode string) string {
	if port == "" {
		port = "3306"
	}
	params := url.Values{}
	params.Set("parseTime", "true")
	params.Set("loc", "UTC")
	params.Set("multiStatements", "true")
	params.Set("charset", "utf8mb4")
	params.Set("collation", "utf8mb4_unicode_ci")

	switch strings.TrimSpace(sslmode) {
	case "", "disable":
		// no tls param
	case "require":
		params.Set("tls", "true")
	case "verify-ca":
		params.Set("tls", "skip-verify")
	case "verify-full":
		params.Set("tls", "true")
	default:
		params.Set("tls", sslmode)
	}

	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?%s",
		user, password, host, port, name, params.Encode(),
	)
}

// envIntOr reads an env var as int, falling back to dflt if unset or unparseable.
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
