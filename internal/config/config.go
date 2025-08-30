package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the S3-compatible static file service
type Config struct {
	// Server configuration
	Port string `env:"PORT"`
	Host string `env:"HOST"`

	// Storage configuration
	BasePath   string `env:"BASE_PATH"`
	BucketName string `env:"BUCKET_NAME"`

	// Cache configuration
	DefaultCacheDuration time.Duration `env:"CACHE_DURATION"`

	// Logging configuration
	LogLevel string `env:"LOG_LEVEL"`
}

// DefaultConfig returns a Config with default values
func DefaultConfig() *Config {
	return &Config{
		Port:                 "8080",
		Host:                 "0.0.0.0",
		BasePath:             "./data",
		BucketName:           "default",
		DefaultCacheDuration: time.Hour,
		LogLevel:             "info",
	}
}

// LoadFromEnv loads configuration from environment variables
func LoadFromEnv() (*Config, error) {
	config := DefaultConfig()

	// Load string values
	if port := os.Getenv("PORT"); port != "" {
		config.Port = port
	}
	if host := os.Getenv("HOST"); host != "" {
		config.Host = host
	}
	if basePath := os.Getenv("BASE_PATH"); basePath != "" {
		config.BasePath = basePath
	}
	if bucketName := os.Getenv("BUCKET_NAME"); bucketName != "" {
		config.BucketName = bucketName
	}
	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		config.LogLevel = logLevel
	}

	// Load duration value
	if cacheDurationStr := os.Getenv("CACHE_DURATION"); cacheDurationStr != "" {
		duration, err := time.ParseDuration(cacheDurationStr)
		if err != nil {
			return nil, fmt.Errorf("invalid CACHE_DURATION format: %w", err)
		}
		config.DefaultCacheDuration = duration
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Port == "" {
		return fmt.Errorf("port cannot be empty")
	}

	// Validate port is a valid number
	if port, err := strconv.Atoi(c.Port); err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("port must be a valid number between 1 and 65535, got: %s", c.Port)
	}

	if c.Host == "" {
		return fmt.Errorf("host cannot be empty")
	}

	if c.BasePath == "" {
		return fmt.Errorf("base path cannot be empty")
	}

	if c.BucketName == "" {
		return fmt.Errorf("bucket name cannot be empty")
	}

	if c.DefaultCacheDuration <= 0 {
		return fmt.Errorf("default cache duration must be positive, got: %v", c.DefaultCacheDuration)
	}

	// Validate log level
	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
		"fatal": true,
	}
	if !validLogLevels[c.LogLevel] {
		return fmt.Errorf("invalid log level: %s, must be one of: debug, info, warn, error, fatal", c.LogLevel)
	}

	return nil
}

// GetAddress returns the full address for the HTTP server
func (c *Config) GetAddress() string {
	return c.Host + ":" + c.Port
}