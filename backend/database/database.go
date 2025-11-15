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

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})

	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Handle migration for code fields (backfill existing data)
	if err := migrateCodeFields(); err != nil {
		return fmt.Errorf("failed to migrate code fields: %w", err)
	}

	// Auto-migrate tables
	err = DB.AutoMigrate(
		&models.User{},
		&models.Application{},
		&models.Component{},
		&models.TranslationVersion{},
		&models.AuditLog{},
	)

	if err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
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

// CleanupOldVersions keeps only 2 versions per component-locale-stage combination
func CleanupOldVersions() error {
	// Delete versions with version > 2
	result := DB.Where("version > ?", 2).Delete(&models.TranslationVersion{})
	if result.Error != nil {
		return result.Error
	}
	return nil
}
