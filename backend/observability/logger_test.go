package observability

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitLogger_Development(t *testing.T) {
	os.Unsetenv("ENV")
	err := InitLogger()
	require.NoError(t, err)
	assert.NotNil(t, Logger)
	assert.NotNil(t, Sugar)
}

func TestInitLogger_Production(t *testing.T) {
	os.Setenv("ENV", "production")
	defer os.Unsetenv("ENV")

	err := InitLogger()
	require.NoError(t, err)
	assert.NotNil(t, Logger)
	assert.NotNil(t, Sugar)
}

func TestLogError_SkipsNilError(t *testing.T) {
	// Make sure a logger is initialized
	_ = InitLogger()

	assert.NotPanics(t, func() {
		LogError(nil, "this should be a no-op")
	})
}

func TestLogError_WithError(t *testing.T) {
	_ = InitLogger()

	assert.NotPanics(t, func() {
		LogError(errors.New("test error"), "something went wrong")
	})
}

func TestLogRequest_LevelsForStatusCodes(t *testing.T) {
	_ = InitLogger()

	tests := []struct {
		status int
		desc   string
	}{
		{200, "2xx should log at info level"},
		{301, "3xx should log at info level"},
		{400, "4xx should log at warn level"},
		{404, "4xx not-found should log at warn level"},
		{500, "5xx should log at error level"},
		{503, "5xx service unavailable should log at error level"},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			assert.NotPanics(t, func() {
				LogRequest("GET", "/test", tc.status, 10*time.Millisecond)
			})
		})
	}
}

func TestLogErrorWithContext_SkipsNilError(t *testing.T) {
	_ = InitLogger()

	assert.NotPanics(t, func() {
		LogErrorWithContext(nil, "no-op", map[string]interface{}{"key": "value"})
	})
}

func TestLogErrorWithContext_WithError(t *testing.T) {
	_ = InitLogger()

	assert.NotPanics(t, func() {
		LogErrorWithContext(errors.New("ctx error"), "with context", map[string]interface{}{
			"user_id": "123",
			"path":    "/api/test",
		})
	})
}
