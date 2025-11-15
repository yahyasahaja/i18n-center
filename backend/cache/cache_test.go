package cache

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCacheKeyGenerators(t *testing.T) {
	tests := []struct {
		name     string
		function func() string
		expected string
	}{
		{
			name:     "ComponentKey",
			function: func() string { return ComponentKey("test-id") },
			expected: "component:test-id",
		},
		{
			name:     "TranslationKey",
			function: func() string { return TranslationKey("comp-id", "en", "production") },
			expected: "translation:comp-id:en:production",
		},
		{
			name:     "ApplicationKey",
			function: func() string { return ApplicationKey("app-id") },
			expected: "application:app-id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.function()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetRedisDB(t *testing.T) {
	// Test default
	os.Unsetenv("REDIS_DB")
	db := getRedisDB()
	assert.Equal(t, 0, db)

	// Test with value
	os.Setenv("REDIS_DB", "5")
	db = getRedisDB()
	assert.Equal(t, 5, db)
}
