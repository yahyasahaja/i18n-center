package observability

import (
	"os"
	"strconv"
	"time"

	"github.com/DataDog/datadog-go/v5/statsd"
)

var StatsdClient *statsd.Client

// InitMetrics initializes the Datadog StatsD client.
// No-op (StatsdClient stays nil) when DD_ENABLED is not "true"/"1" — service works without metrics.
func InitMetrics() error {
	ddEnabled := os.Getenv("DD_ENABLED")
	if ddEnabled != "true" && ddEnabled != "1" {
		return nil
	}

	host := os.Getenv("DD_AGENT_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("DD_DOGSTATSD_PORT")
	if port == "" {
		port = "8125"
	}

	client, err := statsd.New(host+":"+port,
		// No namespace — metric names are fully qualified (e.g. i18n_center_latency_seconds).
		statsd.WithTags([]string{
			"service:i18n-center",
			"env:" + getEnv(),
			"version:" + getVersion(),
		}),
		statsd.WithoutTelemetry(), // Reduce unnecessary DD agent traffic.
	)
	if err != nil {
		return err
	}

	StatsdClient = client
	return nil
}

// RecordLatency emits the i18n_center_latency_seconds histogram to Datadog.
//
// Tags:
//   - path:   Gin route template with params already masked (e.g. /api/applications/:id).
//             Pass c.FullPath() from the middleware — Gin fills this in after routing.
//   - status: HTTP status code as a string (e.g. "200", "404", "500").
//
// Guarded by two toggles (both must be satisfied to emit):
//  1. DD_ENABLED=true    → StatsdClient is non-nil after InitMetrics.
//  2. DD_METRIC_LATENCY_ENABLED != "false"/"0"  (default: enabled when DD is on).
func RecordLatency(path string, statusCode int, latency time.Duration) {
	if StatsdClient == nil || !isLatencyMetricEnabled() {
		return
	}
	StatsdClient.Histogram(
		"i18n_center_latency_seconds",
		latency.Seconds(),
		[]string{
			"path:" + path,
			"status:" + strconv.Itoa(statusCode),
		},
		1.0,
	)
}

// isLatencyMetricEnabled returns false only when DD_METRIC_LATENCY_ENABLED is explicitly
// "false" or "0". Omitting the var means enabled — so enabling DD is enough to get latency.
func isLatencyMetricEnabled() bool {
	v := os.Getenv("DD_METRIC_LATENCY_ENABLED")
	return v != "false" && v != "0"
}

// RecordServiceHealth emits a gauge (1 = up, 0 = down) for liveness monitoring.
func RecordServiceHealth(healthy bool) {
	if StatsdClient == nil {
		return
	}
	value := 0.0
	if healthy {
		value = 1.0
	}
	StatsdClient.Gauge("i18n_center_service_health", value, nil, 1.0)
}

// IncrementCounter increments a named counter. Used internally for panic tracking.
func IncrementCounter(name string, tags []string, rate float64) {
	if StatsdClient == nil {
		return
	}
	StatsdClient.Incr(name, tags, rate)
}

// RecordDatabaseMetrics is a no-op stub kept for call-site compatibility.
// Database-level metrics are not emitted — latency at the HTTP layer covers the overall picture.
func RecordDatabaseMetrics(_ string, _ time.Duration, _ error) {}

// RecordCacheMetrics is a no-op stub kept for call-site compatibility.
func RecordCacheMetrics(_ string, _ bool, _ time.Duration) {}

func getEnv() string {
	if v := os.Getenv("ENV"); v != "" {
		return v
	}
	return "development"
}

func getVersion() string {
	if v := os.Getenv("VERSION"); v != "" {
		return v
	}
	return "unknown"
}
