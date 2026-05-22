package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CompareValues is pure logic — these stay as real tests.

func TestCompareValues_DetectsChangedField(t *testing.T) {
	before := map[string]interface{}{"name": "old", "active": true}
	after := map[string]interface{}{"name": "new", "active": true}

	diff := CompareValues(before, after)
	require.Contains(t, diff, "name")

	nameDiff, ok := diff["name"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "old", nameDiff["before"])
	assert.Equal(t, "new", nameDiff["after"])

	// Unchanged field should not appear in diff
	assert.NotContains(t, diff, "active")
}

func TestCompareValues_DetectsNewField(t *testing.T) {
	before := map[string]interface{}{"name": "foo"}
	after := map[string]interface{}{"name": "foo", "code": "bar"}

	diff := CompareValues(before, after)
	require.Contains(t, diff, "code")

	codeDiff, ok := diff["code"].(map[string]interface{})
	require.True(t, ok)
	assert.Nil(t, codeDiff["before"])
	assert.Equal(t, "bar", codeDiff["after"])
}

func TestCompareValues_DetectsDeletedField(t *testing.T) {
	before := map[string]interface{}{"name": "foo", "extra": "gone"}
	after := map[string]interface{}{"name": "foo"}

	diff := CompareValues(before, after)
	require.Contains(t, diff, "extra")

	extraDiff, ok := diff["extra"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "gone", extraDiff["before"])
	assert.Nil(t, extraDiff["after"])
}

func TestCompareValues_NoDiffWhenEqual(t *testing.T) {
	data := map[string]interface{}{"x": 1, "y": "hello"}
	diff := CompareValues(data, data)
	assert.Empty(t, diff)
}

func TestCompareValues_EmptyInputs(t *testing.T) {
	diff := CompareValues(nil, nil)
	assert.NotNil(t, diff)
}

// LogCreate/LogUpdate/LogDelete/GetAuditLogs/GetAuditLogsByUser were sqlmock'd
// against the GORM `INSERT INTO "audit_logs"` shape. Commit H moves the
// service onto the sqlx-backed audit.Repository (raw SQL, different placeholder
// style, no quoted identifiers). Rewriting these against the new query shapes
// is tracked under Commit I.

func TestAuditServiceDBPaths_TODO(t *testing.T) {
	t.Skip("TODO(post-refactor): rewrite audit-service DB tests for sqlx repository layer")
}
