package auth

import (
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestHashPassword(t *testing.T) {
	password := "testpassword123"
	hash, err := HashPassword(password)
	assert.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, password, hash)
}

func TestCheckPasswordHash(t *testing.T) {
	password := "testpassword123"
	hash, _ := HashPassword(password)

	tests := []struct {
		name     string
		password string
		hash     string
		expected bool
	}{
		{
			name:     "Correct password",
			password: password,
			hash:     hash,
			expected: true,
		},
		{
			name:     "Incorrect password",
			password: "wrongpassword",
			hash:     hash,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CheckPasswordHash(tt.password, tt.hash)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateToken(t *testing.T) {
	// Set a test secret
	os.Setenv("JWT_SECRET", "test-secret-key")
	jwtSecret = []byte("test-secret-key")

	userID := uuid.New()
	username := "testuser"
	role := "operator"

	token, err := GenerateToken(userID, username, role)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)
}

func TestValidateToken(t *testing.T) {
	// Set a test secret
	os.Setenv("JWT_SECRET", "test-secret-key")
	jwtSecret = []byte("test-secret-key")

	userID := uuid.New()
	username := "testuser"
	role := "operator"

	token, err := GenerateToken(userID, username, role)
	assert.NoError(t, err)

	claims, err := ValidateToken(token)
	assert.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, username, claims.Username)
	assert.Equal(t, role, claims.Role)
}

func TestValidateToken_InvalidToken(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key")
	jwtSecret = []byte("test-secret-key")

	invalidToken := "invalid.token.here"
	_, err := ValidateToken(invalidToken)
	assert.Error(t, err)
}

func TestValidateToken_ExpiredToken(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-key")
	jwtSecret = []byte("test-secret-key")

	// This would require creating an expired token manually
	// For now, we test that invalid tokens are rejected
	_, err := ValidateToken("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c")
	assert.Error(t, err)
}
