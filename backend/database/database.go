package database

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/your-org/i18n-center/models"
	"github.com/your-org/i18n-center/observability"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// InitDatabase initializes the database connection
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

	// Handle migration for code fields (backfill existing data)
	if err := migrateCodeFields(); err != nil {
		return fmt.Errorf("failed to migrate code fields: %w", err)
	}

	// Drop name column from tags and pages (identifiers are code-only now)
	if err := dropTagPageNameColumns(); err != nil {
		return fmt.Errorf("failed to drop tag/page name columns: %w", err)
	}

	// Auto-migrate tables
	err = DB.AutoMigrate(
		&models.User{},
		&models.Application{},
		&models.ApplicationAPIKey{},
		&models.Tag{},
		&models.Page{},
		&models.Component{},
		&models.TranslationVersion{},
		&models.AuditLog{},
		&models.ApplicationLocaleDeploy{},
		&models.AddLanguageJob{},
		&models.TranslateJob{},
		// CMS models
		&models.CmsTemplate{},
		&models.CmsTemplateField{},
		&models.CmsItem{},
		&models.CmsLocalization{},
		&models.CmsTranslateJob{},
	)

	if err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	// Enable pg_trgm for efficient ILIKE text search on components
	if err := ensureSearchIndexes(); err != nil {
		log.Printf("Note: search indexes setup error (non-fatal): %v", err)
	}

	// Indexes that protect hot read paths and prevent version-number races.
	// Non-fatal: server still starts if these fail (e.g. on a slave at boot),
	// but performance and write-concurrency safety will be degraded.
	if err := ensurePerformanceIndexes(); err != nil {
		log.Printf("Note: performance indexes setup error (non-fatal): %v", err)
	}

	// Add observability callbacks
	setupObservabilityCallbacks()

	log.Println("Database connected and migrated successfully")
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

// migrateCodeFields handles migration of code fields for existing data
func migrateCodeFields() error {
	// Check if applications table has code column
	var hasCodeColumn bool
	err := DB.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'applications' AND column_name = 'code'
		)
	`).Scan(&hasCodeColumn).Error

	if err != nil {
		return fmt.Errorf("failed to check code column: %w", err)
	}

	// If code column doesn't exist, add it as nullable first
	if !hasCodeColumn {
		// Add code column as nullable
		if err := DB.Exec("ALTER TABLE applications ADD COLUMN code text").Error; err != nil {
			// Column might already exist, ignore error
			log.Printf("Note: applications.code column may already exist: %v", err)
		}
	}

	// Backfill code for existing applications (use name as base, make it URL-safe)
	// This handles both new columns and existing nullable columns
	if err := DB.Exec(`
		UPDATE applications
		SET code = LOWER(REGEXP_REPLACE(name, '[^a-zA-Z0-9]+', '_', 'g'))
		WHERE code IS NULL OR code = ''
	`).Error; err != nil {
		return fmt.Errorf("failed to backfill application codes: %w", err)
	}

	// Make code NOT NULL (safe now that all rows have values)
	// Check if column is already NOT NULL to avoid errors
	var isNotNull bool
	err = DB.Raw(`
		SELECT is_nullable = 'NO'
		FROM information_schema.columns
		WHERE table_name = 'applications' AND column_name = 'code'
	`).Scan(&isNotNull).Error

	if err == nil && !isNotNull {
		if err := DB.Exec("ALTER TABLE applications ALTER COLUMN code SET NOT NULL").Error; err != nil {
			return fmt.Errorf("failed to set code as NOT NULL: %w", err)
		}
	}

	// Check if components table has code column
	var hasComponentCodeColumn bool
	err = DB.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'components' AND column_name = 'code'
		)
	`).Scan(&hasComponentCodeColumn).Error

	if err != nil {
		return fmt.Errorf("failed to check component code column: %w", err)
	}

	// If code column doesn't exist, add it as nullable first
	if !hasComponentCodeColumn {
		// Add code column as nullable
		if err := DB.Exec("ALTER TABLE components ADD COLUMN code text").Error; err != nil {
			// Column might already exist, ignore error
			log.Printf("Note: components.code column may already exist: %v", err)
		}
	}

	// Backfill code for existing components
	// This handles both new columns and existing nullable columns
	if err := DB.Exec(`
		UPDATE components
		SET code = LOWER(REGEXP_REPLACE(name, '[^a-zA-Z0-9]+', '_', 'g'))
		WHERE code IS NULL OR code = ''
	`).Error; err != nil {
		return fmt.Errorf("failed to backfill component codes: %w", err)
	}

	// Make code NOT NULL (safe now that all rows have values)
	// Check if column is already NOT NULL to avoid errors
	var isComponentNotNull bool
	err = DB.Raw(`
		SELECT is_nullable = 'NO'
		FROM information_schema.columns
		WHERE table_name = 'components' AND column_name = 'code'
	`).Scan(&isComponentNotNull).Error

	if err == nil && !isComponentNotNull {
		if err := DB.Exec("ALTER TABLE components ALTER COLUMN code SET NOT NULL").Error; err != nil {
			return fmt.Errorf("failed to set component code as NOT NULL: %w", err)
		}
	}

	// Update unique constraint: change from single column to composite (application_id, code)
	// Check if the old unique index exists and drop it
	var oldIndexExists bool
	err = DB.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM pg_indexes
			WHERE tablename = 'components'
			AND indexname = 'components_code_key'
		)
	`).Scan(&oldIndexExists).Error

	if err == nil && oldIndexExists {
		// Drop the old single-column unique index
		if err := DB.Exec("DROP INDEX IF EXISTS components_code_key").Error; err != nil {
			log.Printf("Note: Could not drop old unique index (may not exist): %v", err)
		}
	}

	// Check if composite unique index exists
	var compositeIndexExists bool
	err = DB.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM pg_indexes
			WHERE tablename = 'components'
			AND indexname = 'idx_component_app_code'
		)
	`).Scan(&compositeIndexExists).Error

	if err == nil && !compositeIndexExists {
		// Create composite unique index
		if err := DB.Exec("CREATE UNIQUE INDEX idx_component_app_code ON components(application_id, code) WHERE deleted_at IS NULL").Error; err != nil {
			// If it fails, try without WHERE clause (for older PostgreSQL or if soft delete isn't used)
			if err2 := DB.Exec("CREATE UNIQUE INDEX idx_component_app_code ON components(application_id, code)").Error; err2 != nil {
				return fmt.Errorf("failed to create composite unique index: %w (original: %v)", err2, err)
			}
		}
	}

	return nil
}

// dropTagPageNameColumns drops the name column from tags and pages tables if present.
// Tag and Page are now identified by code only.
func dropTagPageNameColumns() error {
	for _, table := range []string{"tags", "pages"} {
		var hasName bool
		err := DB.Raw(`
			SELECT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_schema = 'public' AND table_name = $1 AND column_name = 'name'
			)
		`, table).Scan(&hasName).Error
		if err != nil {
			return fmt.Errorf("failed to check %s.name column: %w", table, err)
		}
		if hasName {
			if err := DB.Exec(fmt.Sprintf("ALTER TABLE %s DROP COLUMN IF EXISTS name", table)).Error; err != nil {
				return fmt.Errorf("failed to drop %s.name: %w", table, err)
			}
			log.Printf("Dropped column name from table %s", table)
		}
	}
	return nil
}

// ensureSearchIndexes creates GIN trigram indexes on components for fast ILIKE search.
func ensureSearchIndexes() error {
	// Enable pg_trgm extension (needed for GIN trigram index)
	if err := DB.Exec("CREATE EXTENSION IF NOT EXISTS pg_trgm").Error; err != nil {
		return fmt.Errorf("pg_trgm extension: %w", err)
	}
	// GIN index on components.name for trigram ILIKE
	if err := DB.Exec(`
		CREATE INDEX IF NOT EXISTS idx_components_name_trgm
		ON components USING GIN (name gin_trgm_ops)
	`).Error; err != nil {
		return fmt.Errorf("name trigram index: %w", err)
	}
	// GIN index on components.code for trigram ILIKE
	if err := DB.Exec(`
		CREATE INDEX IF NOT EXISTS idx_components_code_trgm
		ON components USING GIN (code gin_trgm_ops)
	`).Error; err != nil {
		return fmt.Errorf("code trigram index: %w", err)
	}
	return nil
}

// ensurePerformanceIndexes creates indexes that protect the two highest-stakes
// access patterns on translation_versions:
//
//  1. The hot read query: latest active version for a (component_id, locale, stage).
//     Public translation endpoints hit this path on every cache miss; without a
//     composite index that includes version DESC, the planner falls back to the
//     component_id b-tree and re-sorts in memory — fine at low row counts,
//     painful past a few hundred thousand rows.
//
//  2. The version-number race on concurrent saves. saveVersion reads MAX(version)
//     and then inserts version+1; without a unique constraint two concurrent
//     writers can both pick the same number and silently create duplicate rows.
//     A partial unique index turns that into a duplicate-key error the service
//     can retry deterministically.
func ensurePerformanceIndexes() error {
	// Composite read index for the hot path.
	// Partial on deleted_at IS NULL so soft-deleted rows don't bloat the index.
	if err := DB.Exec(`
		CREATE INDEX IF NOT EXISTS idx_tv_lookup
		ON translation_versions (component_id, locale, stage, version DESC)
		WHERE deleted_at IS NULL
	`).Error; err != nil {
		return fmt.Errorf("translation_versions lookup index: %w", err)
	}

	// Partial unique index to eliminate the version-number race.
	// Only active, non-deleted rows participate — historical soft-deleted rows
	// may share a version number with newer rows from the same logical chain.
	if err := DB.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_tv_unique_version
		ON translation_versions (component_id, locale, stage, version)
		WHERE deleted_at IS NULL
	`).Error; err != nil {
		return fmt.Errorf("translation_versions unique version index: %w", err)
	}

	// Idempotency for translate-job creation: at most one pending+running job
	// per (component, source, first target, type) tuple. Stops double-clicks
	// from queuing duplicate OpenAI work that races each other on save.
	// target_locales is a Postgres text[]; index on the first element so the
	// auto-translate case (single target) is constrained while backfill
	// (multi-target) is unaffected.
	if err := DB.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_translate_jobs_dedupe
		ON translate_jobs (component_id, source_locale, (target_locales[1]), job_type)
		WHERE deleted_at IS NULL AND status IN ('pending', 'running')
	`).Error; err != nil {
		return fmt.Errorf("translate_jobs dedupe index: %w", err)
	}

	return nil
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
