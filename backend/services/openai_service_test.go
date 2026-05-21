package services

import (
	"context"
	"testing"
	"time"

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

func TestOpenAIService_MockMode(t *testing.T) {
	t.Setenv("OPENAI_MOCK", "")
	s := NewOpenAIService("mock")
	assert.True(t, s.isMockMode())

	s = NewOpenAIService("mock:anything")
	assert.True(t, s.isMockMode())

	s = NewOpenAIService("real-key")
	assert.False(t, s.isMockMode())

	t.Setenv("OPENAI_MOCK", "true")
	assert.True(t, s.isMockMode())
}

func TestMockTranslateText(t *testing.T) {
	assert.Equal(t, "https://example.com", mockTranslateText("https://example.com", "id"))
	assert.Equal(t, "user@example.com", mockTranslateText("user@example.com", "id"))
	assert.Equal(t, "[name]", mockTranslateText("[name]", "id"))
	assert.Equal(t, "Hello [id-mock]", mockTranslateText("Hello", "id"))
}

func TestMockTranslateJSON(t *testing.T) {
	src := map[string]interface{}{
		"title": "Hello",
		"meta": map[string]interface{}{
			"subtitle": "World",
			"url":      "https://example.com",
		},
		"count": 12,
	}
	got := mockTranslateJSON(src, "fr")
	assert.Equal(t, "Hello [fr-mock]", got["title"])
	meta := got["meta"].(map[string]interface{})
	assert.Equal(t, "World [fr-mock]", meta["subtitle"])
	assert.Equal(t, "https://example.com", meta["url"])
	assert.Equal(t, 12, got["count"])
}

func TestTranslateJSONBatch_MockMode(t *testing.T) {
	s := NewOpenAIService("mock")
	got, err := s.TranslateJSONBatch(context.Background(), map[string]interface{}{"hello": "Hello"}, nil, "en", "es")
	assert.NoError(t, err)
	assert.Equal(t, "Hello [es-mock]", got["hello"])
}

func TestTranslate_MockMode(t *testing.T) {
	s := NewOpenAIService("mock")
	got, err := s.Translate(context.Background(), "Hello", "", "en", "id")
	assert.NoError(t, err)
	assert.Equal(t, "Hello [id-mock]", got)
}

func TestBuildKeyHintsSection(t *testing.T) {
	data := map[string]interface{}{
		"greeting": map[string]interface{}{
			"welcome": "Hi",
		},
		"checkout": map[string]interface{}{
			"cta": "Pay now",
		},
		"meta": map[string]interface{}{
			"count": 5,
		},
	}
	contexts := map[string]string{
		"greeting.welcome": "user greeting",
		"checkout.cta":     "payment confirmation button",
		"unknown.path":     "irrelevant — should be dropped",
		"meta.count":       "non-string leaf — should be dropped",
		"empty.note":       "   ",
	}
	got := buildKeyHintsSection(data, contexts)
	assert.Contains(t, got, "KEY HINTS")
	assert.Contains(t, got, "checkout.cta: payment confirmation button")
	assert.Contains(t, got, "greeting.welcome: user greeting")
	assert.NotContains(t, got, "unknown.path")
	assert.NotContains(t, got, "meta.count")
	assert.NotContains(t, got, "empty.note")

	// No contexts at all → no section
	assert.Equal(t, "", buildKeyHintsSection(data, nil))
}

func TestValidateTranslatedJSON(t *testing.T) {
	source := map[string]interface{}{
		"hello": "Hello [name]",
		"meta": map[string]interface{}{
			"title": "Title",
		},
	}
	valid := map[string]interface{}{
		"hello": "Halo [name]",
		"meta": map[string]interface{}{
			"title": "Judul",
		},
	}
	assert.NoError(t, validateTranslatedJSON(source, valid))

	missingKey := map[string]interface{}{"hello": "Halo [name]"}
	assert.Error(t, validateTranslatedJSON(source, missingKey))

	missingPlaceholder := map[string]interface{}{
		"hello": "Halo",
		"meta":  map[string]interface{}{"title": "Judul"},
	}
	assert.Error(t, validateTranslatedJSON(source, missingPlaceholder))
}

func TestOpenAIRetryDelay(t *testing.T) {
	d, retry := openAIRetryDelay(assert.AnError, 1)
	assert.True(t, retry)
	assert.Equal(t, 2*time.Second, d)

	d, retry = openAIRetryDelay(assert.AnError, 2)
	assert.True(t, retry)
	assert.Equal(t, 4*time.Second, d)

	d, retry = openAIRetryDelay(assert.AnError, 3)
	assert.True(t, retry)
	assert.Equal(t, 8*time.Second, d)

	d, retry = openAIRetryDelay(assert.AnError, 1)
	assert.True(t, retry)
	assert.NotZero(t, d)

	d, retry = openAIRetryDelay(&customErr{msg: "OpenAI API error 401: bad key"}, 1)
	assert.False(t, retry)
	assert.Equal(t, time.Duration(0), d)
}

type customErr struct{ msg string }

func (e *customErr) Error() string { return e.msg }

// Note: Actual translation tests would require mocking HTTP calls
// For unit tests, we focus on the service initialization and structure
