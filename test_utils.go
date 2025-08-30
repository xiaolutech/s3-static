package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"s3-static/internal/config"
	"s3-static/internal/handler"
	"s3-static/internal/storage"
	"s3-static/pkg/interfaces"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestSuite provides utilities for integration testing
type TestSuite struct {
	Container testcontainers.Container
	Storage   *storage.S3Storage
	Handler   *handler.FileHandler
	Config    *config.Config
	Logger    *config.Logger
	Client    *minio.Client
}

// SetupTestSuite creates a complete test environment with MinIO container
func SetupTestSuite(t *testing.T) *TestSuite {
	ctx := context.Background()

	// Start MinIO container
	req := testcontainers.ContainerRequest{
		Image:        "minio/minio:RELEASE.2024-01-16T16-07-38Z",
		ExposedPorts: []string{"9000/tcp"},
		Env: map[string]string{
			"MINIO_ACCESS_KEY": "minioadmin",
			"MINIO_SECRET_KEY": "minioadmin",
		},
		Cmd:        []string{"server", "/data"},
		WaitingFor: wait.ForHTTP("/minio/health/live").WithPort("9000/tcp"),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("Failed to start MinIO container: %v", err)
	}

	// Get connection details
	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("Failed to get MinIO host: %v", err)
	}

	port, err := container.MappedPort(ctx, "9000")
	if err != nil {
		t.Fatalf("Failed to get MinIO port: %v", err)
	}

	endpoint := fmt.Sprintf("%s:%s", host, port.Port())

	// Create MinIO client
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4("minioadmin", "minioadmin", ""),
		Secure: false,
		Region: "us-east-1",
	})
	if err != nil {
		t.Fatalf("Failed to create MinIO client: %v", err)
	}

	// Create test bucket
	bucketName := "test-bucket"
	err = client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
	if err != nil {
		t.Fatalf("Failed to create test bucket: %v", err)
	}

	// Create configuration
	cfg := &config.Config{
		Port:                 "8080",
		Host:                 "localhost",
		S3Endpoint:           endpoint,
		S3AccessKeyID:        "minioadmin",
		S3SecretAccessKey:    "minioadmin",
		S3Region:             "us-east-1",
		BucketName:           bucketName,
		S3UseSSL:             false,
		DefaultCacheDuration: time.Hour,
		LogLevel:             "info",
	}

	// Create logger
	logger := config.NewLogger(cfg.LogLevel)

	// Create storage
	s3Storage, err := storage.NewS3Storage(storage.S3Config{
		Endpoint:        endpoint,
		Region:          "us-east-1",
		AccessKeyID:     "minioadmin",
		SecretAccessKey: "minioadmin",
		Bucket:          bucketName,
		UseSSL:          false,
	})
	if err != nil {
		t.Fatalf("Failed to create S3 storage: %v", err)
	}

	// Create handler
	fileHandler := handler.NewFileHandler(s3Storage, cfg, logger)

	return &TestSuite{
		Container: container,
		Storage:   s3Storage,
		Handler:   fileHandler,
		Config:    cfg,
		Logger:    logger,
		Client:    client,
	}
}

// Cleanup terminates the test container
func (ts *TestSuite) Cleanup() {
	if ts.Container != nil {
		ts.Container.Terminate(context.Background())
	}
}

// UploadTestFile uploads a test file to the MinIO container
func (ts *TestSuite) UploadTestFile(key, content string) error {
	_, err := ts.Client.PutObject(context.TODO(), ts.Config.BucketName, key,
		strings.NewReader(content), int64(len(content)), minio.PutObjectOptions{})
	return err
}

// CreateTestRequest creates an HTTP test request
func (ts *TestSuite) CreateTestRequest(method, path string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, path, body)
	return req
}

// ExecuteRequest executes a request against the handler and returns the response
func (ts *TestSuite) ExecuteRequest(req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	ts.Handler.ServeHTTP(w, req)
	return w
}

// AssertStatusCode asserts that the response has the expected status code
func (ts *TestSuite) AssertStatusCode(t *testing.T, w *httptest.ResponseRecorder, expected int) {
	if w.Code != expected {
		t.Errorf("Expected status code %d, got %d. Body: %s", expected, w.Code, w.Body.String())
	}
}

// AssertHeader asserts that the response has the expected header value
func (ts *TestSuite) AssertHeader(t *testing.T, w *httptest.ResponseRecorder, header, expected string) {
	actual := w.Header().Get(header)
	if actual != expected {
		t.Errorf("Expected header %s to be '%s', got '%s'", header, expected, actual)
	}
}

// AssertHeaderExists asserts that the response has the specified header
func (ts *TestSuite) AssertHeaderExists(t *testing.T, w *httptest.ResponseRecorder, header string) {
	if w.Header().Get(header) == "" {
		t.Errorf("Expected header %s to be present", header)
	}
}

// AssertBodyContains asserts that the response body contains the expected string
func (ts *TestSuite) AssertBodyContains(t *testing.T, w *httptest.ResponseRecorder, expected string) {
	body := w.Body.String()
	if !strings.Contains(body, expected) {
		t.Errorf("Expected body to contain '%s', got: %s", expected, body)
	}
}

// AssertBodyEquals asserts that the response body equals the expected string
func (ts *TestSuite) AssertBodyEquals(t *testing.T, w *httptest.ResponseRecorder, expected string) {
	body := w.Body.String()
	if body != expected {
		t.Errorf("Expected body to be '%s', got: %s", expected, body)
	}
}

// MockStorage provides a simple in-memory storage implementation for testing
type MockStorage struct {
	files map[string]*interfaces.FileInfo
	data  map[string][]byte
}

// NewMockStorage creates a new MockStorage instance
func NewMockStorage() *MockStorage {
	return &MockStorage{
		files: make(map[string]*interfaces.FileInfo),
		data:  make(map[string][]byte),
	}
}

// GetFileInfo implements interfaces.Storage
func (m *MockStorage) GetFileInfo(path string) (*interfaces.FileInfo, error) {
	if info, exists := m.files[path]; exists {
		return info, nil
	}
	return nil, &storage.StorageError{
		Type:    storage.ErrorNotFound,
		Message: "file not found",
		Path:    path,
	}
}

// ReadFile implements interfaces.Storage
func (m *MockStorage) ReadFile(path string) ([]byte, error) {
	if data, exists := m.data[path]; exists {
		return data, nil
	}
	return nil, &storage.StorageError{
		Type:    storage.ErrorNotFound,
		Message: "file not found",
		Path:    path,
	}
}

// FileExists implements interfaces.Storage
func (m *MockStorage) FileExists(path string) bool {
	_, exists := m.files[path]
	return exists
}

// AddFile adds a file to the mock storage
func (m *MockStorage) AddFile(path string, content []byte, modTime time.Time, etag string) {
	if etag == "" {
		etag = "mock-etag"
	}
	m.files[path] = &interfaces.FileInfo{
		Path:    path,
		Size:    int64(len(content)),
		ModTime: modTime,
		IsDir:   false,
		ETag:    etag,
	}
	m.data[path] = content
}

// RemoveFile removes a file from the mock storage
func (m *MockStorage) RemoveFile(path string) {
	delete(m.files, path)
	delete(m.data, path)
}

// Clear removes all files from the mock storage
func (m *MockStorage) Clear() {
	m.files = make(map[string]*interfaces.FileInfo)
	m.data = make(map[string][]byte)
}

// CaptureLogOutput captures log output for testing
type LogCapture struct {
	buffer *bytes.Buffer
}

// NewLogCapture creates a new log capture instance
func NewLogCapture() *LogCapture {
	return &LogCapture{
		buffer: &bytes.Buffer{},
	}
}

// GetOutput returns the captured log output
func (lc *LogCapture) GetOutput() string {
	return lc.buffer.String()
}

// Contains checks if the captured output contains the specified string
func (lc *LogCapture) Contains(s string) bool {
	return strings.Contains(lc.buffer.String(), s)
}

// Clear clears the captured output
func (lc *LogCapture) Clear() {
	lc.buffer.Reset()
}

// TestHelper provides common test utilities
type TestHelper struct{}

// NewTestHelper creates a new test helper
func NewTestHelper() *TestHelper {
	return &TestHelper{}
}

// AssertNoError asserts that the error is nil
func (th *TestHelper) AssertNoError(t *testing.T, err error) {
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

// AssertError asserts that an error occurred
func (th *TestHelper) AssertError(t *testing.T, err error) {
	if err == nil {
		t.Fatal("Expected an error, got nil")
	}
}

// AssertErrorContains asserts that the error message contains the expected string
func (th *TestHelper) AssertErrorContains(t *testing.T, err error, expected string) {
	if err == nil {
		t.Fatal("Expected an error, got nil")
	}
	if !strings.Contains(err.Error(), expected) {
		t.Errorf("Expected error to contain '%s', got: %v", expected, err)
	}
}

// AssertStringEquals asserts that two strings are equal
func (th *TestHelper) AssertStringEquals(t *testing.T, actual, expected string) {
	if actual != expected {
		t.Errorf("Expected '%s', got '%s'", expected, actual)
	}
}

// AssertIntEquals asserts that two integers are equal
func (th *TestHelper) AssertIntEquals(t *testing.T, actual, expected int) {
	if actual != expected {
		t.Errorf("Expected %d, got %d", expected, actual)
	}
}

// AssertInt64Equals asserts that two int64 values are equal
func (th *TestHelper) AssertInt64Equals(t *testing.T, actual, expected int64) {
	if actual != expected {
		t.Errorf("Expected %d, got %d", expected, actual)
	}
}

// AssertBoolEquals asserts that two boolean values are equal
func (th *TestHelper) AssertBoolEquals(t *testing.T, actual, expected bool) {
	if actual != expected {
		t.Errorf("Expected %t, got %t", expected, actual)
	}
}

// AssertTimeEquals asserts that two times are equal (within a second)
func (th *TestHelper) AssertTimeEquals(t *testing.T, actual, expected time.Time) {
	if actual.Truncate(time.Second) != expected.Truncate(time.Second) {
		t.Errorf("Expected time %v, got %v", expected, actual)
	}
}
