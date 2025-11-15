package observability

import (
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.Logger
var Sugar *zap.SugaredLogger

// InitLogger initializes structured logger
func InitLogger() error {
	env := os.Getenv("ENV")
	if env == "" {
		env = "development"
	}

	var config zap.Config
	if env == "production" {
		config = zap.NewProductionConfig()
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	} else {
		config = zap.NewDevelopmentConfig()
		config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	}

	// Custom encoder config for better readability
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncoderConfig.MessageKey = "message"
	config.EncoderConfig.LevelKey = "level"
	config.EncoderConfig.CallerKey = "caller"
	config.EncoderConfig.StacktraceKey = "stacktrace"

	// Add service metadata
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

// LogError logs an error with context
func LogError(err error, msg string, fields ...zap.Field) {
	if err != nil {
		Logger.Error(msg,
			append([]zap.Field{
				zap.Error(err),
			}, fields...)...,
		)
	}
}

// LogErrorWithContext logs an error with additional context
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

// LogPanic logs a panic with stack trace
func LogPanic(msg string, fields ...zap.Field) {
	Logger.Panic(msg, fields...)
}

// LogRequest logs HTTP request details
func LogRequest(method, path string, statusCode int, latency time.Duration, fields ...zap.Field) {
	level := zapcore.InfoLevel
	if statusCode >= 500 {
		level = zapcore.ErrorLevel
	} else if statusCode >= 400 {
		level = zapcore.WarnLevel
	}

	allFields := append([]zap.Field{
		zap.String("method", method),
		zap.String("path", path),
		zap.Int("status_code", statusCode),
		zap.Duration("latency_ms", latency),
	}, fields...)

	Logger.Check(level, "HTTP Request").Write(allFields...)
}

