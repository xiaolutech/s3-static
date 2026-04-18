package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	testBucket    = "test-bucket"
	testAccessKey = "minioadmin"
	testSecretKey = "minioadmin"
)

// setupMinIOContainer starts a MinIO container for testing
func setupMinIOContainer(t *testing.T) (testcontainers.Container, *S3Storage) {
	ctx := context.Background()

	// Start MinIO container
	req := testcontainers.ContainerRequest{
		Image:        "minio/minio:RELEASE.2024-01-16T16-07-38Z",
		ExposedPorts: []string{"9000/tcp"},
		Env: map[string]string{
			"MINIO_ACCESS_KEY": testAccessKey,
			"MINIO_SECRET_KEY": testSecretKey,
		},
		Cmd:        []string{"server", "/data"},
		WaitingFor: wait.ForHTTP("/minio/health/live").WithPort("9000/tcp"),
	}

	minioContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("Failed to start MinIO container: %v", err)
	}

	// Get connection details
	host, err := minioContainer.Host(ctx)
	if err != nil {
		t.Fatalf("Failed to get MinIO host: %v", err)
	}

	port, err := minioContainer.MappedPort(ctx, "9000")
	if err != nil {
		t.Fatalf("Failed to get MinIO port: %v", err)
	}

	endpoint := fmt.Sprintf("%s:%s", host, port.Port())

	client, err := newTestS3Client(ctx, endpoint)
	if err != nil {
		t.Fatalf("Failed to create S3 client: %v", err)
	}

	err = createBucket(ctx, client, testBucket)
	if err != nil {
		t.Fatalf("Failed to create test bucket: %v", err)
	}

	// Create S3 storage instance
	cfg := S3Config{
		Endpoint:        endpoint,
		Region:          "us-east-1",
		AccessKeyID:     testAccessKey,
		SecretAccessKey: testSecretKey,
		Bucket:          testBucket,
		UseSSL:          false,
	}

	storage, err := NewS3Storage(cfg)
	if err != nil {
		t.Fatalf("Failed to create S3 storage: %v", err)
	}

	return minioContainer, storage
}

func newTestS3Client(ctx context.Context, endpoint string) (*awss3.Client, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(testAccessKey, testSecretKey, "")),
	)
	if err != nil {
		return nil, err
	}

	return awss3.NewFromConfig(cfg, func(o *awss3.Options) {
		o.UsePathStyle = true
		o.BaseEndpoint = aws.String("http://" + endpoint)
	}), nil
}

// createBucket creates a bucket for testing
func createBucket(ctx context.Context, client *awss3.Client, bucket string) error {
	_, err := client.CreateBucket(ctx, &awss3.CreateBucketInput{Bucket: aws.String(bucket)})
	return err
}

// putTestObject uploads a test object to S3
func putTestObject(client *awss3.Client, bucket, key, content string) error {
	_, err := client.PutObject(context.TODO(), &awss3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   strings.NewReader(content),
	})
	return err
}

func TestS3Storage_GetFileInfo(t *testing.T) {
	container, storage := setupMinIOContainer(t)
	defer container.Terminate(context.Background())

	// Test data
	testKey := "test-file.txt"
	testContent := "Hello, World!"

	// Upload test file
	err := putTestObject(storage.client, testBucket, testKey, testContent)
	if err != nil {
		t.Fatalf("Failed to upload test file: %v", err)
	}

	// Test GetFileInfo
	fileInfo, err := storage.GetFileInfo(testKey)
	if err != nil {
		t.Fatalf("GetFileInfo failed: %v", err)
	}

	if fileInfo.Path != testKey {
		t.Errorf("Expected path %s, got %s", testKey, fileInfo.Path)
	}

	if fileInfo.Size != int64(len(testContent)) {
		t.Errorf("Expected size %d, got %d", len(testContent), fileInfo.Size)
	}

	if fileInfo.IsDir {
		t.Error("Expected IsDir to be false")
	}

	if time.Since(fileInfo.ModTime) > time.Minute {
		t.Error("ModTime seems too old")
	}

	// Test ETag is populated
	if fileInfo.ETag == "" {
		t.Error("Expected ETag to be populated")
	}

	// ETag should be a valid format (typically hex string without quotes)
	if len(fileInfo.ETag) < 3 {
		t.Errorf("Expected ETag to have minimum length, got: %s", fileInfo.ETag)
	}
}

func TestS3Storage_ReadFile(t *testing.T) {
	container, storage := setupMinIOContainer(t)
	defer container.Terminate(context.Background())

	// Test data
	testKey := "test-file.txt"
	testContent := "Hello, World!"

	// Upload test file
	err := putTestObject(storage.client, testBucket, testKey, testContent)
	if err != nil {
		t.Fatalf("Failed to upload test file: %v", err)
	}

	// Test ReadFile
	data, err := storage.ReadFile(testKey)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if string(data) != testContent {
		t.Errorf("Expected content %s, got %s", testContent, string(data))
	}
}

func TestS3Storage_OpenFileContext(t *testing.T) {
	container, storage := setupMinIOContainer(t)
	defer container.Terminate(context.Background())

	testKey := "open-file.txt"
	testContent := "Hello from OpenFileContext"

	err := putTestObject(storage.client, testBucket, testKey, testContent)
	if err != nil {
		t.Fatalf("Failed to upload test file: %v", err)
	}

	opened, err := storage.OpenFileContext(context.Background(), testKey)
	if err != nil {
		t.Fatalf("OpenFileContext failed: %v", err)
	}
	defer opened.Reader.Close()

	if opened.Info.Path != testKey {
		t.Fatalf("Expected path %s, got %s", testKey, opened.Info.Path)
	}
	if opened.Info.Size != int64(len(testContent)) {
		t.Fatalf("Expected size %d, got %d", len(testContent), opened.Info.Size)
	}
	if opened.Info.ETag == "" {
		t.Fatal("Expected ETag to be populated")
	}

	data, err := io.ReadAll(opened.Reader)
	if err != nil {
		t.Fatalf("Failed to read opened reader: %v", err)
	}
	if string(data) != testContent {
		t.Fatalf("Expected content %s, got %s", testContent, string(data))
	}
}

func TestS3Storage_FileExists(t *testing.T) {
	container, storage := setupMinIOContainer(t)
	defer container.Terminate(context.Background())

	// Test data
	testKey := "test-file.txt"
	testContent := "Hello, World!"

	// Test non-existent file
	if storage.FileExists(testKey) {
		t.Error("FileExists should return false for non-existent file")
	}

	// Upload test file
	err := putTestObject(storage.client, testBucket, testKey, testContent)
	if err != nil {
		t.Fatalf("Failed to upload test file: %v", err)
	}

	// Test existing file
	if !storage.FileExists(testKey) {
		t.Error("FileExists should return true for existing file")
	}
}

func TestS3Storage_FileNotFound(t *testing.T) {
	container, storage := setupMinIOContainer(t)
	defer container.Terminate(context.Background())

	// Test GetFileInfo with non-existent file
	_, err := storage.GetFileInfo("non-existent-file.txt")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}

	if !IsNotFound(err) {
		t.Errorf("Expected NotFound error, got %T: %v", err, err)
	}

	// Test ReadFile with non-existent file
	_, err = storage.ReadFile("non-existent-file.txt")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}

	if !IsNotFound(err) {
		t.Errorf("Expected NotFound error, got %T: %v", err, err)
	}
}

func TestS3Storage_PathHandling(t *testing.T) {
	container, storage := setupMinIOContainer(t)
	defer container.Terminate(context.Background())

	// Test data
	testKey := "folder/test-file.txt"
	testContent := "Hello, World!"

	// Upload test file
	err := putTestObject(storage.client, testBucket, testKey, testContent)
	if err != nil {
		t.Fatalf("Failed to upload test file: %v", err)
	}

	// Test with leading slash
	data, err := storage.ReadFile("/" + testKey)
	if err != nil {
		t.Fatalf("ReadFile with leading slash failed: %v", err)
	}

	if string(data) != testContent {
		t.Errorf("Expected content %s, got %s", testContent, string(data))
	}

	// Test without leading slash
	data, err = storage.ReadFile(testKey)
	if err != nil {
		t.Fatalf("ReadFile without leading slash failed: %v", err)
	}

	if string(data) != testContent {
		t.Errorf("Expected content %s, got %s", testContent, string(data))
	}
}

func TestS3Storage_ErrorHandling(t *testing.T) {
	container, _ := setupMinIOContainer(t)
	defer container.Terminate(context.Background())

	t.Run("Permission Error Simulation", func(t *testing.T) {
		// Create a storage instance with invalid credentials
		host, err := container.Host(context.Background())
		if err != nil {
			t.Fatalf("Failed to get MinIO host: %v", err)
		}

		port, err := container.MappedPort(context.Background(), "9000")
		if err != nil {
			t.Fatalf("Failed to get MinIO port: %v", err)
		}

		endpoint := fmt.Sprintf("%s:%s", host, port.Port())

		cfg := S3Config{
			Endpoint:        endpoint,
			Region:          "us-east-1",
			AccessKeyID:     "invalid-key",
			SecretAccessKey: "invalid-secret",
			Bucket:          testBucket,
			UseSSL:          false,
		}

		// The constructor should fail with invalid credentials
		_, err = NewS3Storage(cfg)
		if err == nil {
			t.Error("Expected error for invalid credentials during initialization")
		}

		// The error should contain information about access key
		if !strings.Contains(err.Error(), "Access Key") {
			t.Errorf("Expected access key error, got: %v", err)
		}
	})

	t.Run("Invalid Bucket Error", func(t *testing.T) {
		// Create a storage instance with non-existent bucket
		host, err := container.Host(context.Background())
		if err != nil {
			t.Fatalf("Failed to get MinIO host: %v", err)
		}

		port, err := container.MappedPort(context.Background(), "9000")
		if err != nil {
			t.Fatalf("Failed to get MinIO port: %v", err)
		}

		endpoint := fmt.Sprintf("%s:%s", host, port.Port())

		cfg := S3Config{
			Endpoint:        endpoint,
			Region:          "us-east-1",
			AccessKeyID:     testAccessKey,
			SecretAccessKey: testSecretKey,
			Bucket:          "non-existent-bucket",
			UseSSL:          false,
		}

		// The constructor should fail with non-existent bucket
		_, err = NewS3Storage(cfg)
		if err == nil {
			t.Error("Expected error for non-existent bucket during initialization")
		}

		// The error should contain information about bucket not existing
		if !strings.Contains(err.Error(), "does not exist") {
			t.Errorf("Expected bucket not exist error, got: %v", err)
		}
	})
}

func TestS3Storage_ETag(t *testing.T) {
	container, storage := setupMinIOContainer(t)
	defer container.Terminate(context.Background())

	// Test data
	testKey := "test-file.txt"
	testContent := "Hello, World!"

	// Upload test file
	err := putTestObject(storage.client, testBucket, testKey, testContent)
	if err != nil {
		t.Fatalf("Failed to upload test file: %v", err)
	}

	// Test GetFileInfo returns ETag
	fileInfo, err := storage.GetFileInfo(testKey)
	if err != nil {
		t.Fatalf("GetFileInfo failed: %v", err)
	}

	if fileInfo.ETag == "" {
		t.Error("Expected ETag to be populated")
	}

	// ETag should be a valid format (typically quoted hex string)
	if len(fileInfo.ETag) < 3 {
		t.Errorf("Expected ETag to have minimum length, got: %s", fileInfo.ETag)
	}

	// Test that ETag is consistent across calls
	fileInfo2, err := storage.GetFileInfo(testKey)
	if err != nil {
		t.Fatalf("Second GetFileInfo failed: %v", err)
	}

	if fileInfo.ETag != fileInfo2.ETag {
		t.Errorf("Expected consistent ETag, got %s and %s", fileInfo.ETag, fileInfo2.ETag)
	}

	// Test that different files have different ETags
	testKey2 := "test-file-2.txt"
	testContent2 := "Different content"

	err = putTestObject(storage.client, testBucket, testKey2, testContent2)
	if err != nil {
		t.Fatalf("Failed to upload second test file: %v", err)
	}

	fileInfo3, err := storage.GetFileInfo(testKey2)
	if err != nil {
		t.Fatalf("GetFileInfo for second file failed: %v", err)
	}

	if fileInfo.ETag == fileInfo3.ETag {
		t.Error("Expected different files to have different ETags")
	}
}

func TestS3Storage_HTTPStatusMapping(t *testing.T) {
	container, storage := setupMinIOContainer(t)
	defer container.Terminate(context.Background())

	// Test file not found error maps to 404
	_, err := storage.GetFileInfo("non-existent-file.txt")
	if err != nil {
		var storageErr *StorageError
		if errors.As(err, &storageErr) {
			expectedStatus := http.StatusNotFound
			actualStatus := storageErr.Type.ToHTTPStatus()
			if actualStatus != expectedStatus {
				t.Errorf("Expected HTTP status %d, got %d", expectedStatus, actualStatus)
			}
		}
	}
}
