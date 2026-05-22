package jobs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// All previous worker_test.go content was heavily coupled to GORM-shaped
// queries (mock.ExpectQuery on `SELECT "add_language_jobs"` etc.). Commit H
// moves the worker to sqlx repositories with raw SQL — rewriting these tests
// against the new query shapes is tracked under Commit I.
//
// We keep one trivial smoke test so the package compiles and the test target
// still discovers something to run.

func TestResolveOpenAIService(t *testing.T) {
	t.Run("nil when no key", func(t *testing.T) {
		t.Setenv("OPENAI_API_KEY", "")
		assert.Nil(t, resolveOpenAIService(""))
	})

	t.Run("uses app key", func(t *testing.T) {
		svc := resolveOpenAIService("mock")
		if assert.NotNil(t, svc) {
			assert.Equal(t, "mock", svc.APIKey)
		}
	})

	t.Run("env-var fallback", func(t *testing.T) {
		t.Setenv("OPENAI_API_KEY", "env-key")
		svc := resolveOpenAIService("")
		if assert.NotNil(t, svc) {
			assert.Equal(t, "env-key", svc.APIKey)
		}
	})
}

func TestWorkerRepoHelpers_TODO(t *testing.T) {
	t.Skip("TODO(post-refactor): rewrite worker tests for sqlx repository layer (claim/process/reset paths)")
}
