package storage

import (
	"fmt"
	"s3-static/internal/config"
	"s3-static/pkg/interfaces"
)

// NewStorage creates a new storage instance based on configuration
func NewStorage(cfg *config.Config) (interfaces.Storage, error) {
	if cfg.IsS3Enabled() {
		// Create S3 storage
		s3Config := S3Config{
			Endpoint:        cfg.S3Endpoint,
			AccessKeyID:     cfg.S3AccessKeyID,
			SecretAccessKey: cfg.S3SecretAccessKey,
			UseSSL:          cfg.S3UseSSL,
			Region:          cfg.S3Region,
			Bucket:          cfg.BucketName,
		}
		
		storage, err := NewS3Storage(s3Config)
		if err != nil {
			return nil, fmt.Errorf("failed to create S3 storage: %w", err)
		}
		
		return storage, nil
	}
	
	// For now, return an error if S3 is not configured
	// In the future, we could add local file storage as fallback
	return nil, fmt.Errorf("no storage backend configured - S3_ENDPOINT is required")
}