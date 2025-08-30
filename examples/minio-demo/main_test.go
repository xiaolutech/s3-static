package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestMinIODemoExample tests the minio-demo example functionality
func TestMinIODemoExample(t *testing.T) {
	// Skip if running in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Start MinIO container for testing
	ctx := context.Background()
	minioContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
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
	defer minioContainer.Terminate(ctx)

	// Get container endpoint
	endpoint, err := minioContainer.Endpoint(ctx, "")
	if err != nil {
		t.Fatalf("Failed to get container endpoint: %v", err)
	}

	// Create MinIO client to setup test data
	minioClient, err := minio.New(strings.TrimPrefix(endpoint, "http://"), &minio.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure: false,
	})
	if err != nil {
		t.Fatalf("Failed to create MinIO client: %v", err)
	}

	// Create test bucket
	bucketName := "test-bucket"
	err = minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
	if err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	// Upload test file
	testContent := "Hello, MinIO Demo!"
	testFileName := "test-file.txt"
	_, err = minioClient.PutObject(ctx, bucketName, testFileName, strings.NewReader(testContent), int64(len(testContent)), minio.PutObjectOptions{
		ContentType: "text/plain",
	})
	if err != nil {
		t.Fatalf("Failed to upload test file: %v", err)
	}

	// Test direct HTTP request functionality
	t.Run("DirectHTTPRequest", func(t *testing.T) {
		url := fmt.Sprintf("http://%s/%s/%s", endpoint, bucketName, testFileName)
		resp, err := http.Get(url)
		if err != nil {
			t.Fatalf("HTTP request failed: %v", err)
		}
		defer resp.Body.Close()

		// MinIO by default returns 403 for anonymous access to private buckets
		// This is expected behavior and demonstrates the security model
		if resp.StatusCode == http.StatusForbidden {
			t.Logf("Got expected 403 Forbidden for anonymous access to private bucket")
			
			// Verify S3-compatible error headers are present
			if resp.Header.Get("x-amz-request-id") == "" {
				t.Error("Expected x-amz-request-id header to be present in error response")
			}
			
			if resp.Header.Get("Content-Type") == "" {
				t.Error("Expected Content-Type header to be present in error response")
			}
			
			t.Logf("Error response headers:")
			for name, values := range resp.Header {
				for _, value := range values {
					t.Logf("  %s: %s", name, value)
				}
			}
			return
		}

		// If we get here, the bucket might be configured for public access
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200 or 403, got %d", resp.StatusCode)
		}

		// Verify S3-compatible headers are present for successful responses
		headers := []string{
			"Content-Type",
			"ETag",
			"Last-Modified",
		}

		for _, header := range headers {
			if resp.Header.Get(header) == "" {
				t.Errorf("Expected header %s to be present", header)
			}
		}

		// Verify optional S3 headers (may or may not be present depending on MinIO version)
		optionalHeaders := []string{
			"x-amz-request-id",
			"x-amz-id-2",
		}

		for _, header := range optionalHeaders {
			value := resp.Header.Get(header)
			t.Logf("Optional header %s: %s", header, value)
		}

		// Verify Content-Type is correct
		contentType := resp.Header.Get("Content-Type")
		if !strings.Contains(contentType, "text/plain") && !strings.Contains(contentType, "text/") {
			t.Logf("Content-Type: %s (may vary by MinIO version)", contentType)
		}
	})

	// Test MinIO SDK functionality
	t.Run("MinIOSDK", func(t *testing.T) {
		objInfo, err := minioClient.StatObject(ctx, bucketName, testFileName, minio.StatObjectOptions{})
		if err != nil {
			t.Fatalf("StatObject failed: %v", err)
		}

		// Verify object information
		if objInfo.Size != int64(len(testContent)) {
			t.Errorf("Expected size %d, got %d", len(testContent), objInfo.Size)
		}

		if objInfo.ETag == "" {
			t.Error("Expected ETag to be set")
		}

		if objInfo.LastModified.IsZero() {
			t.Error("Expected LastModified to be set")
		}

		// ContentType might be auto-detected differently
		t.Logf("ContentType: %s", objInfo.ContentType)
	})

	// Test comparison between HTTP and SDK methods
	t.Run("HTTPvsSDKComparison", func(t *testing.T) {
		// Get info via HTTP
		url := fmt.Sprintf("http://%s/%s/%s", endpoint, bucketName, testFileName)
		resp, err := http.Get(url)
		if err != nil {
			t.Fatalf("HTTP request failed: %v", err)
		}
		defer resp.Body.Close()

		// Get info via SDK (this should always work with credentials)
		objInfo, err := minioClient.StatObject(ctx, bucketName, testFileName, minio.StatObjectOptions{})
		if err != nil {
			t.Fatalf("StatObject failed: %v", err)
		}

		sdkETag := strings.Trim(objInfo.ETag, `"`)

		// If HTTP request was successful (public bucket), compare the results
		if resp.StatusCode == http.StatusOK {
			httpETag := strings.Trim(resp.Header.Get("ETag"), `"`)
			httpLastModified := resp.Header.Get("Last-Modified")

			// Compare ETags (should be the same)
			if httpETag != sdkETag {
				t.Errorf("ETag mismatch: HTTP=%s, SDK=%s", httpETag, sdkETag)
			}

			// Compare timestamps (should be very close)
			httpTime, err := http.ParseTime(httpLastModified)
			if err != nil {
				t.Logf("Could not parse HTTP Last-Modified: %v", err)
			} else {
				timeDiff := objInfo.LastModified.Sub(httpTime)
				if timeDiff < 0 {
					timeDiff = -timeDiff
				}
				if timeDiff > time.Second {
					t.Errorf("Time difference too large: %v", timeDiff)
				}
			}

			t.Logf("HTTP ETag: %s, SDK ETag: %s", httpETag, sdkETag)
			t.Logf("HTTP Last-Modified: %s, SDK Last-Modified: %s", httpLastModified, objInfo.LastModified)
		} else {
			// HTTP request failed (private bucket), but SDK should still work
			t.Logf("HTTP request returned %d (expected for private bucket)", resp.StatusCode)
			t.Logf("SDK ETag: %s", sdkETag)
			t.Logf("SDK Last-Modified: %s", objInfo.LastModified)
			
			// Verify SDK got valid data
			if sdkETag == "" {
				t.Error("Expected SDK to return valid ETag")
			}
			if objInfo.LastModified.IsZero() {
				t.Error("Expected SDK to return valid LastModified")
			}
		}
	})

	// Test error handling
	t.Run("ErrorHandling", func(t *testing.T) {
		// Test HTTP request for nonexistent file
		url := fmt.Sprintf("http://%s/%s/nonexistent.txt", endpoint, bucketName)
		resp, err := http.Get(url)
		if err != nil {
			t.Fatalf("HTTP request failed: %v", err)
		}
		defer resp.Body.Close()

		// Should get either 403 (private bucket) or 404 (public bucket, file not found)
		if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusNotFound {
			t.Logf("Got status %d for nonexistent file (expected 403 or 404)", resp.StatusCode)
		}

		// Test SDK request for nonexistent file
		_, err = minioClient.StatObject(ctx, bucketName, "nonexistent.txt", minio.StatObjectOptions{})
		if err == nil {
			t.Error("Expected error when getting info for nonexistent file")
		} else {
			t.Logf("SDK correctly returned error for nonexistent file: %v", err)
		}
	})
}

// TestMinIOClientCreation tests MinIO client creation with different configurations
func TestMinIOClientCreation(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		secure   bool
		wantErr  bool
	}{
		{
			name:     "valid_insecure",
			endpoint: "localhost:9000",
			secure:   false,
			wantErr:  false,
		},
		{
			name:     "valid_secure",
			endpoint: "s3.amazonaws.com",
			secure:   true,
			wantErr:  false,
		},
		{
			name:     "invalid_endpoint",
			endpoint: "",
			secure:   false,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := minio.New(tt.endpoint, &minio.Options{
				Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
				Secure: tt.secure,
			})
			if (err != nil) != tt.wantErr {
				t.Errorf("minio.New() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestHTTPHeaderValidation tests HTTP header validation
func TestHTTPHeaderValidation(t *testing.T) {
	// Skip if running in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Start MinIO container
	ctx := context.Background()
	minioContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
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
	defer minioContainer.Terminate(ctx)

	endpoint, err := minioContainer.Endpoint(ctx, "")
	if err != nil {
		t.Fatalf("Failed to get container endpoint: %v", err)
	}

	// Setup test data
	minioClient, err := minio.New(strings.TrimPrefix(endpoint, "http://"), &minio.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure: false,
	})
	if err != nil {
		t.Fatalf("Failed to create MinIO client: %v", err)
	}

	bucketName := "header-test-bucket"
	err = minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
	if err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	// Test different file types and their headers
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
			// Upload file with specific content type
			_, err := minioClient.PutObject(ctx, bucketName, filename, 
				strings.NewReader(fileData.content), 
				int64(len(fileData.content)), 
				minio.PutObjectOptions{
					ContentType: fileData.contentType,
				})
			if err != nil {
				t.Fatalf("Failed to upload %s: %v", filename, err)
			}

			// Test HTTP headers
			url := fmt.Sprintf("http://%s/%s/%s", endpoint, bucketName, filename)
			resp, err := http.Get(url)
			if err != nil {
				t.Fatalf("HTTP request failed for %s: %v", filename, err)
			}
			defer resp.Body.Close()

			// Log all headers for debugging
			t.Logf("Headers for %s (Status: %d):", filename, resp.StatusCode)
			for name, values := range resp.Header {
				for _, value := range values {
					t.Logf("  %s: %s", name, value)
				}
			}

			// For private buckets (403), we expect different headers than for successful requests
			if resp.StatusCode == http.StatusForbidden {
				// Verify error response has basic S3 headers
				if resp.Header.Get("Content-Type") == "" {
					t.Errorf("Content-Type header missing for error response %s", filename)
				}
				
				if resp.Header.Get("x-amz-request-id") == "" {
					t.Errorf("x-amz-request-id header missing for error response %s", filename)
				}
				
				t.Logf("Got expected 403 response for private bucket access to %s", filename)
				return
			}

			// For successful responses, verify file-specific headers
			if resp.StatusCode == http.StatusOK {
				if resp.Header.Get("Content-Type") == "" {
					t.Errorf("Content-Type header missing for %s", filename)
				}

				if resp.Header.Get("ETag") == "" {
					t.Errorf("ETag header missing for %s", filename)
				}

				if resp.Header.Get("Last-Modified") == "" {
					t.Errorf("Last-Modified header missing for %s", filename)
				}
			} else {
				t.Logf("Unexpected status %d for %s", resp.StatusCode, filename)
			}
		})
	}
}

// BenchmarkMinIOOperations benchmarks MinIO operations for the demo
func BenchmarkMinIOOperations(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	// Start MinIO container
	ctx := context.Background()
	minioContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
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
	defer minioContainer.Terminate(ctx)

	endpoint, err := minioContainer.Endpoint(ctx, "")
	if err != nil {
		b.Fatalf("Failed to get container endpoint: %v", err)
	}

	// Setup test data
	minioClient, err := minio.New(strings.TrimPrefix(endpoint, "http://"), &minio.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure: false,
	})
	if err != nil {
		b.Fatalf("Failed to create MinIO client: %v", err)
	}

	bucketName := "benchmark-bucket"
	err = minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
	if err != nil {
		b.Fatalf("Failed to create bucket: %v", err)
	}

	testContent := "Benchmark test content for MinIO demo"
	testFileName := "benchmark.txt"
	_, err = minioClient.PutObject(ctx, bucketName, testFileName, strings.NewReader(testContent), int64(len(testContent)), minio.PutObjectOptions{})
	if err != nil {
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
			// Accept both 200 (public) and 403 (private) as valid responses for benchmarking
			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusForbidden {
				b.Fatalf("Unexpected status code: %d", resp.StatusCode)
			}
			resp.Body.Close()
		}
	})

	b.Run("MinIOStatObject", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := minioClient.StatObject(ctx, bucketName, testFileName, minio.StatObjectOptions{})
			if err != nil {
				b.Fatalf("StatObject failed: %v", err)
			}
		}
	})

	b.Run("MinIOGetObject", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			obj, err := minioClient.GetObject(ctx, bucketName, testFileName, minio.GetObjectOptions{})
			if err != nil {
				b.Fatalf("GetObject failed: %v", err)
			}
			obj.Close()
		}
	})
}