package config

import (
	"os"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Port != "8080" {
		t.Errorf("Expected default port to be '8080', got '%s'", config.Port)
	}
	if config.Host != "0.0.0.0" {
		t.Errorf("Expected default host to be '0.0.0.0', got '%s'", config.Host)
	}
	if config.BasePath != "./data" {
		t.Errorf("Expected default base path to be './data', got '%s'", config.BasePath)
	}
	if config.BucketName != "default" {
		t.Errorf("Expected default bucket name to be 'default', got '%s'", config.BucketName)
	}
	if config.DefaultCacheDuration != time.Hour {
		t.Errorf("Expected default cache duration to be 1 hour, got %v", config.DefaultCacheDuration)
	}
	if config.LogLevel != "info" {
		t.Errorf("Expected default log level to be 'info', got '%s'", config.LogLevel)
	}
}

func TestLoadFromEnv_WithDefaults(t *testing.T) {
	// Clear environment variables
	clearEnvVars()

	config, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should use default values
	expected := DefaultConfig()
	if config.Port != expected.Port {
		t.Errorf("Expected port '%s', got '%s'", expected.Port, config.Port)
	}
	if config.Host != expected.Host {
		t.Errorf("Expected host '%s', got '%s'", expected.Host, config.Host)
	}
}

func TestLoadFromEnv_WithEnvironmentVariables(t *testing.T) {
	// Set environment variables
	os.Setenv("PORT", "9000")
	os.Setenv("HOST", "127.0.0.1")
	os.Setenv("BASE_PATH", "/custom/path")
	os.Setenv("BUCKET_NAME", "custom-bucket")
	os.Setenv("CACHE_DURATION", "2h30m")
	os.Setenv("LOG_LEVEL", "debug")
	defer clearEnvVars()

	config, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if config.Port != "9000" {
		t.Errorf("Expected port '9000', got '%s'", config.Port)
	}
	if config.Host != "127.0.0.1" {
		t.Errorf("Expected host '127.0.0.1', got '%s'", config.Host)
	}
	if config.BasePath != "/custom/path" {
		t.Errorf("Expected base path '/custom/path', got '%s'", config.BasePath)
	}
	if config.BucketName != "custom-bucket" {
		t.Errorf("Expected bucket name 'custom-bucket', got '%s'", config.BucketName)
	}
	if config.DefaultCacheDuration != 2*time.Hour+30*time.Minute {
		t.Errorf("Expected cache duration '2h30m', got %v", config.DefaultCacheDuration)
	}
	if config.LogLevel != "debug" {
		t.Errorf("Expected log level 'debug', got '%s'", config.LogLevel)
	}
}

func TestLoadFromEnv_InvalidCacheDuration(t *testing.T) {
	os.Setenv("CACHE_DURATION", "invalid-duration")
	defer clearEnvVars()

	_, err := LoadFromEnv()
	if err == nil {
		t.Error("Expected error for invalid cache duration, got nil")
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	config := DefaultConfig()
	err := config.Validate()
	if err != nil {
		t.Errorf("Expected no error for valid config, got %v", err)
	}
}

func TestValidate_InvalidPort(t *testing.T) {
	tests := []struct {
		name string
		port string
	}{
		{"empty port", ""},
		{"non-numeric port", "abc"},
		{"port too low", "0"},
		{"port too high", "65536"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()
			config.Port = tt.port
			err := config.Validate()
			if err == nil {
				t.Errorf("Expected error for port '%s', got nil", tt.port)
			}
		})
	}
}

func TestValidate_EmptyFields(t *testing.T) {
	tests := []struct {
		name   string
		modify func(*Config)
	}{
		{"empty host", func(c *Config) { c.Host = "" }},
		{"empty base path", func(c *Config) { c.BasePath = "" }},
		{"empty bucket name", func(c *Config) { c.BucketName = "" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()
			tt.modify(config)
			err := config.Validate()
			if err == nil {
				t.Error("Expected error for empty field, got nil")
			}
		})
	}
}

func TestValidate_InvalidCacheDuration(t *testing.T) {
	config := DefaultConfig()
	config.DefaultCacheDuration = -time.Hour
	err := config.Validate()
	if err == nil {
		t.Error("Expected error for negative cache duration, got nil")
	}
}

func TestValidate_InvalidLogLevel(t *testing.T) {
	config := DefaultConfig()
	config.LogLevel = "invalid"
	err := config.Validate()
	if err == nil {
		t.Error("Expected error for invalid log level, got nil")
	}
}

func TestGetAddress(t *testing.T) {
	config := &Config{
		Host: "localhost",
		Port: "8080",
	}
	expected := "localhost:8080"
	if address := config.GetAddress(); address != expected {
		t.Errorf("Expected address '%s', got '%s'", expected, address)
	}
}

// Helper function to clear environment variables
func clearEnvVars() {
	envVars := []string{
		"PORT", "HOST", "BASE_PATH", "BUCKET_NAME", "CACHE_DURATION", "LOG_LEVEL",
	}
	for _, env := range envVars {
		os.Unsetenv(env)
	}
}
