package config

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected LogLevel
	}{
		{"debug", LogLevelDebug},
		{"DEBUG", LogLevelDebug},
		{"info", LogLevelInfo},
		{"INFO", LogLevelInfo},
		{"warn", LogLevelWarn},
		{"warning", LogLevelWarn},
		{"WARN", LogLevelWarn},
		{"error", LogLevelError},
		{"ERROR", LogLevelError},
		{"fatal", LogLevelFatal},
		{"FATAL", LogLevelFatal},
		{"invalid", LogLevelInfo}, // default to info
		{"", LogLevelInfo},        // default to info
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseLogLevel(tt.input)
			if result != tt.expected {
				t.Errorf("parseLogLevel(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNewLogger(t *testing.T) {
	logger := NewLogger("info")
	if logger == nil {
		t.Fatal("NewLogger returned nil")
	}
	if logger.GetLevel() != LogLevelInfo {
		t.Errorf("Expected log level Info, got %v", logger.GetLevel())
	}
}

func TestLoggerLevels(t *testing.T) {
	// Capture output
	var buf bytes.Buffer

	// Create a logger that writes to our buffer
	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	handler := slog.NewJSONHandler(&buf, opts)
	slogLogger := slog.New(handler)

	logger := &Logger{
		Logger: slogLogger,
		level:  LogLevelDebug,
	}

	// Test different log levels
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	output := buf.String()

	// Check that all messages were logged
	if !strings.Contains(output, "debug message") {
		t.Error("Debug message not found in output")
	}
	if !strings.Contains(output, "info message") {
		t.Error("Info message not found in output")
	}
	if !strings.Contains(output, "warn message") {
		t.Error("Warn message not found in output")
	}
	if !strings.Contains(output, "error message") {
		t.Error("Error message not found in output")
	}
}

func TestLoggerWithFields(t *testing.T) {
	var buf bytes.Buffer

	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	handler := slog.NewJSONHandler(&buf, opts)
	slogLogger := slog.New(handler)

	logger := &Logger{
		Logger: slogLogger,
		level:  LogLevelInfo,
	}

	fields := map[string]any{
		"user_id": "123",
		"action":  "login",
	}

	fieldLogger := logger.WithFields(fields)
	fieldLogger.Info("User action")

	output := buf.String()

	// Parse JSON to verify fields are present
	var logEntry map[string]any
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) > 0 {
		err := json.Unmarshal([]byte(lines[0]), &logEntry)
		if err != nil {
			t.Fatalf("Failed to parse log JSON: %v", err)
		}

		if logEntry["user_id"] != "123" {
			t.Error("user_id field not found or incorrect")
		}
		if logEntry["action"] != "login" {
			t.Error("action field not found or incorrect")
		}
	}
}

func TestLogRequest(t *testing.T) {
	var buf bytes.Buffer

	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	handler := slog.NewJSONHandler(&buf, opts)
	slogLogger := slog.New(handler)

	logger := &Logger{
		Logger: slogLogger,
		level:  LogLevelInfo,
	}

	logger.LogRequest("GET", "/test/path", "192.168.1.1", 200, "10ms")

	output := buf.String()

	// Parse JSON to verify request fields are present
	var logEntry map[string]any
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) > 0 {
		err := json.Unmarshal([]byte(lines[0]), &logEntry)
		if err != nil {
			t.Fatalf("Failed to parse log JSON: %v", err)
		}

		expectedFields := map[string]any{
			"method":      "GET",
			"path":        "/test/path",
			"remote_addr": "192.168.1.1",
			"status_code": float64(200), // JSON numbers are float64
			"duration":    "10ms",
		}

		for field, expectedValue := range expectedFields {
			if logEntry[field] != expectedValue {
				t.Errorf("Field %s: expected %v, got %v", field, expectedValue, logEntry[field])
			}
		}
	}
}

func TestLogError(t *testing.T) {
	var buf bytes.Buffer

	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	handler := slog.NewJSONHandler(&buf, opts)
	slogLogger := slog.New(handler)

	logger := &Logger{
		Logger: slogLogger,
		level:  LogLevelInfo,
	}

	testErr := &testError{message: "test error occurred"}
	context := map[string]any{
		"file": "test.go",
		"line": 42,
	}

	logger.LogError(testErr, context)

	output := buf.String()

	// Parse JSON to verify error fields are present
	var logEntry map[string]any
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) > 0 {
		err := json.Unmarshal([]byte(lines[0]), &logEntry)
		if err != nil {
			t.Fatalf("Failed to parse log JSON: %v", err)
		}

		if logEntry["error"] != "test error occurred" {
			t.Errorf("Error message not found or incorrect: %v", logEntry["error"])
		}
		if logEntry["file"] != "test.go" {
			t.Errorf("Context field 'file' not found or incorrect: %v", logEntry["file"])
		}
		if logEntry["line"] != float64(42) {
			t.Errorf("Context field 'line' not found or incorrect: %v", logEntry["line"])
		}
	}
}

func TestGetLevel(t *testing.T) {
	logger := NewLogger("warn")
	if logger.GetLevel() != LogLevelWarn {
		t.Errorf("Expected log level Warn, got %v", logger.GetLevel())
	}
}

// testError is a simple error implementation for testing
type testError struct {
	message string
}

func (e *testError) Error() string {
	return e.message
}

// TestFatal is tricky to test since it calls os.Exit
// We'll test the behavior without actually exiting
func TestFatalLogging(t *testing.T) {
	var buf bytes.Buffer

	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	handler := slog.NewJSONHandler(&buf, opts)
	slogLogger := slog.New(handler)

	logger := &Logger{
		Logger: slogLogger,
		level:  LogLevelFatal,
	}

	// We can't test the actual Fatal method because it calls os.Exit
	// Instead, we test that the underlying logger works for error level
	logger.Error("fatal-level message")

	output := buf.String()
	if !strings.Contains(output, "fatal-level message") {
		t.Error("Fatal-level message not found in output")
	}
}