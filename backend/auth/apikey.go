package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/google/uuid"

	"github.com/your-org/i18n-center/database"
	"github.com/your-org/i18n-center/repository/apikey"
)

// APIKeyPrefix is the literal prefix every issued key must carry. Stored as a
// package-level const so tests can build keys without importing models/.
const APIKeyPrefix = "sk_"

// apiKeyRepo is constructed once at package load. Stateless — safe to share.
var apiKeyRepo = apikey.New()

// HashKey returns the lowercase-hex SHA-256 of a key string. Exported because
// handlers/api_key_handler.go uses the same function to derive the stored hash
// at creation time.
func HashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

// ValidateAPIKey returns the application ID if the given key is valid,
// otherwise uuid.Nil and false. The key must start with APIKeyPrefix and the
// hash must match a non-deleted row in application_api_keys.
//
// This is on the hot path for every API-key-authenticated request, so it
// reads through database.SQLX (single SELECT against idx_application_api_keys_hash).
func ValidateAPIKey(rawKey string) (uuid.UUID, bool) {
	rawKey = strings.TrimSpace(rawKey)
	if rawKey == "" || !strings.HasPrefix(rawKey, APIKeyPrefix) {
		return uuid.Nil, false
	}
	hash := HashKey(rawKey)
	k, err := apiKeyRepo.GetByHash(context.Background(), database.SQLX, hash)
	if err != nil {
		return uuid.Nil, false
	}
	return k.ApplicationID, true
}
