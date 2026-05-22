package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// CleanupOldVersions / setupObservabilityCallbacks were removed in Commit I
// (GORM strip). Retention is now driven by the repository layer
// (`translation.Repository.DeleteOldVersions`), and we no longer hand-wire
// callbacks since there's no GORM session to attach to.
//
// envIntOr is the only remaining pure-Go helper worth exercising.

func TestEnvIntOr(t *testing.T) {
	t.Run("unset → default", func(t *testing.T) {
		t.Setenv("DB_TEST_KEY", "")
		assert.Equal(t, 7, envIntOr("DB_TEST_KEY", 7))
	})
	t.Run("valid integer", func(t *testing.T) {
		t.Setenv("DB_TEST_KEY", "12")
		assert.Equal(t, 12, envIntOr("DB_TEST_KEY", 7))
	})
	t.Run("invalid integer → default", func(t *testing.T) {
		t.Setenv("DB_TEST_KEY", "not-a-number")
		assert.Equal(t, 7, envIntOr("DB_TEST_KEY", 7))
	})
	t.Run("zero → default", func(t *testing.T) {
		t.Setenv("DB_TEST_KEY", "0")
		assert.Equal(t, 7, envIntOr("DB_TEST_KEY", 7))
	})
}
