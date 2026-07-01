package storage

import (
	"fmt"
	"s3-static/internal/config"
	"s3-static/pkg/interfaces"
)

// NewStorage creates a new storage instance based on configuration
func NewStorage(cfg *config.Config) (interfaces.Storage, error) {
	// Validate configuration first
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	if cfg.IsS3Enabled() {
		storage, err := NewS3Storage(s3ConfigForBucket(cfg, cfg.BucketName))
		if err != nil {
			return nil, fmt.Errorf("failed to create S3 storage: %w", err)
		}

		return storage, nil
	}

	// For now, return an error if S3 is not configured
	// In the future, we could add local file storage as fallback
	return nil, fmt.Errorf("no storage backend configured - S3_ENDPOINT is required")
}

func NewS3StorageForBucket(cfg *config.Config, bucket string) (*S3Storage, error) {
	if bucket == "" {
		return nil, fmt.Errorf("bucket cannot be empty")
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	storage, err := NewS3Storage(s3ConfigForBucket(cfg, bucket))
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 storage for bucket %q: %w", bucket, err)
	}
	return storage, nil
}

func s3ConfigForBucket(cfg *config.Config, bucket string) S3Config {
	return S3Config{
		Endpoint:        cfg.S3Endpoint,
		AccessKeyID:     cfg.S3AccessKeyID,
		SecretAccessKey: cfg.S3SecretAccessKey,
		UseSSL:          cfg.S3UseSSL,
		Region:          cfg.S3Region,
		Bucket:          bucket,
	}
}
