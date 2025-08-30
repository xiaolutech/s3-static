package config

import (
	"log/slog"
	"os"
	"strings"
)

// LogLevel represents the logging level
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
	LogLevelFatal
)

// Logger wraps slog.Logger with additional functionality
type Logger struct {
	*slog.Logger
	level LogLevel
}

// NewLogger creates a new logger with the specified level
func NewLogger(levelStr string) *Logger {
	level := parseLogLevel(levelStr)

	var slogLevel slog.Level
	switch level {
	case LogLevelDebug:
		slogLevel = slog.LevelDebug
	case LogLevelInfo:
		slogLevel = slog.LevelInfo
	case LogLevelWarn:
		slogLevel = slog.LevelWarn
	case LogLevelError:
		slogLevel = slog.LevelError
	case LogLevelFatal:
		slogLevel = slog.LevelError // slog doesn't have fatal, use error
	default:
		slogLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: slogLevel,
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(handler)

	return &Logger{
		Logger: logger,
		level:  level,
	}
}

// parseLogLevel converts string to LogLevel
func parseLogLevel(levelStr string) LogLevel {
	switch strings.ToLower(levelStr) {
	case "debug":
		return LogLevelDebug
	case "info":
		return LogLevelInfo
	case "warn", "warning":
		return LogLevelWarn
	case "error":
		return LogLevelError
	case "fatal":
		return LogLevelFatal
	default:
		return LogLevelInfo
	}
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, args ...any) {
	l.Logger.Debug(msg, args...)
}

// Info logs an info message
func (l *Logger) Info(msg string, args ...any) {
	l.Logger.Info(msg, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, args ...any) {
	l.Logger.Warn(msg, args...)
}

// Error logs an error message
func (l *Logger) Error(msg string, args ...any) {
	l.Logger.Error(msg, args...)
}

// Fatal logs a fatal message and exits the program
func (l *Logger) Fatal(msg string, args ...any) {
	l.Logger.Error(msg, args...)
	os.Exit(1)
}

// WithFields returns a logger with additional fields
func (l *Logger) WithFields(fields map[string]any) *Logger {
	args := make([]any, 0, len(fields)*2)
	for k, v := range fields {
		args = append(args, k, v)
	}

	newLogger := l.Logger.With(args...)
	return &Logger{
		Logger: newLogger,
		level:  l.level,
	}
}

// LogRequest logs an HTTP request with structured fields
func (l *Logger) LogRequest(method, path, remoteAddr string, statusCode int, duration string) {
	l.Info("HTTP request",
		"method", method,
		"path", path,
		"remote_addr", remoteAddr,
		"status_code", statusCode,
		"duration", duration,
	)
}

// LogError logs an error with structured fields
func (l *Logger) LogError(err error, context map[string]any) {
	args := []any{"error", err.Error()}
	for k, v := range context {
		args = append(args, k, v)
	}
	l.Error("Error occurred", args...)
}

// GetLevel returns the current log level
func (l *Logger) GetLevel() LogLevel {
	return l.level
}
