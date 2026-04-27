package observability

import (
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.Logger
var Sugar *zap.SugaredLogger

// InitLogger initializes structured logging.
// Development: human-readable colored output.
// Production (ENV=production): JSON lines — streamed and picked up by the Datadog agent.
func InitLogger() error {
	env := os.Getenv("ENV")

	var config zap.Config
	if env == "production" {
		config = zap.NewProductionConfig()
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	} else {
		config = zap.NewDevelopmentConfig()
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncoderConfig.MessageKey = "message"
	config.EncoderConfig.LevelKey = "level"
	config.EncoderConfig.CallerKey = "caller"
	config.EncoderConfig.StacktraceKey = "stacktrace"

	config.InitialFields = map[string]interface{}{
		"service":     "i18n-center",
		"environment": env,
		"version":     os.Getenv("VERSION"),
	}

	var err error
	Logger, err = config.Build(
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
	if err != nil {
		return err
	}

	Sugar = Logger.Sugar()
	return nil
}

// LogRequest logs an HTTP request only for error responses:
//   - 5xx → Error  (stack trace attached, logged for alerting)
//   - 4xx → Warn
//   - 2xx/3xx → skipped (no log entry produced)
func LogRequest(method, path string, statusCode int, latency time.Duration, fields ...zap.Field) {
	var level zapcore.Level
	if statusCode >= 500 {
		level = zapcore.ErrorLevel
	} else if statusCode >= 400 {
		level = zapcore.WarnLevel
	} else {
		return // 2xx/3xx — suppress to avoid log bloat
	}

	base := []zap.Field{
		zap.String("method", method),
		zap.String("path", path),
		zap.Int("status_code", statusCode),
		zap.Int64("latency_ms", latency.Milliseconds()),
	}

	Logger.Check(level, "HTTP request").Write(append(base, fields...)...)
}

// LogError logs an error with an optional message and extra fields.
func LogError(err error, msg string, fields ...zap.Field) {
	if err == nil {
		return
	}
	Logger.Error(msg, append([]zap.Field{zap.Error(err)}, fields...)...)
}

// LogErrorWithContext logs an error together with a free-form context map.
// Useful when you want to attach request metadata without constructing zap.Field slices.
func LogErrorWithContext(err error, msg string, context map[string]interface{}) {
	if err == nil {
		return
	}
	fields := []zap.Field{zap.Error(err)}
	for k, v := range context {
		fields = append(fields, zap.Any(k, v))
	}
	Logger.Error(msg, fields...)
}

// LogPanic logs at Panic level (writes log then panics). Use inside recovery handlers.
func LogPanic(msg string, fields ...zap.Field) {
	Logger.Panic(msg, fields...)
}
