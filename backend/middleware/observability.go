package middleware

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	"github.com/lapakgaming/i18n-center/observability"
)

// ObservabilityMiddleware logs every request and emits the i18n_center_latency_seconds metric.
//
// Path masking: c.FullPath() returns the Gin route template (e.g. /api/applications/:id),
// so UUIDs and other param values are never included in metric tags — keeping cardinality low.
func ObservabilityMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Attach a DD trace span when tracing is enabled.
		var span tracer.Span
		if observability.IsTracingEnabled() {
			path := c.Request.URL.Path
			span, _ = tracer.StartSpanFromContext(c.Request.Context(), "http.request",
				tracer.ResourceName(c.Request.Method+" "+path),
				tracer.Tag("http.method", c.Request.Method),
				tracer.Tag("http.url", path),
			)
			defer span.Finish()
		}

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()
		method := c.Request.Method

		// c.FullPath() returns the matched route template (params masked).
		// Falls back to "/unknown" for unmatched routes (404s from Gin's router).
		routePath := c.FullPath()
		if routePath == "" {
			routePath = "/unknown"
		}

		// Structured request log (always on — Datadog agent streams stdout/stderr).
		observability.LogRequest(method, routePath, statusCode, latency,
			zap.String("raw_path", c.Request.URL.Path),
			zap.String("user_id", getStringFromContext(c, "user_id")),
			zap.String("username", getStringFromContext(c, "username")),
			zap.String("ip", c.ClientIP()),
		)

		// DD metric: i18n_center_latency_seconds (gated by DD_ENABLED + DD_METRIC_LATENCY_ENABLED).
		observability.RecordLatency(routePath, statusCode, latency)

		// Annotate the trace span with outcome.
		if observability.IsTracingEnabled() && span != nil {
			span.SetTag("http.status_code", statusCode)
			span.SetTag("http.route", routePath)
			span.SetTag("http.latency_ms", latency.Milliseconds())
			if statusCode >= 500 {
				span.SetTag("error", true)
				span.SetTag("error.type", "http_server_error")
				span.SetTag("error.message", fmt.Sprintf("HTTP %d", statusCode))
			}
		}
	}
}

// PanicRecoveryMiddleware recovers from panics, logs them as errors, and returns 500.
func PanicRecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				observability.LogPanic("Panic recovered",
					zap.Any("error", rec),
					zap.String("method", c.Request.Method),
					zap.String("path", c.Request.URL.Path),
					zap.String("ip", c.ClientIP()),
					zap.String("user_id", getStringFromContext(c, "user_id")),
				)

				observability.IncrementCounter("i18n_center_service_panics", []string{
					"method:" + c.Request.Method,
					"path:" + c.FullPath(),
				}, 1.0)

				c.JSON(500, gin.H{"error": "Internal server error"})
				c.Abort()
			}
		}()

		c.Next()
	}
}

// ErrorLoggingMiddleware logs errors that handlers attach to the context via c.Error().
func ErrorLoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		for _, err := range c.Errors {
			observability.LogErrorWithContext(err.Err, "Handler error",
				map[string]interface{}{
					"method":  c.Request.Method,
					"path":    c.Request.URL.Path,
					"status":  c.Writer.Status(),
					"user_id": getStringFromContext(c, "user_id"),
					"ip":      c.ClientIP(),
				},
			)
		}
	}
}

func getStringFromContext(c *gin.Context, key string) string {
	val, exists := c.Get(key)
	if !exists {
		return ""
	}
	if s, ok := val.(string); ok {
		return s
	}
	return ""
}
