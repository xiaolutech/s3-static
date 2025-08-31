package storage

import (
	"testing"
	"time"

	"s3-static/internal/config"
)

func TestNewStorage_S3Enabled(t *testing.T) {
	cfg := &config.Config{
		Port:                 "8080",
		Host:                 "localhost",
		BasePath:             "/tmp",
		S3Endpoint:           "localhost:9000",
		S3AccessKeyID:        "testkey",
		S3SecretAccessKey:    "testsecret",
		S3Region:             "us-east-1",
		BucketName:           "test-bucket",
		S3UseSSL:             false,
		LogLevel:             "info",
		DefaultCacheDuration: time.Hour,
		CacheStrategy:        "no-cache",
	}

	// This will fail because we don't have a real S3 server running
	// but we can test that the factory method is called correctly
	_, err := NewStorage(cfg)
	if err == nil {
		t.Error("Expected error when connecting to non-existent S3 server")
	}

	// The error should be wrapped with our factory error message
	if err != nil && !containsAny(err.Error(), []string{"failed to create S3 storage"}) {
		t.Errorf("Expected factory error wrapper, got: %v", err)
	}
}

func TestNewStorage_S3Disabled(t *testing.T) {
	cfg := &config.Config{
		Port:                 "8080",
		Host:                 "localhost",
		S3Endpoint:           "", // S3 disabled
		BasePath:             "/tmp/test",
		BucketName:           "test-bucket",
		LogLevel:             "info",
		DefaultCacheDuration: time.Hour,
		CacheStrategy:        "no-cache",
	}

	_, err := NewStorage(cfg)
	if err == nil {
		t.Error("Expected error for unsupported storage type (file system not implemented)")
	}

	// Should get an error about unsupported storage type
	if err != nil && !containsAny(err.Error(), []string{"unsupported", "not implemented", "file system", "no storage backend configured"}) {
		t.Errorf("Expected unsupported storage error, got: %v", err)
	}
}

func TestNewStorage_InvalidS3Config(t *testing.T) {
	tests := []struct {
		name   string
		config *config.Config
	}{
		{
			name: "missing access key",
			config: &config.Config{
				Port:                 "8080",
				Host:                 "localhost",
				BasePath:             "/tmp",
				S3Endpoint:           "localhost:9000",
				S3SecretAccessKey:    "testsecret",
				S3Region:             "us-east-1",
				BucketName:           "test-bucket",
				LogLevel:             "info",
				DefaultCacheDuration: time.Hour,
				CacheStrategy:        "no-cache",
			},
		},
		{
			name: "missing secret key",
			config: &config.Config{
				Port:                 "8080",
				Host:                 "localhost",
				BasePath:             "/tmp",
				S3Endpoint:           "localhost:9000",
				S3AccessKeyID:        "testkey",
				S3Region:             "us-east-1",
				BucketName:           "test-bucket",
				LogLevel:             "info",
				DefaultCacheDuration: time.Hour,
				CacheStrategy:        "no-cache",
			},
		},
		{
			name: "missing region",
			config: &config.Config{
				Port:                 "8080",
				Host:                 "localhost",
				BasePath:             "/tmp",
				S3Endpoint:           "localhost:9000",
				S3AccessKeyID:        "testkey",
				S3SecretAccessKey:    "testsecret",
				BucketName:           "test-bucket",
				LogLevel:             "info",
				DefaultCacheDuration: time.Hour,
				CacheStrategy:        "no-cache",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewStorage(tt.config)
			if err == nil {
				t.Error("Expected error for invalid S3 configuration")
			}
			// The error should be related to configuration validation
			if err != nil && !containsAny(err.Error(), []string{"invalid configuration"}) {
				t.Errorf("Expected configuration validation error, got: %v", err)
			}
		})
	}
}

func TestNewStorage_ConfigValidation(t *testing.T) {
	// Test that config validation happens before attempting S3 connection
	tests := []struct {
		name           string
		config         *config.Config
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "valid S3 config",
			config: &config.Config{
				Port:                 "8080",
				Host:                 "localhost",
				BasePath:             "/tmp",
				BucketName:           "test-bucket",
				S3Endpoint:           "localhost:9000",
				S3AccessKeyID:        "testkey",
				S3SecretAccessKey:    "testsecret",
				S3Region:             "us-east-1",
				S3UseSSL:             false,
				LogLevel:             "info",
				DefaultCacheDuration: time.Hour,
				CacheStrategy:        "no-cache",
			},
			expectError:    true, // Will fail on connection, but config is valid
			expectedErrMsg: "failed to create S3 storage",
		},
		{
			name: "invalid port",
			config: &config.Config{
				Port:                 "invalid",
				Host:                 "localhost",
				BasePath:             "/tmp",
				BucketName:           "test-bucket",
				S3Endpoint:           "localhost:9000",
				S3AccessKeyID:        "testkey",
				S3SecretAccessKey:    "testsecret",
				S3Region:             "us-east-1",
				LogLevel:             "info",
				DefaultCacheDuration: time.Hour,
				CacheStrategy:        "no-cache",
			},
			expectError:    true,
			expectedErrMsg: "invalid configuration",
		},
		{
			name: "missing bucket name",
			config: &config.Config{
				Port:                 "8080",
				Host:                 "localhost",
				BasePath:             "/tmp",
				BucketName:           "", // Invalid
				S3Endpoint:           "localhost:9000",
				S3AccessKeyID:        "testkey",
				S3SecretAccessKey:    "testsecret",
				S3Region:             "us-east-1",
				LogLevel:             "info",
				DefaultCacheDuration: time.Hour,
				CacheStrategy:        "no-cache",
			},
			expectError:    true,
			expectedErrMsg: "invalid configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewStorage(tt.config)
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
			if tt.expectError && err != nil && !containsAny(err.Error(), []string{tt.expectedErrMsg}) {
				t.Errorf("Expected error containing '%s', got: %v", tt.expectedErrMsg, err)
			}
		})
	}
}

// Helper function to check if error message contains any of the expected strings
func containsAny(str string, substrings []string) bool {
	for _, substr := range substrings {
		if len(str) >= len(substr) {
			for i := 0; i <= len(str)-len(substr); i++ {
				if str[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}
