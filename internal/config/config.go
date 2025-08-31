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

	// S3 Storage configuration
	S3Endpoint        string `env:"S3_ENDPOINT"`
	S3Region          string `env:"S3_REGION"`
	S3AccessKeyID     string `env:"S3_ACCESS_KEY_ID"`
	S3SecretAccessKey string `env:"S3_SECRET_ACCESS_KEY"`
	S3UseSSL          bool   `env:"S3_USE_SSL"`

	// Cache configuration
	DefaultCacheDuration time.Duration `env:"CACHE_DURATION"`
	CacheStrategy        string        `env:"CACHE_STRATEGY"` // "no-cache", "max-age", or "immutable"

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
		S3Region:             "us-east-1",
		S3UseSSL:             true,
		DefaultCacheDuration: time.Hour * 24 * 365, // 1 year
		CacheStrategy:        "immutable",          // Default strategy
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
	if cacheStrategy := os.Getenv("CACHE_STRATEGY"); cacheStrategy != "" {
		config.CacheStrategy = cacheStrategy
	}

	// Load S3 configuration
	if s3Endpoint := os.Getenv("S3_ENDPOINT"); s3Endpoint != "" {
		config.S3Endpoint = s3Endpoint
	}
	if s3Region := os.Getenv("S3_REGION"); s3Region != "" {
		config.S3Region = s3Region
	}
	if s3AccessKeyID := os.Getenv("S3_ACCESS_KEY_ID"); s3AccessKeyID != "" {
		config.S3AccessKeyID = s3AccessKeyID
	}
	if s3SecretAccessKey := os.Getenv("S3_SECRET_ACCESS_KEY"); s3SecretAccessKey != "" {
		config.S3SecretAccessKey = s3SecretAccessKey
	}

	// Load boolean values
	if s3UseSSLStr := os.Getenv("S3_USE_SSL"); s3UseSSLStr != "" {
		useSSL, err := strconv.ParseBool(s3UseSSLStr)
		if err != nil {
			return nil, fmt.Errorf("invalid S3_USE_SSL format: %w", err)
		}
		config.S3UseSSL = useSSL
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

	// Validate cache strategy
	validCacheStrategies := map[string]bool{
		"no-cache":  true,
		"max-age":   true,
		"immutable": true,
	}
	if !validCacheStrategies[c.CacheStrategy] {
		return fmt.Errorf("invalid cache strategy: %s, must be one of: no-cache, max-age, immutable", c.CacheStrategy)
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

	// Validate S3 configuration if S3 endpoint is provided
	if c.S3Endpoint != "" {
		if c.S3AccessKeyID == "" {
			return fmt.Errorf("S3_ACCESS_KEY_ID is required when S3_ENDPOINT is provided")
		}
		if c.S3SecretAccessKey == "" {
			return fmt.Errorf("S3_SECRET_ACCESS_KEY is required when S3_ENDPOINT is provided")
		}
		if c.S3Region == "" {
			return fmt.Errorf("S3_REGION is required when S3_ENDPOINT is provided")
		}
	}

	return nil
}

// GetAddress returns the full address for the HTTP server
func (c *Config) GetAddress() string {
	return c.Host + ":" + c.Port
}

// GetS3Config returns S3 configuration from the main config
func (c *Config) GetS3Config() *S3Config {
	return &S3Config{
		Endpoint:        c.S3Endpoint,
		AccessKeyID:     c.S3AccessKeyID,
		SecretAccessKey: c.S3SecretAccessKey,
		UseSSL:          c.S3UseSSL,
		Region:          c.S3Region,
		BucketName:      c.BucketName,
	}
}

// S3Config holds S3-specific configuration
type S3Config struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
	Region          string
	BucketName      string
}

// IsS3Enabled returns true if S3 configuration is provided
func (c *Config) IsS3Enabled() bool {
	return c.S3Endpoint != ""
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	return LoadFromEnv()
}
