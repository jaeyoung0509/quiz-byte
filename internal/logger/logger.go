package logger

import (
	"os"
	"quiz-byte/internal/config"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var log *zap.Logger

// Initialize sets up the logger with the given configuration
func Initialize(loggerCfg config.LoggerConfig) error {
	// Create encoder config
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Create core
	var core zapcore.Core
	logLevel := zapcore.InfoLevel // Default to InfoLevel
	if loggerCfg.Level == "debug" {
		logLevel = zapcore.DebugLevel
	}

	if loggerCfg.Env == "production" {
		// Production: JSON format
		core = zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.AddSync(os.Stdout),
			logLevel,
		)
	} else {
		// Development: Console format
		core = zapcore.NewCore(
			zapcore.NewConsoleEncoder(encoderConfig),
			zapcore.AddSync(os.Stdout),
			logLevel,
		)
	}

	// Create logger
	log = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	return nil
}

// Get returns the global logger instance
func Get() *zap.Logger {
	return log
}

// Sync flushes any buffered log entries
func Sync() error {
	return log.Sync()
}
