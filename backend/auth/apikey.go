package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/google/uuid"
	"github.com/your-org/i18n-center/database"
	"github.com/your-org/i18n-center/models"
)

func hashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

// ValidateAPIKey returns the application ID if the given key is valid, otherwise uuid.Nil and false.
// The key must start with the configured prefix (e.g. sk_).
func ValidateAPIKey(rawKey string) (uuid.UUID, bool) {
	rawKey = strings.TrimSpace(rawKey)
	if rawKey == "" || !strings.HasPrefix(rawKey, models.APIKeyPrefix) {
		return uuid.Nil, false
	}
	hash := hashKey(rawKey)
	var key models.ApplicationAPIKey
	if err := database.DB.Where("key_hash = ?", hash).First(&key).Error; err != nil {
		return uuid.Nil, false
	}
	return key.ApplicationID, true
}
