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
	if config.OptimizedImageEnabled {
		t.Error("Expected optimized image serving to be disabled by default")
	}
	if config.OptimizedBucketName != "" {
		t.Errorf("Expected default optimized bucket name to be empty, got '%s'", config.OptimizedBucketName)
	}
	if config.OptimizationProfile != "v1-jpeg82-png-best-w1920" {
		t.Errorf("Expected default optimization profile to be 'v1-jpeg82-png-best-w1920', got '%s'", config.OptimizationProfile)
	}
	if config.OptimizedMinBytes != 512*1024 {
		t.Errorf("Expected default optimized min bytes to be 524288, got %d", config.OptimizedMinBytes)
	}
	if config.OptimizerTriggerURL != "" {
		t.Errorf("Expected default optimizer trigger URL to be empty, got '%s'", config.OptimizerTriggerURL)
	}
	if config.OptimizerTriggerTimeout != 2*time.Second {
		t.Errorf("Expected default optimizer trigger timeout 2s, got %v", config.OptimizerTriggerTimeout)
	}
	if config.DefaultCacheDuration != time.Hour*24*365 {
		t.Errorf("Expected default cache duration to be 1 year, got %v", config.DefaultCacheDuration)
	}
	if config.LogLevel != "info" {
		t.Errorf("Expected default log level to be 'info', got '%s'", config.LogLevel)
	}
}

func TestLoadFromEnv_WithDefaults(t *testing.T) {
	clearEnvVars(t)

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
	clearEnvVars(t)

	// Set environment variables
	os.Setenv("PORT", "9000")
	os.Setenv("HOST", "127.0.0.1")
	os.Setenv("BASE_PATH", "/custom/path")
	os.Setenv("BUCKET_NAME", "custom-bucket")
	os.Setenv("CACHE_DURATION", "2h30m")
	os.Setenv("OPTIMIZED_IMAGE_ENABLED", "true")
	os.Setenv("OPTIMIZED_BUCKET_NAME", "optimized-assets")
	os.Setenv("OPTIMIZATION_PROFILE", "v2-jpeg76-w2560")
	os.Setenv("OPTIMIZED_MIN_BYTES", "262144")
	os.Setenv("OPTIMIZER_TRIGGER_URL", "http://s3-image-optimizer:8080/optimize")
	os.Setenv("OPTIMIZER_TRIGGER_TIMEOUT", "1500ms")
	os.Setenv("LOG_LEVEL", "debug")

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
	if !config.OptimizedImageEnabled {
		t.Error("Expected optimized image serving to be enabled")
	}
	if config.OptimizedBucketName != "optimized-assets" {
		t.Errorf("Expected optimized bucket name 'optimized-assets', got '%s'", config.OptimizedBucketName)
	}
	if config.OptimizationProfile != "v2-jpeg76-w2560" {
		t.Errorf("Expected optimization profile 'v2-jpeg76-w2560', got '%s'", config.OptimizationProfile)
	}
	if config.OptimizedMinBytes != 262144 {
		t.Errorf("Expected optimized min bytes 262144, got %d", config.OptimizedMinBytes)
	}
	if config.OptimizerTriggerURL != "http://s3-image-optimizer:8080/optimize" {
		t.Errorf("Expected optimizer trigger URL to be loaded, got '%s'", config.OptimizerTriggerURL)
	}
	if config.OptimizerTriggerTimeout != 1500*time.Millisecond {
		t.Errorf("Expected optimizer trigger timeout 1500ms, got %v", config.OptimizerTriggerTimeout)
	}
	if config.LogLevel != "debug" {
		t.Errorf("Expected log level 'debug', got '%s'", config.LogLevel)
	}
}

func TestLoadFromEnv_InvalidCacheDuration(t *testing.T) {
	clearEnvVars(t)

	os.Setenv("CACHE_DURATION", "invalid-duration")

	_, err := LoadFromEnv()
	if err == nil {
		t.Error("Expected error for invalid cache duration, got nil")
	}
}

func TestLoadFromEnv_InvalidOptimizedImageEnabled(t *testing.T) {
	clearEnvVars(t)

	os.Setenv("OPTIMIZED_IMAGE_ENABLED", "sometimes")

	_, err := LoadFromEnv()
	if err == nil {
		t.Error("Expected error for invalid optimized image enabled value, got nil")
	}
}

func TestLoadFromEnv_InvalidOptimizedMinBytes(t *testing.T) {
	clearEnvVars(t)

	os.Setenv("OPTIMIZED_MIN_BYTES", "invalid-bytes")

	_, err := LoadFromEnv()
	if err == nil {
		t.Error("Expected error for invalid optimized min bytes value, got nil")
	}
}

func TestLoadFromEnv_InvalidOptimizerTriggerURL(t *testing.T) {
	clearEnvVars(t)

	os.Setenv("OPTIMIZER_TRIGGER_URL", "://bad-url")

	_, err := LoadFromEnv()
	if err == nil {
		t.Error("Expected error for invalid optimizer trigger URL, got nil")
	}
}

func TestLoadFromEnv_InvalidOptimizerTriggerTimeout(t *testing.T) {
	clearEnvVars(t)

	os.Setenv("OPTIMIZER_TRIGGER_TIMEOUT", "soon")

	_, err := LoadFromEnv()
	if err == nil {
		t.Error("Expected error for invalid optimizer trigger timeout, got nil")
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

func TestValidate_OptimizedServingRequiresBucket(t *testing.T) {
	config := DefaultConfig()
	config.OptimizedImageEnabled = true
	config.OptimizedBucketName = ""

	err := config.Validate()
	if err == nil {
		t.Error("Expected error when optimized image serving is enabled without a bucket, got nil")
	}
}

func TestValidate_OptimizedServingValid(t *testing.T) {
	config := DefaultConfig()
	config.OptimizedImageEnabled = true
	config.OptimizedBucketName = "optimized-assets"

	err := config.Validate()
	if err != nil {
		t.Errorf("Expected no error for valid optimized image serving config, got %v", err)
	}
}

func TestValidate_OptimizedMinBytesCannotBeNegative(t *testing.T) {
	config := DefaultConfig()
	config.OptimizedMinBytes = -1

	err := config.Validate()
	if err == nil {
		t.Error("Expected error for negative optimized min bytes, got nil")
	}
}

func TestValidate_OptimizationProfileCannotBeEmpty(t *testing.T) {
	config := DefaultConfig()
	config.OptimizationProfile = ""

	err := config.Validate()
	if err == nil {
		t.Error("Expected error for empty optimization profile, got nil")
	}
}

func TestValidate_OptimizerTriggerTimeoutMustBePositive(t *testing.T) {
	config := DefaultConfig()
	config.OptimizerTriggerURL = "http://s3-image-optimizer:8080/optimize"
	config.OptimizerTriggerTimeout = 0

	err := config.Validate()
	if err == nil {
		t.Error("Expected error for non-positive optimizer trigger timeout, got nil")
	}
}

func TestValidate_OptimizerTriggerURLMustBeAbsoluteHTTP(t *testing.T) {
	tests := []string{
		"://bad-url",
		"ftp://s3-image-optimizer/optimize",
		"/optimize",
	}

	for _, triggerURL := range tests {
		t.Run(triggerURL, func(t *testing.T) {
			config := DefaultConfig()
			config.OptimizerTriggerURL = triggerURL

			err := config.Validate()
			if err == nil {
				t.Error("Expected error for invalid optimizer trigger URL, got nil")
			}
		})
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
func clearEnvVars(t *testing.T) {
	t.Helper()

	envVars := []string{
		"PORT", "HOST",
		"BASE_PATH", "BUCKET_NAME",
		"S3_ENDPOINT", "S3_REGION", "S3_ACCESS_KEY_ID", "S3_SECRET_ACCESS_KEY", "S3_USE_SSL",
		"CACHE_DURATION", "CACHE_STRATEGY",
		"LOG_LEVEL",
		"OPTIMIZED_IMAGE_ENABLED", "OPTIMIZED_BUCKET_NAME", "OPTIMIZATION_PROFILE", "OPTIMIZED_MIN_BYTES",
		"OPTIMIZER_TRIGGER_URL", "OPTIMIZER_TRIGGER_TIMEOUT",
	}

	original := make(map[string]string, len(envVars))
	present := make(map[string]bool, len(envVars))
	for _, env := range envVars {
		if value, ok := os.LookupEnv(env); ok {
			original[env] = value
			present[env] = true
		}
		os.Unsetenv(env)
	}

	t.Cleanup(func() {
		for _, env := range envVars {
			if present[env] {
				os.Setenv(env, original[env])
			} else {
				os.Unsetenv(env)
			}
		}
	})
}
