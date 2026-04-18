package storage

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"s3-static/pkg/interfaces"
)

// S3Config holds S3 connection configuration
type S3Config struct {
	Endpoint        string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	UseSSL          bool
}

// S3Storage implements the Storage interface for S3-compatible storage
type S3Storage struct {
	client *minio.Client
	bucket string
}

// NewS3Storage creates a new S3Storage instance
func NewS3Storage(cfg S3Config) (*S3Storage, error) {
	// Create MinIO client
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %w", err)
	}

	storage := &S3Storage{
		client: client,
		bucket: cfg.Bucket,
	}

	// Verify bucket exists
	ctx := context.Background()
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket existence: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("bucket %s does not exist", cfg.Bucket)
	}

	return storage, nil
}

// GetFileInfo retrieves file metadata for the given path
func (s *S3Storage) GetFileInfo(path string) (*interfaces.FileInfo, error) {
	return s.GetFileInfoContext(context.Background(), path)
}

// GetFileInfoContext retrieves file metadata for the given path using the provided context.
func (s *S3Storage) GetFileInfoContext(ctx context.Context, path string) (*interfaces.FileInfo, error) {
	key := strings.TrimPrefix(path, "/")

	objInfo, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return nil, MapMinIOError(err, path)
	}

	return &interfaces.FileInfo{
		Path:        path,
		Size:        objInfo.Size,
		ModTime:     objInfo.LastModified,
		IsDir:       false,
		ETag:        strings.Trim(objInfo.ETag, `"`),
		ContentType: objInfo.ContentType,
	}, nil
}

// ReadFile reads the entire file content
func (s *S3Storage) ReadFile(path string) ([]byte, error) {
	return s.ReadFileContext(context.Background(), path)
}

// ReadFileContext reads the entire file content using the provided context.
func (s *S3Storage) ReadFileContext(ctx context.Context, path string) ([]byte, error) {
	key := strings.TrimPrefix(path, "/")

	object, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, MapMinIOError(err, path)
	}
	defer object.Close()

	data, err := io.ReadAll(object)
	if err != nil {
		return nil, MapMinIOError(err, path)
	}

	return data, nil
}

// GetFileReader returns an io.ReadSeekCloser for the given path
func (s *S3Storage) GetFileReader(path string) (io.ReadSeekCloser, error) {
	return s.GetFileReaderContext(context.Background(), path)
}

// GetFileReaderContext returns an io.ReadSeekCloser for the given path using the provided context.
func (s *S3Storage) GetFileReaderContext(ctx context.Context, path string) (io.ReadSeekCloser, error) {
	key := strings.TrimPrefix(path, "/")

	object, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, MapMinIOError(err, path)
	}

	return object, nil
}

// FileExists checks if a file exists at the given path
func (s *S3Storage) FileExists(path string) bool {
	return s.FileExistsContext(context.Background(), path)
}

// FileExistsContext checks if a file exists at the given path using the provided context.
func (s *S3Storage) FileExistsContext(ctx context.Context, path string) bool {
	key := strings.TrimPrefix(path, "/")

	_, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	return err == nil
}
