package observability

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRecordLatency_DisabledWhenClientNil(t *testing.T) {
	orig := StatsdClient
	StatsdClient = nil
	defer func() { StatsdClient = orig }()

	// Should not panic when client is nil
	assert.NotPanics(t, func() {
		RecordLatency("/api/test", 200, 50*time.Millisecond)
	})
}

func TestIsLatencyMetricEnabled_Default(t *testing.T) {
	os.Unsetenv("DD_METRIC_LATENCY_ENABLED")
	// When env var is missing, latency is enabled by default
	assert.True(t, isLatencyMetricEnabled())
}

func TestIsLatencyMetricEnabled_FalseWhenDisabled(t *testing.T) {
	os.Setenv("DD_METRIC_LATENCY_ENABLED", "false")
	defer os.Unsetenv("DD_METRIC_LATENCY_ENABLED")

	assert.False(t, isLatencyMetricEnabled())
}

func TestIsLatencyMetricEnabled_FalseWhenZero(t *testing.T) {
	os.Setenv("DD_METRIC_LATENCY_ENABLED", "0")
	defer os.Unsetenv("DD_METRIC_LATENCY_ENABLED")

	assert.False(t, isLatencyMetricEnabled())
}

func TestRecordServiceHealth_DisabledWhenClientNil(t *testing.T) {
	orig := StatsdClient
	StatsdClient = nil
	defer func() { StatsdClient = orig }()

	assert.NotPanics(t, func() {
		RecordServiceHealth(true)
		RecordServiceHealth(false)
	})
}

func TestIncrementCounter_DisabledWhenClientNil(t *testing.T) {
	orig := StatsdClient
	StatsdClient = nil
	defer func() { StatsdClient = orig }()

	assert.NotPanics(t, func() {
		IncrementCounter("test.counter", []string{"env:test"}, 1.0)
	})
}

func TestInitMetrics_DisabledByDefault(t *testing.T) {
	orig := StatsdClient
	StatsdClient = nil
	defer func() { StatsdClient = orig }()

	os.Unsetenv("DD_ENABLED")
	err := InitMetrics()
	assert.NoError(t, err)
	// DD_ENABLED not set → StatsdClient should remain nil
	assert.Nil(t, StatsdClient)
}

func TestInitMetrics_DisabledWhenFalse(t *testing.T) {
	orig := StatsdClient
	StatsdClient = nil
	defer func() { StatsdClient = orig }()

	os.Setenv("DD_ENABLED", "false")
	defer os.Unsetenv("DD_ENABLED")

	err := InitMetrics()
	assert.NoError(t, err)
	assert.Nil(t, StatsdClient)
}

func TestRecordDatabaseMetrics_NoopDoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		RecordDatabaseMetrics("query", 10*time.Millisecond, nil)
	})
}

func TestRecordCacheMetrics_NoopDoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		RecordCacheMetrics("get", true, 5*time.Millisecond)
	})
}

func TestGetEnvAndVersionDefaultsAndOverrides(t *testing.T) {
	os.Unsetenv("ENV")
	os.Unsetenv("VERSION")
	assert.Equal(t, "development", getEnv())
	assert.Equal(t, "unknown", getVersion())

	os.Setenv("ENV", "staging")
	os.Setenv("VERSION", "v1.2.3")
	assert.Equal(t, "staging", getEnv())
	assert.Equal(t, "v1.2.3", getVersion())
}

func TestInitMetrics_EnabledCreatesClient(t *testing.T) {
	orig := StatsdClient
	StatsdClient = nil
	defer func() {
		if StatsdClient != nil {
			_ = StatsdClient.Close()
		}
		StatsdClient = orig
	}()

	os.Setenv("DD_ENABLED", "true")
	os.Setenv("DD_AGENT_HOST", "127.0.0.1")
	os.Setenv("DD_DOGSTATSD_PORT", "8125")
	defer os.Unsetenv("DD_ENABLED")
	defer os.Unsetenv("DD_AGENT_HOST")
	defer os.Unsetenv("DD_DOGSTATSD_PORT")

	err := InitMetrics()
	assert.NoError(t, err)
	assert.NotNil(t, StatsdClient)
}
