package storage

import (
	"testing"
	"time"

	"s3-static/internal/config"
)

func TestNewStorage_S3Enabled(t *testing.T) {
	cfg := newStorageFactoryConfig(func(c *config.Config) {
		c.S3Endpoint = "localhost:9000"
		c.S3AccessKeyID = "testkey"
		c.S3SecretAccessKey = "testsecret"
		c.S3Region = "us-east-1"
		c.S3UseSSL = false
	})

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
	cfg := newStorageFactoryConfig(func(c *config.Config) {
		c.S3Endpoint = "" // S3 disabled
		c.BasePath = "/tmp/test"
	})

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
			config: newStorageFactoryConfig(func(c *config.Config) {
				c.S3Endpoint = "localhost:9000"
				c.S3AccessKeyID = ""
				c.S3SecretAccessKey = "testsecret"
				c.S3Region = "us-east-1"
			}),
		},
		{
			name: "missing secret key",
			config: newStorageFactoryConfig(func(c *config.Config) {
				c.S3Endpoint = "localhost:9000"
				c.S3AccessKeyID = "testkey"
				c.S3SecretAccessKey = ""
				c.S3Region = "us-east-1"
			}),
		},
		{
			name: "missing region",
			config: newStorageFactoryConfig(func(c *config.Config) {
				c.S3Endpoint = "localhost:9000"
				c.S3AccessKeyID = "testkey"
				c.S3SecretAccessKey = "testsecret"
				c.S3Region = ""
			}),
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
			config: newStorageFactoryConfig(func(c *config.Config) {
				c.S3Endpoint = "localhost:9000"
				c.S3AccessKeyID = "testkey"
				c.S3SecretAccessKey = "testsecret"
				c.S3Region = "us-east-1"
				c.S3UseSSL = false
			}),
			expectError:    true, // Will fail on connection, but config is valid
			expectedErrMsg: "failed to create S3 storage",
		},
		{
			name: "invalid port",
			config: newStorageFactoryConfig(func(c *config.Config) {
				c.Port = "invalid"
				c.S3Endpoint = "localhost:9000"
				c.S3AccessKeyID = "testkey"
				c.S3SecretAccessKey = "testsecret"
				c.S3Region = "us-east-1"
			}),
			expectError:    true,
			expectedErrMsg: "invalid configuration",
		},
		{
			name: "missing bucket name",
			config: newStorageFactoryConfig(func(c *config.Config) {
				c.BucketName = "" // Invalid
				c.S3Endpoint = "localhost:9000"
				c.S3AccessKeyID = "testkey"
				c.S3SecretAccessKey = "testsecret"
				c.S3Region = "us-east-1"
			}),
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

func newStorageFactoryConfig(overrides ...func(*config.Config)) *config.Config {
	cfg := config.DefaultConfig()
	cfg.Host = "localhost"
	cfg.BasePath = "/tmp"
	cfg.BucketName = "test-bucket"
	cfg.DefaultCacheDuration = time.Hour
	cfg.CacheStrategy = "no-cache"

	for _, override := range overrides {
		override(cfg)
	}

	return cfg
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
