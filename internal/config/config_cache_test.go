package config

import (
	"os"
	"testing"
	"time"
)

// TestDefaultConfig_CacheStrategy tests that the default cache strategy is set correctly
func TestDefaultConfig_CacheStrategy(t *testing.T) {
	config := DefaultConfig()

	if config.CacheStrategy != "immutable" {
		t.Errorf("Expected default cache strategy to be 'immutable', got '%s'", config.CacheStrategy)
	}

	if config.DefaultCacheDuration != time.Hour*24*365 {
		t.Errorf("Expected default cache duration to be 1 year, got %v", config.DefaultCacheDuration)
	}
}

// TestLoadFromEnv_CacheStrategy tests loading cache strategy from environment variables
func TestLoadFromEnv_CacheStrategy(t *testing.T) {
	testCases := []struct {
		name          string
		envValue      string
		expectedValue string
		shouldError   bool
	}{
		{
			name:          "no-cache strategy",
			envValue:      "no-cache",
			expectedValue: "no-cache",
			shouldError:   false,
		},
		{
			name:          "max-age strategy",
			envValue:      "max-age",
			expectedValue: "max-age",
			shouldError:   false,
		},
		{
			name:          "immutable strategy",
			envValue:      "immutable",
			expectedValue: "immutable",
			shouldError:   false,
		},
		{
			name:          "invalid strategy",
			envValue:      "invalid-strategy",
			expectedValue: "",
			shouldError:   true,
		},
		{
			name:          "empty strategy uses default",
			envValue:      "",
			expectedValue: "immutable",
			shouldError:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Clear environment variables first
			clearCacheEnvVars()

			if tc.envValue != "" {
				os.Setenv("CACHE_STRATEGY", tc.envValue)
			}
			defer clearCacheEnvVars()

			config, err := LoadFromEnv()

			if tc.shouldError {
				if err == nil {
					t.Errorf("Expected error for cache strategy '%s', got nil", tc.envValue)
				}
				return
			}

			if err != nil {
				t.Fatalf("Expected no error for cache strategy '%s', got %v", tc.envValue, err)
			}

			if config.CacheStrategy != tc.expectedValue {
				t.Errorf("Expected cache strategy '%s', got '%s'", tc.expectedValue, config.CacheStrategy)
			}
		})
	}
}

// TestValidate_CacheStrategy tests cache strategy validation
func TestValidate_CacheStrategy(t *testing.T) {
	testCases := []struct {
		name        string
		strategy    string
		shouldError bool
		description string
	}{
		{
			name:        "valid no-cache",
			strategy:    "no-cache",
			shouldError: false,
			description: "no-cache should be valid",
		},
		{
			name:        "valid max-age",
			strategy:    "max-age",
			shouldError: false,
			description: "max-age should be valid",
		},
		{
			name:        "valid immutable",
			strategy:    "immutable",
			shouldError: false,
			description: "immutable should be valid",
		},
		{
			name:        "invalid strategy",
			strategy:    "invalid-strategy",
			shouldError: true,
			description: "invalid strategy should cause validation error",
		},
		{
			name:        "empty strategy",
			strategy:    "",
			shouldError: true,
			description: "empty strategy should cause validation error",
		},
		{
			name:        "case sensitive",
			strategy:    "NO-CACHE",
			shouldError: true,
			description: "cache strategy should be case sensitive",
		},
		{
			name:        "whitespace strategy",
			strategy:    " no-cache ",
			shouldError: true,
			description: "strategy with whitespace should be invalid",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := DefaultConfig()
			config.CacheStrategy = tc.strategy

			err := config.Validate()

			if tc.shouldError && err == nil {
				t.Errorf("%s: Expected error but got none", tc.description)
			}

			if !tc.shouldError && err != nil {
				t.Errorf("%s: Expected no error but got: %v", tc.description, err)
			}

			if tc.shouldError && err != nil {
				// Verify error message mentions cache strategy
				if !containsString(err.Error(), "cache strategy") {
					t.Errorf("Error message should mention 'cache strategy', got: %v", err)
				}
			}
		})
	}
}

// Helper function to clear cache-related environment variables
func clearCacheEnvVars() {
	envVars := []string{
		"CACHE_STRATEGY",
		"CACHE_DURATION",
	}
	for _, env := range envVars {
		os.Unsetenv(env)
	}
}

// Helper function to check if a string contains a substring
func containsString(str, substr string) bool {
	return len(str) >= len(substr) &&
		(str == substr ||
			(len(str) > len(substr) &&
				(str[:len(substr)] == substr ||
					str[len(str)-len(substr):] == substr ||
					containsSubstring(str, substr))))
}

func containsSubstring(str, substr string) bool {
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}