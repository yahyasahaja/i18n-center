package observability

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitTracing_Disabled(t *testing.T) {
	tracingEnabled = true
	os.Setenv("DD_ENABLED", "false")
	defer os.Unsetenv("DD_ENABLED")

	err := InitTracing()
	assert.NoError(t, err)
	assert.False(t, IsTracingEnabled())
}

func TestInitTracing_Enabled(t *testing.T) {
	os.Setenv("DD_ENABLED", "true")
	os.Setenv("ENV", "development")
	os.Setenv("VERSION", "test")
	defer os.Unsetenv("DD_ENABLED")
	defer os.Unsetenv("ENV")
	defer os.Unsetenv("VERSION")

	err := InitTracing()
	assert.NoError(t, err)
	assert.True(t, IsTracingEnabled())
	StopTracing()
	assert.NotPanics(t, func() { StopTracing() })
}
