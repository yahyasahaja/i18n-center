package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewOpenAIService(t *testing.T) {
	service := NewOpenAIService("test-key")
	assert.NotNil(t, service)
	assert.Equal(t, "test-key", service.APIKey)
}

func TestGetDefaultOpenAIKey(t *testing.T) {
	// This test just ensures the function doesn't panic
	key := GetDefaultOpenAIKey()
	// Key might be empty if env not set, that's okay
	_ = key
}

// Note: Actual translation tests would require mocking HTTP calls
// For unit tests, we focus on the service initialization and structure

