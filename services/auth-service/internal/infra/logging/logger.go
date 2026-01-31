package logging

import (
	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

// logger is the shared zerolog instance for the application.
var logger zerolog.Logger

// InitLogger sets up zerolog with a rolling file backend using lumberjack.
// Log level and rotation settings are provided via parameters.
func InitLogger(logPath string, maxSize, maxBackups, maxAge int, compress bool, logLevel string) {
	// Setup lumberjack for log rotation
	lumberjackLogger := &lumberjack.Logger{
		Filename:   logPath,    // Log file path
		MaxSize:    maxSize,    // Max size in megabytes before rotation
		MaxBackups: maxBackups, // Max number of rotated backups
		MaxAge:     maxAge,     // Max age in days to retain old logs
		Compress:   compress,   // Compress rotated files
	}

	// Setup zerolog with multi-writer output
	multi := zerolog.MultiLevelWriter(lumberjackLogger)

	// Parse log level from string
	level, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		level = zerolog.InfoLevel // fallback to "info"
	}

	// Build logger with timestamp and log level
	logger = zerolog.New(multi).
		With().
		Timestamp().
		Logger().
		Level(level)
}

// Info logs a message with level "info" and optional key-value fields.
func Info(msg string, fields ...any) {
	event := logger.Info()
	for i := 0; i < len(fields)-1; i += 2 {
		key, ok := fields[i].(string)
		if ok {
			event = event.Interface(key, fields[i+1])
		}
	}
	event.Msg(msg)
}

// Warn logs a message with level "warn" and optional key-value fields.
func Warn(msg string, fields ...any) {
	event := logger.Warn()
	for i := 0; i < len(fields)-1; i += 2 {
		key, ok := fields[i].(string)
		if ok {
			event = event.Interface(key, fields[i+1])
		}
	}
	event.Msg(msg)
}

// Error logs a message with level "error" and optional key-value fields.
func Error(msg string, fields ...any) {
	event := logger.Error()
	for i := 0; i < len(fields)-1; i += 2 {
		key, ok := fields[i].(string)
		if ok {
			event = event.Interface(key, fields[i+1])
		}
	}
	event.Msg(msg)
}

// SetLogLevel updates the logger's minimum log level at runtime.
func SetLogLevel(level string) {
	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}
	logger = logger.Level(lvl)
}

// SetLoggerForTest replaces the global logger – for test purposes only.
func SetLoggerForTest(l zerolog.Logger) {
	logger = l
}
