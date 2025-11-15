package observability

import (
	"os"

	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

var tracingEnabled bool

// InitTracing initializes Datadog APM tracing
// Returns nil if Datadog is disabled or if initialization fails (non-fatal)
func InitTracing() error {
	// Check if Datadog is enabled
	ddEnabled := os.Getenv("DD_ENABLED")
	if ddEnabled == "false" || ddEnabled == "0" || ddEnabled == "" {
		// Datadog disabled - service will work without tracing
		tracingEnabled = false
		return nil
	}

	serviceName := "i18n-center"
	env := os.Getenv("ENV")
	if env == "" {
		env = "development"
	}

	// Sampling rate for cost efficiency
	// 100% for errors, 10% for normal requests
	sampleRate := 0.1
	if env == "production" {
		// In production, use even lower sampling for cost efficiency
		sampleRate = 0.05 // 5% sampling
	}

	tracer.Start(
		tracer.WithService(serviceName),
		tracer.WithEnv(env),
		tracer.WithGlobalTag("version", os.Getenv("VERSION")),
		tracer.WithSampler(tracer.NewRateSampler(sampleRate)),
		tracer.WithRuntimeMetrics(), // Enable runtime metrics
		tracer.WithAnalytics(true),  // Enable analytics
	)

	tracingEnabled = true
	return nil
}

// IsTracingEnabled returns whether tracing is enabled
func IsTracingEnabled() bool {
	return tracingEnabled
}

// StopTracing stops the tracer (only if tracing was enabled)
func StopTracing() {
	if tracingEnabled {
		tracer.Stop()
	}
}

