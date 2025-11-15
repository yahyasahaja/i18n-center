package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/your-org/i18n-center/observability"
)

var (
	Client *redis.Client
	ctx    = context.Background()
)

// InitCache initializes Redis connection
func InitCache() error {
	Client = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", os.Getenv("REDIS_HOST"), os.Getenv("REDIS_PORT")),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       getRedisDB(),
	})

	// Test connection
	_, err := Client.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return nil
}

func getRedisDB() int {
	db := 0
	if os.Getenv("REDIS_DB") != "" {
		fmt.Sscanf(os.Getenv("REDIS_DB"), "%d", &db)
	}
	return db
}

// Get retrieves a value from cache
func Get(key string, dest interface{}) error {
	start := time.Now()
	val, err := Client.Get(ctx, key).Result()
	duration := time.Since(start)

	hit := err == nil
	observability.RecordCacheMetrics("get", hit, duration)

	if err == redis.Nil {
		return fmt.Errorf("key not found")
	}
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(val), dest)
}

// Set stores a value in cache with expiration
func Set(key string, value interface{}, expiration time.Duration) error {
	start := time.Now()
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	err = Client.Set(ctx, key, data, expiration).Err()
	duration := time.Since(start)

	observability.RecordCacheMetrics("set", err == nil, duration)
	return err
}

// Delete removes a key from cache
func Delete(key string) error {
	start := time.Now()
	err := Client.Del(ctx, key).Err()
	duration := time.Since(start)

	observability.RecordCacheMetrics("delete", err == nil, duration)
	return err
}

// DeletePattern deletes all keys matching a pattern
func DeletePattern(pattern string) error {
	iter := Client.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		if err := Client.Del(ctx, iter.Val()).Err(); err != nil {
			return err
		}
	}
	return iter.Err()
}

// Cache key generators
func ComponentKey(componentID string) string {
	return fmt.Sprintf("component:%s", componentID)
}

func TranslationKey(componentID, locale, stage string) string {
	return fmt.Sprintf("translation:%s:%s:%s", componentID, locale, stage)
}

func ApplicationKey(applicationID string) string {
	return fmt.Sprintf("application:%s", applicationID)
}

