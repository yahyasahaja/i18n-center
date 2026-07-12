// Package main is the i18n-center migration tool — a thin wrapper over goose
// that ships in the same binary image as the server. Operators run it manually
// (`kubectl exec ... -- i18n-center-migrate up`) before bumping a deploy; the
// server itself never touches the schema at boot.
//
// Usage:
//
//	i18n-center-migrate <command> [args]
//
// Commands:
//
//	up                  apply all pending migrations
//	up-by-one           apply just the next pending migration
//	down                roll back the most recent migration
//	redo                roll back + re-apply the most recent migration
//	status              list applied vs pending migrations
//	version             print current migration version
//	create <name> sql   scaffold a new SQL migration file in ./migrations
//
// Connection settings come from the same env vars as the server (DB_HOST etc.)
// so the same Vault-mounted secret works for both.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"github.com/pressly/goose/v3"

	"github.com/lapakgaming/i18n-center/migrations"
)

// allowedCommands documents what we expose. goose itself supports more, but
// pinning the surface area makes the operator-facing CLI explicit.
var allowedCommands = map[string]bool{
	"up":        true,
	"up-by-one": true,
	"down":      true,
	"redo":      true,
	"status":    true,
	"version":   true,
	"create":    true,
}

func main() {
	// .env is convenience-only for local dev. In K8s, env vars come from the
	// Pod spec (Vault → secret → env). godotenv silently no-ops if no file.
	_ = godotenv.Load()

	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	if !allowedCommands[cmd] {
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		usage()
		os.Exit(1)
	}

	// "create" is the only command that doesn't need a DB connection — it just
	// writes a new file to disk for the dev to fill in. It also needs to write
	// to the host filesystem (not embed.FS), so we use a different code path.
	if cmd == "create" {
		if err := runCreate(args); err != nil {
			log.Fatalf("migrate create failed: %v", err)
		}
		return
	}

	db, err := openDB()
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer db.Close()

	// Read migrations from the embedded FS. The current working directory
	// doesn't matter — the binary is self-contained.
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("mysql"); err != nil {
		log.Fatalf("set dialect: %v", err)
	}

	ctx := context.Background()
	if err := goose.RunContext(ctx, cmd, db, ".", args...); err != nil {
		log.Fatalf("migrate %s failed: %v", cmd, err)
	}
}

func openDB() (*sql.DB, error) {
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	if port == "" {
		port = "3306"
	}
	params := url.Values{}
	params.Set("parseTime", "true")
	params.Set("loc", "UTC")
	params.Set("multiStatements", "true")
	params.Set("charset", "utf8mb4")
	params.Set("collation", "utf8mb4_unicode_ci")
	switch strings.TrimSpace(defaultEnv("DB_SSLMODE", "disable")) {
	case "", "disable":
	case "require", "verify-full":
		params.Set("tls", "true")
	case "verify-ca":
		params.Set("tls", "skip-verify")
	default:
		params.Set("tls", defaultEnv("DB_SSLMODE", "disable"))
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?%s",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		host, port,
		os.Getenv("DB_NAME"),
		params.Encode(),
	)
	return sql.Open("mysql", dsn)
}

func defaultEnv(key, dflt string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return dflt
}

// runCreate writes a new empty migration file. Convenience for dev — production
// would never call this. goose handles the numbering based on existing files.
func runCreate(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: migrate create <name> [sql]")
	}
	name := args[0]
	migrationsDir := "migrations"
	if err := os.MkdirAll(migrationsDir, 0o755); err != nil {
		return err
	}
	if err := goose.SetDialect("mysql"); err != nil {
		return err
	}
	// goose.Create writes to the host filesystem, so don't set BaseFS here.
	return goose.Create(nil, migrationsDir, name, "sql")
}

func usage() {
	fmt.Fprint(os.Stderr, `i18n-center migration tool — runs against $DB_HOST.

Usage:
  i18n-center-migrate <command> [args]

Commands:
  up                  apply all pending migrations
  up-by-one           apply just the next pending migration
  down                roll back the most recent migration
  redo                roll back + re-apply the most recent migration
  status              list applied vs pending migrations
  version             print current migration version
  create <name> sql   scaffold a new SQL migration file in ./migrations

Environment variables (same as the server):
  DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME, DB_SSLMODE

Examples:
  i18n-center-migrate up                         # in a pod: apply all pending
  i18n-center-migrate status                     # what's been applied?
  go run ./cmd/migrate create add_foo_column sql # dev: new migration file

See backend/migrations/README.md for the safe-pattern playbook.
`)
}
