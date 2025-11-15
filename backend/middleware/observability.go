package middleware

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	"github.com/your-org/i18n-center/observability"
)

// ObservabilityMiddleware adds observability (logging, metrics, tracing)
func ObservabilityMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		// Add trace context (only if tracing is enabled)
		var span tracer.Span
		if observability.IsTracingEnabled() {
			var ctx context.Context
			span, ctx = tracer.StartSpanFromContext(c.Request.Context(), "http.request",
				tracer.ResourceName(method+" "+path),
				tracer.Tag("http.method", method),
				tracer.Tag("http.url", path),
			)
			defer span.Finish()
			c.Request = c.Request.WithContext(ctx)
		}

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)
		statusCode := c.Writer.Status()

		// Log request (always - logging is always enabled)
		observability.LogRequest(method, path, statusCode, latency,
			zap.String("user_id", getStringFromContext(c, "user_id")),
			zap.String("username", getStringFromContext(c, "username")),
			zap.String("ip", c.ClientIP()),
		)

		// Record metrics (only if metrics are enabled)
		observability.RecordRequestMetrics(method, path, statusCode, latency)

		// Add span tags (only if tracing is enabled)
		if observability.IsTracingEnabled() && span != nil {
			span.SetTag("http.status_code", statusCode)
			span.SetTag("http.latency_ms", latency.Milliseconds())

			// Mark errors in trace
			if statusCode >= 500 {
				span.SetTag("error", true)
				span.SetTag("error.type", "http_error")
				span.SetTag("error.message", fmt.Sprintf("HTTP %d", statusCode))
			}
		}
	}
}

// PanicRecoveryMiddleware recovers from panics and logs them
func PanicRecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Log panic with context
				observability.LogPanic("Panic recovered",
					zap.Any("error", err),
					zap.String("method", c.Request.Method),
					zap.String("path", c.Request.URL.Path),
					zap.String("ip", c.ClientIP()),
					zap.String("user_id", getStringFromContext(c, "user_id")),
				)

				// Record panic metric
				observability.IncrementCounter("service.panics", []string{
					"method:" + c.Request.Method,
					"path:" + c.Request.URL.Path,
				}, 1.0)

				// Return 500 error
				c.JSON(500, gin.H{
					"error":   "Internal server error",
					"message": "A panic occurred. Please check logs for details.",
				})
				c.Abort()
			}
		}()

		c.Next()
	}
}

// ErrorLoggingMiddleware logs errors with context
func ErrorLoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Check for errors in response
		if len(c.Errors) > 0 {
			for _, err := range c.Errors {
				observability.LogErrorWithContext(err.Err, "Request error",
					map[string]interface{}{
						"method":     c.Request.Method,
						"path":       c.Request.URL.Path,
						"status":     c.Writer.Status(),
						"error_type": fmt.Sprintf("%d", err.Type),
						"user_id":    getStringFromContext(c, "user_id"),
						"ip":         c.ClientIP(),
					},
				)
			}
		}
	}
}

// Helper function to get string from context
func getStringFromContext(c *gin.Context, key string) string {
	val, exists := c.Get(key)
	if !exists {
		return ""
	}
	if str, ok := val.(string); ok {
		return str
	}
	return ""
}
