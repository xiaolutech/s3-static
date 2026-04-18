package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
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
	client *awss3.Client
	bucket string
}

// NewS3Storage creates a new S3Storage instance
func NewS3Storage(cfg S3Config) (*S3Storage, error) {
	endpoint, err := normalizeEndpoint(cfg.Endpoint, cfg.UseSSL)
	if err != nil {
		return nil, fmt.Errorf("invalid S3 endpoint: %w", err)
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := awss3.NewFromConfig(awsCfg, func(o *awss3.Options) {
		o.UsePathStyle = true
		o.BaseEndpoint = aws.String(endpoint)
	})

	storage := &S3Storage{
		client: client,
		bucket: cfg.Bucket,
	}

	exists, err := storage.bucketExists(context.Background(), cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket existence: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("bucket %s does not exist", cfg.Bucket)
	}

	return storage, nil
}

func normalizeEndpoint(endpoint string, useSSL bool) (string, error) {
	if endpoint == "" {
		return "", fmt.Errorf("endpoint cannot be empty")
	}
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		if _, err := url.Parse(endpoint); err != nil {
			return "", err
		}
		return endpoint, nil
	}
	scheme := "http"
	if useSSL {
		scheme = "https"
	}
	normalized := scheme + "://" + endpoint
	if _, err := url.Parse(normalized); err != nil {
		return "", err
	}
	return normalized, nil
}

func (s *S3Storage) bucketExists(ctx context.Context, bucket string) (bool, error) {
	_, err := s.client.HeadBucket(ctx, &awss3.HeadBucketInput{Bucket: aws.String(bucket)})
	if err != nil {
		if IsNotFound(MapS3Error(err, bucket)) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetFileInfo retrieves file metadata for the given path
func (s *S3Storage) GetFileInfo(path string) (*interfaces.FileInfo, error) {
	return s.GetFileInfoContext(context.Background(), path)
}

// GetFileInfoContext retrieves file metadata for the given path using the provided context.
func (s *S3Storage) GetFileInfoContext(ctx context.Context, path string) (*interfaces.FileInfo, error) {
	key := strings.TrimPrefix(path, "/")

	objInfo, err := s.client.HeadObject(ctx, &awss3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, MapS3Error(err, path)
	}

	return fileInfoFromHeadOutput(path, objInfo), nil
}

// ReadFile reads the entire file content
func (s *S3Storage) ReadFile(path string) ([]byte, error) {
	return s.ReadFileContext(context.Background(), path)
}

// ReadFileContext reads the entire file content using the provided context.
func (s *S3Storage) ReadFileContext(ctx context.Context, path string) ([]byte, error) {
	key := strings.TrimPrefix(path, "/")

	object, err := s.client.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, MapS3Error(err, path)
	}
	defer object.Body.Close()

	data, err := io.ReadAll(object.Body)
	if err != nil {
		return nil, MapS3Error(err, path)
	}

	return data, nil
}

// GetFileReader returns an io.ReadSeekCloser for the given path
func (s *S3Storage) GetFileReader(path string) (io.ReadSeekCloser, error) {
	return s.GetFileReaderContext(context.Background(), path)
}

// GetFileReaderContext returns an io.ReadSeekCloser for the given path using the provided context.
func (s *S3Storage) GetFileReaderContext(ctx context.Context, path string) (io.ReadSeekCloser, error) {
	opened, err := s.OpenFileContext(ctx, path)
	if err != nil {
		return nil, err
	}
	return opened.Reader, nil
}

// FileExists checks if a file exists at the given path
func (s *S3Storage) FileExists(path string) bool {
	return s.FileExistsContext(context.Background(), path)
}

// FileExistsContext checks if a file exists at the given path using the provided context.
func (s *S3Storage) FileExistsContext(ctx context.Context, path string) bool {
	_, err := s.GetFileInfoContext(ctx, path)
	return err == nil
}

// OpenFileContext opens a file and retrieves its metadata.
func (s *S3Storage) OpenFileContext(ctx context.Context, path string) (*interfaces.OpenedFile, error) {
	key := strings.TrimPrefix(path, "/")

	object, err := s.client.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, MapS3Error(err, path)
	}

	info := fileInfoFromGetOutput(path, object)
	reader := newS3ObjectReader(ctx, s.client, s.bucket, key, info.Size, object.Body)

	return &interfaces.OpenedFile{
		Info:   info,
		Reader: reader,
	}, nil
}

func fileInfoFromHeadOutput(path string, objInfo *awss3.HeadObjectOutput) *interfaces.FileInfo {
	modTime := time.Time{}
	if objInfo.LastModified != nil {
		modTime = *objInfo.LastModified
	}
	return &interfaces.FileInfo{
		Path:        path,
		Size:        aws.ToInt64(objInfo.ContentLength),
		ModTime:     modTime,
		IsDir:       false,
		ETag:        strings.Trim(aws.ToString(objInfo.ETag), `"`),
		ContentType: aws.ToString(objInfo.ContentType),
	}
}

func fileInfoFromGetOutput(path string, objInfo *awss3.GetObjectOutput) *interfaces.FileInfo {
	modTime := time.Time{}
	if objInfo.LastModified != nil {
		modTime = *objInfo.LastModified
	}
	return &interfaces.FileInfo{
		Path:        path,
		Size:        aws.ToInt64(objInfo.ContentLength),
		ModTime:     modTime,
		IsDir:       false,
		ETag:        strings.Trim(aws.ToString(objInfo.ETag), `"`),
		ContentType: aws.ToString(objInfo.ContentType),
	}
}

type s3ObjectReader struct {
	ctx    context.Context
	client *awss3.Client
	bucket string
	key    string
	size   int64
	body   io.ReadCloser
	offset int64
	closed bool
}

func newS3ObjectReader(ctx context.Context, client *awss3.Client, bucket, key string, size int64, body io.ReadCloser) *s3ObjectReader {
	return &s3ObjectReader{ctx: ctx, client: client, bucket: bucket, key: key, size: size, body: body}
}

func (r *s3ObjectReader) Read(p []byte) (int, error) {
	if r.closed {
		return 0, io.ErrClosedPipe
	}
	if r.body == nil {
		return 0, io.EOF
	}
	n, err := r.body.Read(p)
	r.offset += int64(n)
	if err == io.EOF && r.offset >= r.size {
		_ = r.body.Close()
		r.body = nil
	}
	return n, err
}

func (r *s3ObjectReader) Seek(offset int64, whence int) (int64, error) {
	if r.closed {
		return 0, io.ErrClosedPipe
	}

	var absolute int64
	switch whence {
	case io.SeekStart:
		absolute = offset
	case io.SeekCurrent:
		absolute = r.offset + offset
	case io.SeekEnd:
		absolute = r.size + offset
	default:
		return 0, fmt.Errorf("invalid whence: %d", whence)
	}

	if absolute < 0 {
		return 0, fmt.Errorf("negative position")
	}
	if absolute > r.size {
		absolute = r.size
	}
	if absolute == r.offset {
		return absolute, nil
	}
	if err := r.resetTo(absolute); err != nil {
		return 0, err
	}
	return absolute, nil
}

func (r *s3ObjectReader) Close() error {
	r.closed = true
	if r.body == nil {
		return nil
	}
	err := r.body.Close()
	r.body = nil
	return err
}

func (r *s3ObjectReader) resetTo(offset int64) error {
	if r.body != nil {
		_ = r.body.Close()
		r.body = nil
	}
	r.offset = offset
	if offset >= r.size {
		return nil
	}

	resp, err := r.client.GetObject(r.ctx, &awss3.GetObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(r.key),
		Range:  aws.String(fmt.Sprintf("bytes=%d-", offset)),
	})
	if err != nil {
		return err
	}
	body := resp.Body
	if rc, ok := body.(io.ReadCloser); ok {
		r.body = rc
	} else {
		r.body = io.NopCloser(body)
	}
	return nil
}
