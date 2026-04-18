package main

import (
	"context"
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

func newDemoS3Client(ctx context.Context, endpoint string, secure bool) (*awss3.Client, error) {
	if endpoint == "" {
		return nil, fmt.Errorf("endpoint cannot be empty")
	}
	scheme := "http://"
	if secure {
		scheme = "https://"
	}
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		scheme = ""
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("minioadmin", "minioadmin", "")),
	)
	if err != nil {
		return nil, err
	}
	return awss3.NewFromConfig(cfg, func(o *awss3.Options) {
		o.UsePathStyle = true
		o.BaseEndpoint = aws.String(scheme + endpoint)
	}), nil
}

func startDemoMinIO(t testing.TB) (context.Context, testcontainers.Container, string, *awss3.Client) {
	t.Helper()
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
		t.Fatalf("Failed to start MinIO container: %v", err)
	}
	endpoint, err := container.Endpoint(ctx, "")
	if err != nil {
		t.Fatalf("Failed to get container endpoint: %v", err)
	}
	client, err := newDemoS3Client(ctx, endpoint, false)
	if err != nil {
		t.Fatalf("Failed to create S3 client: %v", err)
	}
	return ctx, container, endpoint, client
}

func createDemoBucket(ctx context.Context, client *awss3.Client, bucket string) error {
	_, err := client.CreateBucket(ctx, &awss3.CreateBucketInput{Bucket: aws.String(bucket)})
	return err
}

func putDemoObject(ctx context.Context, client *awss3.Client, bucket, key, content, contentType string) error {
	_, err := client.PutObject(ctx, &awss3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        strings.NewReader(content),
		ContentType: aws.String(contentType),
	})
	return err
}

func TestMinIODemoExample(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, container, endpoint, client := startDemoMinIO(t)
	defer container.Terminate(ctx)

	bucketName := "test-bucket"
	if err := createDemoBucket(ctx, client, bucketName); err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	testContent := "Hello, MinIO Demo!"
	testFileName := "test-file.txt"
	if err := putDemoObject(ctx, client, bucketName, testFileName, testContent, "text/plain"); err != nil {
		t.Fatalf("Failed to upload test file: %v", err)
	}

	t.Run("DirectHTTPRequest", func(t *testing.T) {
		url := fmt.Sprintf("http://%s/%s/%s", endpoint, bucketName, testFileName)
		resp, err := http.Get(url)
		if err != nil {
			t.Fatalf("HTTP request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusForbidden {
			if resp.Header.Get("x-amz-request-id") == "" {
				t.Error("Expected x-amz-request-id header to be present in error response")
			}
			if resp.Header.Get("Content-Type") == "" {
				t.Error("Expected Content-Type header to be present in error response")
			}
			return
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200 or 403, got %d", resp.StatusCode)
		}
		for _, header := range []string{"Content-Type", "ETag", "Last-Modified"} {
			if resp.Header.Get(header) == "" {
				t.Errorf("Expected header %s to be present", header)
			}
		}
	})

	t.Run("S3HeadObject", func(t *testing.T) {
		objInfo, err := client.HeadObject(ctx, &awss3.HeadObjectInput{Bucket: aws.String(bucketName), Key: aws.String(testFileName)})
		if err != nil {
			t.Fatalf("HeadObject failed: %v", err)
		}
		if aws.ToInt64(objInfo.ContentLength) != int64(len(testContent)) {
			t.Errorf("Expected size %d, got %d", len(testContent), aws.ToInt64(objInfo.ContentLength))
		}
		if aws.ToString(objInfo.ETag) == "" {
			t.Error("Expected ETag to be set")
		}
		if objInfo.LastModified == nil || objInfo.LastModified.IsZero() {
			t.Error("Expected LastModified to be set")
		}
	})

	t.Run("HTTPvsSDKComparison", func(t *testing.T) {
		url := fmt.Sprintf("http://%s/%s/%s", endpoint, bucketName, testFileName)
		resp, err := http.Get(url)
		if err != nil {
			t.Fatalf("HTTP request failed: %v", err)
		}
		defer resp.Body.Close()

		objInfo, err := client.HeadObject(ctx, &awss3.HeadObjectInput{Bucket: aws.String(bucketName), Key: aws.String(testFileName)})
		if err != nil {
			t.Fatalf("HeadObject failed: %v", err)
		}

		sdkETag := strings.Trim(aws.ToString(objInfo.ETag), `"`)
		if resp.StatusCode == http.StatusOK {
			httpETag := strings.Trim(resp.Header.Get("ETag"), `"`)
			if httpETag != sdkETag {
				t.Errorf("ETag mismatch: HTTP=%s, SDK=%s", httpETag, sdkETag)
			}
			if lastModified := resp.Header.Get("Last-Modified"); lastModified != "" && objInfo.LastModified != nil {
				httpTime, err := http.ParseTime(lastModified)
				if err == nil {
					delta := objInfo.LastModified.Sub(httpTime)
					if delta < 0 {
						delta = -delta
					}
					if delta > time.Second {
						t.Errorf("Time difference too large: %v", delta)
					}
				}
			}
		} else {
			if sdkETag == "" {
				t.Error("Expected SDK to return valid ETag")
			}
		}
	})

	t.Run("ErrorHandling", func(t *testing.T) {
		url := fmt.Sprintf("http://%s/%s/nonexistent.txt", endpoint, bucketName)
		resp, err := http.Get(url)
		if err != nil {
			t.Fatalf("HTTP request failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusNotFound {
			t.Logf("Got status %d for nonexistent file (expected 403 or 404)", resp.StatusCode)
		}

		_, err = client.HeadObject(ctx, &awss3.HeadObjectInput{Bucket: aws.String(bucketName), Key: aws.String("nonexistent.txt")})
		if err == nil {
			t.Error("Expected error when getting info for nonexistent file")
		}
	})
}

func TestMinIOClientCreation(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		secure   bool
		wantErr  bool
	}{
		{name: "valid_insecure", endpoint: "localhost:9000", secure: false, wantErr: false},
		{name: "valid_secure", endpoint: "s3.amazonaws.com", secure: true, wantErr: false},
		{name: "invalid_endpoint", endpoint: "", secure: false, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newDemoS3Client(context.Background(), tt.endpoint, tt.secure)
			if (err != nil) != tt.wantErr {
				t.Errorf("newDemoS3Client() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHTTPHeaderValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, container, endpoint, client := startDemoMinIO(t)
	defer container.Terminate(ctx)

	bucketName := "header-test-bucket"
	if err := createDemoBucket(ctx, client, bucketName); err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	testFiles := map[string]struct {
		content     string
		contentType string
	}{
		"test.txt":  {"Plain text content", "text/plain"},
		"test.html": {"<html><body>HTML content</body></html>", "text/html"},
		"test.json": {`{"key": "value"}`, "application/json"},
		"test.css":  {"body { color: red; }", "text/css"},
		"test.js":   {"console.log('JavaScript');", "application/javascript"},
	}

	for filename, fileData := range testFiles {
		t.Run(fmt.Sprintf("Headers_%s", filename), func(t *testing.T) {
			if err := putDemoObject(ctx, client, bucketName, filename, fileData.content, fileData.contentType); err != nil {
				t.Fatalf("Failed to upload %s: %v", filename, err)
			}

			url := fmt.Sprintf("http://%s/%s/%s", endpoint, bucketName, filename)
			resp, err := http.Get(url)
			if err != nil {
				t.Fatalf("HTTP request failed for %s: %v", filename, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusForbidden {
				if resp.Header.Get("Content-Type") == "" {
					t.Errorf("Content-Type header missing for error response %s", filename)
				}
				if resp.Header.Get("x-amz-request-id") == "" {
					t.Errorf("x-amz-request-id header missing for error response %s", filename)
				}
				return
			}
			if resp.StatusCode == http.StatusOK {
				for _, header := range []string{"Content-Type", "ETag", "Last-Modified"} {
					if resp.Header.Get(header) == "" {
						t.Errorf("%s header missing for %s", header, filename)
					}
				}
			}
		})
	}
}

func BenchmarkMinIOOperations(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	ctx, container, endpoint, client := startDemoMinIO(b)
	defer container.Terminate(ctx)

	bucketName := "benchmark-bucket"
	if err := createDemoBucket(ctx, client, bucketName); err != nil {
		b.Fatalf("Failed to create bucket: %v", err)
	}

	testContent := "Benchmark test content for MinIO demo"
	testFileName := "benchmark.txt"
	if err := putDemoObject(ctx, client, bucketName, testFileName, testContent, "text/plain"); err != nil {
		b.Fatalf("Failed to upload test file: %v", err)
	}

	b.Run("HTTPRequest", func(b *testing.B) {
		url := fmt.Sprintf("http://%s/%s/%s", endpoint, bucketName, testFileName)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			resp, err := http.Get(url)
			if err != nil {
				b.Fatalf("HTTP request failed: %v", err)
			}
			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusForbidden {
				b.Fatalf("Unexpected status code: %d", resp.StatusCode)
			}
			resp.Body.Close()
		}
	})

	b.Run("S3HeadObject", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := client.HeadObject(ctx, &awss3.HeadObjectInput{Bucket: aws.String(bucketName), Key: aws.String(testFileName)})
			if err != nil {
				b.Fatalf("HeadObject failed: %v", err)
			}
		}
	})

	b.Run("S3GetObject", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			obj, err := client.GetObject(ctx, &awss3.GetObjectInput{Bucket: aws.String(bucketName), Key: aws.String(testFileName)})
			if err != nil {
				b.Fatalf("GetObject failed: %v", err)
			}
			io.Copy(io.Discard, obj.Body)
			obj.Body.Close()
		}
	})
}
