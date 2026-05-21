package database

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/your-org/i18n-center/observability"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Two handles share the same underlying connection pool:
//
//   - DB (*gorm.DB)   — used by handlers/services that still go through the
//                       legacy ORM path. Will be removed once every package
//                       has migrated to the repository layer (Commit I).
//   - SQLX (*sqlx.DB) — used by the new raw-SQL repositories. Lives in
//                       backend/repository/*.
//
// Both pull connections from the same *sql.DB so we don't double-budget the
// shared-with-Hydra Cloud SQL pool.
var (
	DB   *gorm.DB
	SQLX *sqlx.DB
)

// InitDatabase opens the database connection, sizes the pool, and prepares
// both the GORM and sqlx handles. It does NOT migrate the schema — that's
// the job of the `i18n-center-migrate` binary, run manually before each
// deploy that includes a schema change.
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

	// GORM SQL logging: silent by default; set LOG_SQL=true to enable.
	gormLogLevel := logger.Silent
	if os.Getenv("LOG_SQL") == "true" {
		gormLogLevel = logger.Info
	}

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(gormLogLevel),
	})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
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
	sqlDB, dbErr := DB.DB()
	if dbErr != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", dbErr)
	}
	sqlDB.SetMaxOpenConns(envIntOr("DB_MAX_OPEN_CONNS", 20))
	sqlDB.SetMaxIdleConns(envIntOr("DB_MAX_IDLE_CONNS", 5))
	sqlDB.SetConnMaxLifetime(time.Duration(envIntOr("DB_CONN_MAX_LIFETIME_MIN", 30)) * time.Minute)

	// Wrap the same *sql.DB in sqlx so the new repository layer shares the
	// pool with the legacy GORM path. Connections are pooled once, not twice.
	SQLX = sqlx.NewDb(sqlDB, "postgres")

	setupObservabilityCallbacks()

	log.Println("Database connected. Reminder: schema is NOT migrated automatically — run `i18n-center-migrate up` in the pod before sending traffic on a fresh deploy.")
	return nil
}

// setupObservabilityCallbacks adds callbacks to track database operations
func setupObservabilityCallbacks() {
	if DB == nil {
		return
	}

	// Track query execution time and errors
	DB.Callback().Query().Before("gorm:query").Register("observability:before_query", func(db *gorm.DB) {
		db.InstanceSet("start_time", time.Now())
	})

	DB.Callback().Query().After("gorm:query").Register("observability:after_query", func(db *gorm.DB) {
		startTime, ok := db.InstanceGet("start_time")
		if !ok {
			return
		}

		duration := time.Since(startTime.(time.Time))
		operation := "query"

		if db.Error != nil {
			observability.LogError(db.Error, "Database query error",
				zap.String("operation", operation),
				zap.Duration("duration", duration),
			)
		}

		observability.RecordDatabaseMetrics(operation, duration, db.Error)
	})

	// Track create operations
	DB.Callback().Create().Before("gorm:create").Register("observability:before_create", func(db *gorm.DB) {
		db.InstanceSet("start_time", time.Now())
	})

	DB.Callback().Create().After("gorm:create").Register("observability:after_create", func(db *gorm.DB) {
		startTime, ok := db.InstanceGet("start_time")
		if !ok {
			return
		}

		duration := time.Since(startTime.(time.Time))
		observability.RecordDatabaseMetrics("create", duration, db.Error)
	})

	// Track update operations
	DB.Callback().Update().Before("gorm:update").Register("observability:before_update", func(db *gorm.DB) {
		db.InstanceSet("start_time", time.Now())
	})

	DB.Callback().Update().After("gorm:update").Register("observability:after_update", func(db *gorm.DB) {
		startTime, ok := db.InstanceGet("start_time")
		if !ok {
			return
		}

		duration := time.Since(startTime.(time.Time))
		observability.RecordDatabaseMetrics("update", duration, db.Error)
	})

	// Track delete operations
	DB.Callback().Delete().Before("gorm:delete").Register("observability:before_delete", func(db *gorm.DB) {
		db.InstanceSet("start_time", time.Now())
	})

	DB.Callback().Delete().After("gorm:delete").Register("observability:after_delete", func(db *gorm.DB) {
		startTime, ok := db.InstanceGet("start_time")
		if !ok {
			return
		}

		duration := time.Since(startTime.(time.Time))
		observability.RecordDatabaseMetrics("delete", duration, db.Error)
	})
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

// CleanupOldVersions keeps only the last 50 versions per component-locale-stage
func CleanupOldVersions() error {
	// Delete rows that are beyond the 50 most recent per (component_id, locale, stage)
	result := DB.Exec(`
		DELETE FROM translation_versions
		WHERE id IN (
			SELECT id FROM (
				SELECT id, ROW_NUMBER() OVER (PARTITION BY component_id, locale, stage ORDER BY version DESC) as rn
				FROM translation_versions
			) sub
			WHERE rn > 50
		)
	`)
	if result.Error != nil {
		return result.Error
	}
	return nil
}
