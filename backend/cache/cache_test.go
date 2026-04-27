package cache

import (
	"context"
	"os"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestInitCache(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		s, err := miniredis.Run()
		require.NoError(t, err)
		defer s.Close()

		host, port, err := netSplitHostPort(s.Addr())
		require.NoError(t, err)
		t.Setenv("REDIS_HOST", host)
		t.Setenv("REDIS_PORT", port)
		t.Setenv("REDIS_PASSWORD", "")
		t.Setenv("REDIS_DB", "0")

		require.NoError(t, InitCache())
		require.NotNil(t, Client)
	})

	t.Run("failure", func(t *testing.T) {
		t.Setenv("REDIS_HOST", "127.0.0.1")
		t.Setenv("REDIS_PORT", "1")
		t.Setenv("REDIS_PASSWORD", "")
		err := InitCache()
		assert.Error(t, err)
	})
}

func TestCacheCRUDAndPattern(t *testing.T) {
	s, err := miniredis.Run()
	require.NoError(t, err)
	defer s.Close()

	Client = redis.NewClient(&redis.Options{Addr: s.Addr()})

	type payload struct {
		Name string `json:"name"`
	}

	require.NoError(t, Set("k1", payload{Name: "alpha"}, time.Minute))

	var got payload
	require.NoError(t, Get("k1", &got))
	assert.Equal(t, "alpha", got.Name)

	require.NoError(t, Delete("k1"))
	assert.Error(t, Get("k1", &got))

	require.NoError(t, Set("p:1", payload{Name: "one"}, time.Minute))
	require.NoError(t, Set("p:2", payload{Name: "two"}, time.Minute))
	require.NoError(t, DeletePattern("p:*"))

	keys := s.Keys()
	assert.NotContains(t, keys, "p:1")
	assert.NotContains(t, keys, "p:2")
}

func TestGet_InvalidJSON(t *testing.T) {
	s, err := miniredis.Run()
	require.NoError(t, err)
	defer s.Close()

	Client = redis.NewClient(&redis.Options{Addr: s.Addr()})
	require.NoError(t, Client.Set(context.Background(), "bad-json", "{", time.Minute).Err())

	var got map[string]interface{}
	assert.Error(t, Get("bad-json", &got))
}

func TestExtraCacheKeys(t *testing.T) {
	assert.Equal(t, "translations:bytag:app:tag:id:draft", TranslationsByTagKey("app", "tag", "id", "draft"))
	assert.Equal(t, "translations:bypage:app:home:id:production", TranslationsByPageKey("app", "home", "id", "production"))
}

func netSplitHostPort(addr string) (string, string, error) {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i], addr[i+1:], nil
		}
	}
	return "", "", assert.AnError
}
