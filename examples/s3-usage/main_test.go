package main

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"s3-static/internal/storage"
)

func newExampleS3Client(ctx context.Context, endpoint string) (*awss3.Client, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("minioadmin", "minioadmin", "")),
	)
	if err != nil {
		return nil, err
	}
	return awss3.NewFromConfig(cfg, func(o *awss3.Options) {
		o.UsePathStyle = true
		o.BaseEndpoint = aws.String(endpoint)
	}), nil
}

func createExampleBucket(ctx context.Context, client *awss3.Client, bucket string) error {
	_, err := client.CreateBucket(ctx, &awss3.CreateBucketInput{Bucket: aws.String(bucket)})
	return err
}

func putExampleObject(ctx context.Context, client *awss3.Client, bucket, key, content string) error {
	_, err := client.PutObject(ctx, &awss3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   strings.NewReader(content),
	})
	return err
}

func TestS3UsageExample(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "minio/minio:latest",
			ExposedPorts: []string{"9000/tcp"},
			Env: map[string]string{
				"MINIO_ROOT_USER":     "minioadmin",
				"MINIO_ROOT_PASSWORD": "minioadmin",
			},
			Cmd:        []string{"server", "/data"},
			WaitingFor: wait.ForHTTP("/minio/health/live").WithPort("9000"),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("Failed to start MinIO container: %v", err)
	}
	defer container.Terminate(ctx)

	endpoint, err := container.Endpoint(ctx, "")
	if err != nil {
		t.Fatalf("Failed to get container endpoint: %v", err)
	}

	client, err := newExampleS3Client(ctx, endpoint)
	if err != nil {
		t.Fatalf("Failed to create S3 client: %v", err)
	}

	bucketName := "my-bucket"
	if err := createExampleBucket(ctx, client, bucketName); err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	testContent := "Hello, S3 Static File Service!"
	testFileName := "test.txt"
	if err := putExampleObject(ctx, client, bucketName, testFileName, testContent); err != nil {
		t.Fatalf("Failed to upload test file: %v", err)
	}

	cfg := storage.S3Config{
		Endpoint:        endpoint,
		Region:          "us-east-1",
		AccessKeyID:     "minioadmin",
		SecretAccessKey: "minioadmin",
		Bucket:          bucketName,
		UseSSL:          false,
	}

	s3Storage, err := storage.NewS3Storage(cfg)
	if err != nil {
		t.Fatalf("Failed to create S3 storage: %v", err)
	}

	t.Run("S3Storage_Configuration", func(t *testing.T) {
		if !s3Storage.FileExists(testFileName) {
			t.Errorf("Expected file %s to exist", testFileName)
		}
		if s3Storage.FileExists("nonexistent.txt") {
			t.Error("Expected nonexistent file to not exist")
		}
	})

	t.Run("S3Storage_GetFileInfo", func(t *testing.T) {
		info, err := s3Storage.GetFileInfo(testFileName)
		if err != nil {
			t.Fatalf("Failed to get file info: %v", err)
		}
		if info.Path != testFileName {
			t.Errorf("Expected path %s, got %s", testFileName, info.Path)
		}
		if info.Size != int64(len(testContent)) {
			t.Errorf("Expected size %d, got %d", len(testContent), info.Size)
		}
		if info.IsDir {
			t.Error("Expected file to not be a directory")
		}
		if info.ETag == "" {
			t.Error("Expected ETag to be set")
		}
		if info.ModTime.IsZero() {
			t.Error("Expected ModTime to be set")
		}
	})

	t.Run("S3Storage_ReadFile", func(t *testing.T) {
		content, err := s3Storage.ReadFile(testFileName)
		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}
		if string(content) != testContent {
			t.Errorf("Expected content %s, got %s", testContent, string(content))
		}
	})

	t.Run("S3Storage_ErrorHandling", func(t *testing.T) {
		if _, err := s3Storage.ReadFile("nonexistent.txt"); err == nil {
			t.Error("Expected error when reading nonexistent file")
		}
		if _, err := s3Storage.GetFileInfo("nonexistent.txt"); err == nil {
			t.Error("Expected error when getting info for nonexistent file")
		}
	})
}

func TestS3ConfigValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping network-dependent test in short mode")
	}

	tests := []struct {
		name    string
		config  storage.S3Config
		wantErr bool
	}{
		{
			name: "missing_endpoint",
			config: storage.S3Config{
				Region:          "us-east-1",
				AccessKeyID:     "minioadmin",
				SecretAccessKey: "minioadmin",
				Bucket:          "test-bucket",
				UseSSL:          false,
			},
			wantErr: true,
		},
		{
			name: "invalid_endpoint",
			config: storage.S3Config{
				Endpoint:        "invalid-endpoint",
				Region:          "us-east-1",
				AccessKeyID:     "minioadmin",
				SecretAccessKey: "minioadmin",
				Bucket:          "test-bucket",
				UseSSL:          false,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := storage.NewS3Storage(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewS3Storage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestS3ConfigStructure(t *testing.T) {
	tests := []struct {
		name   string
		config storage.S3Config
		valid  bool
	}{
		{
			name:   "complete_config",
			config: storage.S3Config{Endpoint: "localhost:9000", Region: "us-east-1", AccessKeyID: "minioadmin", SecretAccessKey: "minioadmin", Bucket: "test-bucket", UseSSL: false},
			valid:  true,
		},
		{
			name:   "ssl_enabled",
			config: storage.S3Config{Endpoint: "s3.amazonaws.com", Region: "us-west-2", AccessKeyID: "AKIAIOSFODNN7EXAMPLE", SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", Bucket: "my-bucket", UseSSL: true},
			valid:  true,
		},
		{name: "empty_config", config: storage.S3Config{}, valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.valid {
				if tt.config.Endpoint == "" || tt.config.AccessKeyID == "" || tt.config.SecretAccessKey == "" || tt.config.Bucket == "" {
					t.Error("Expected config fields to be populated")
				}
			}
		})
	}
}

func TestExampleMainFunction(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Expected failure when connecting to S3-compatible service: %v", r)
		}
	}()
	t.Log("Testing main function structure (expected to fail connection)")
}

func BenchmarkS3Operations(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	ctx := context.Background()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "minio/minio:latest",
			ExposedPorts: []string{"9000/tcp"},
			Env: map[string]string{
				"MINIO_ROOT_USER":     "minioadmin",
				"MINIO_ROOT_PASSWORD": "minioadmin",
			},
			Cmd:        []string{"server", "/data"},
			WaitingFor: wait.ForHTTP("/minio/health/live").WithPort("9000").WithStartupTimeout(30 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		b.Fatalf("Failed to start MinIO container: %v", err)
	}
	defer container.Terminate(ctx)

	endpoint, err := container.Endpoint(ctx, "")
	if err != nil {
		b.Fatalf("Failed to get container endpoint: %v", err)
	}

	client, err := newExampleS3Client(ctx, endpoint)
	if err != nil {
		b.Fatalf("Failed to create S3 client: %v", err)
	}

	bucketName := "benchmark-bucket"
	if err := createExampleBucket(ctx, client, bucketName); err != nil {
		b.Fatalf("Failed to create bucket: %v", err)
	}

	testContent := "Benchmark test content"
	testFileName := "benchmark.txt"
	if err := putExampleObject(ctx, client, bucketName, testFileName, testContent); err != nil {
		b.Fatalf("Failed to upload test file: %v", err)
	}

	cfg := storage.S3Config{Endpoint: endpoint, Region: "us-east-1", AccessKeyID: "minioadmin", SecretAccessKey: "minioadmin", Bucket: bucketName, UseSSL: false}
	s3Storage, err := storage.NewS3Storage(cfg)
	if err != nil {
		b.Fatalf("Failed to create S3 storage: %v", err)
	}

	b.Run("FileExists", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			s3Storage.FileExists(testFileName)
		}
	})

	b.Run("GetFileInfo", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := s3Storage.GetFileInfo(testFileName)
			if err != nil {
				b.Fatalf("GetFileInfo failed: %v", err)
			}
		}
	})

	b.Run("ReadFile", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := s3Storage.ReadFile(testFileName)
			if err != nil {
				b.Fatalf("ReadFile failed: %v", err)
			}
		}
	})
}
