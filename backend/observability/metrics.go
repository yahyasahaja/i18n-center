package observability

import (
	"fmt"
	"os"
	"time"

	"github.com/DataDog/datadog-go/v5/statsd"
)

var StatsdClient *statsd.Client

// InitMetrics initializes Datadog statsd client
// Returns nil if Datadog is disabled or if initialization fails (non-fatal)
func InitMetrics() error {
	// Check if Datadog is enabled
	ddEnabled := os.Getenv("DD_ENABLED")
	if ddEnabled == "false" || ddEnabled == "0" || ddEnabled == "" {
		// Datadog disabled - service will work without metrics
		return nil
	}

	ddAgentHost := os.Getenv("DD_AGENT_HOST")
	if ddAgentHost == "" {
		ddAgentHost = "localhost"
	}

	ddAgentPort := os.Getenv("DD_DOGSTATSD_PORT")
	if ddAgentPort == "" {
		ddAgentPort = "8125"
	}

	client, err := statsd.New(ddAgentHost + ":" + ddAgentPort,
		statsd.WithNamespace("i18n_center"),
		statsd.WithTags([]string{
			"service:i18n-center",
			"env:" + getEnv(),
			"version:" + getVersion(),
		}),
		statsd.WithoutTelemetry(), // Disable telemetry to reduce cost
	)
	if err != nil {
		// Non-fatal: service can work without metrics
		return err
	}

	StatsdClient = client
	return nil
}

// IncrementCounter increments a counter metric
func IncrementCounter(name string, tags []string, rate float64) {
	if StatsdClient == nil {
		return
	}
	StatsdClient.Incr(name, tags, rate)
}

// RecordHistogram records a histogram metric (for latency, sizes, etc.)
func RecordHistogram(name string, value float64, tags []string, rate float64) {
	if StatsdClient == nil {
		return
	}
	StatsdClient.Histogram(name, value, tags, rate)
}

// RecordGauge records a gauge metric (for current values)
func RecordGauge(name string, value float64, tags []string, rate float64) {
	if StatsdClient == nil {
		return
	}
	StatsdClient.Gauge(name, value, tags, rate)
}

// RecordTiming records a timing metric
func RecordTiming(name string, duration time.Duration, tags []string, rate float64) {
	if StatsdClient == nil {
		return
	}
	StatsdClient.Timing(name, duration, tags, rate)
}

// RecordRequestMetrics records HTTP request metrics
func RecordRequestMetrics(method, path string, statusCode int, latency time.Duration) {
	if StatsdClient == nil {
		return
	}

	// Sample rate: 100% for errors, 10% for success (cost optimization)
	rate := 1.0
	if statusCode < 400 {
		rate = 0.1 // 10% sampling for successful requests
	}

	tags := []string{
		"method:" + method,
		"path:" + normalizePath(path),
		"status:" + statusCodeToString(statusCode),
		"status_class:" + getStatusClass(statusCode),
	}

	// Request count
	IncrementCounter("http.requests", tags, rate)

	// Request latency
	RecordTiming("http.request.duration", latency, tags, rate)

	// Status code specific metrics
	if statusCode >= 500 {
		IncrementCounter("http.errors.server", tags, 1.0) // Always track errors
	} else if statusCode >= 400 {
		IncrementCounter("http.errors.client", tags, 1.0) // Always track client errors
	}
}

// RecordDatabaseMetrics records database operation metrics
func RecordDatabaseMetrics(operation string, duration time.Duration, err error) {
	if StatsdClient == nil {
		return
	}

	tags := []string{"operation:" + operation}
	if err != nil {
		tags = append(tags, "error:true")
		IncrementCounter("db.errors", tags, 1.0)
	} else {
		tags = append(tags, "error:false")
	}

	// Always track DB operations (important for monitoring)
	IncrementCounter("db.operations", tags, 1.0)
	RecordTiming("db.duration", duration, tags, 1.0)
}

// RecordCacheMetrics records cache operation metrics
func RecordCacheMetrics(operation string, hit bool, duration time.Duration) {
	if StatsdClient == nil {
		return
	}

	tags := []string{
		"operation:" + operation,
		"hit:" + boolToString(hit),
	}

	IncrementCounter("cache.operations", tags, 0.1) // 10% sampling for cache ops
	RecordTiming("cache.duration", duration, tags, 0.1)
}

// RecordServiceHealth records service health metrics
func RecordServiceHealth(healthy bool) {
	if StatsdClient == nil {
		return
	}

	value := 0.0
	if healthy {
		value = 1.0
	}

	RecordGauge("service.health", value, []string{}, 1.0)
}

// Helper functions
func getEnv() string {
	env := os.Getenv("ENV")
	if env == "" {
		return "development"
	}
	return env
}

func getVersion() string {
	version := os.Getenv("VERSION")
	if version == "" {
		return "unknown"
	}
	return version
}

func normalizePath(path string) string {
	// Normalize paths to reduce cardinality (e.g., /api/applications/:id -> /api/applications/:id)
	// This prevents high cardinality from UUIDs in paths
	// For now, return as-is, but could implement path normalization
	return path
}

func statusCodeToString(code int) string {
	// Return as string for better grouping in Datadog
	return fmt.Sprintf("%d", code)
}

func getStatusClass(code int) string {
	switch {
	case code >= 500:
		return "5xx"
	case code >= 400:
		return "4xx"
	case code >= 300:
		return "3xx"
	case code >= 200:
		return "2xx"
	default:
		return "1xx"
	}
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

