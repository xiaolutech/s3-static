package handler

import (
	"net/http"
	"net/http/httptest"
	"s3-static/internal/storage"
	"strings"
	"testing"
	"time"

	"s3-static/internal/config"
	"s3-static/pkg/interfaces"
)

// mockStorage implements interfaces.Storage for testing
type mockStorage struct {
	files map[string]*interfaces.FileInfo
	data  map[string][]byte
}

func newMockStorage() *mockStorage {
	return &mockStorage{
		files: make(map[string]*interfaces.FileInfo),
		data:  make(map[string][]byte),
	}
}

func (m *mockStorage) GetFileInfo(path string) (*interfaces.FileInfo, error) {
	if info, exists := m.files[path]; exists {
		return info, nil
	}
	return nil, &storage.StorageError{Type: storage.ErrorNotFound, Message: "file not found"}
}

func (m *mockStorage) ReadFile(path string) ([]byte, error) {
	if data, exists := m.data[path]; exists {
		return data, nil
	}
	return nil, &storage.StorageError{Type: storage.ErrorNotFound, Message: "file not found"}
}

func (m *mockStorage) FileExists(path string) bool {
	_, exists := m.files[path]
	return exists
}

func (m *mockStorage) addFile(path string, content []byte, modTime time.Time) {
	m.files[path] = &interfaces.FileInfo{
		Path:    path,
		Size:    int64(len(content)),
		ModTime: modTime,
		IsDir:   false,
		ETag:    "test-etag",
	}
	m.data[path] = content
}

func (m *mockStorage) addFileWithContentType(path string, content []byte, modTime time.Time, contentType string) {
	m.addFile(path, content, modTime)
	m.files[path].ContentType = contentType
}

func TestFileHandler_UsesS3ETag(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := config.NewLogger("info")
	storage := newMockStorage()
	handler := NewFileHandler(storage, cfg, logger)

	// Add test file with S3 ETag
	content := []byte("Hello, World!")
	modTime := time.Now().Truncate(time.Second)
	storage.addFile("test.txt", content, modTime)

	// Test that handler uses the S3 ETag directly
	req := httptest.NewRequest("GET", "/test.txt", nil)
	w := httptest.NewRecorder()

	handler.handleGetObject(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Check that the ETag header matches the S3 ETag
	expectedETag := `"test-etag"`
	if w.Header().Get("ETag") != expectedETag {
		t.Errorf("Expected ETag header '%s', got '%s'", expectedETag, w.Header().Get("ETag"))
	}
}

func TestFileHandler_CheckConditionalRequest(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := config.NewLogger("info")
	storage := newMockStorage()
	handler := NewFileHandler(storage, cfg, logger)

	etag := "test-etag"
	modTime := time.Now().UTC().Truncate(time.Second)

	tests := []struct {
		name     string
		headers  map[string]string
		expected bool
	}{
		{
			name:     "No conditional headers",
			headers:  map[string]string{},
			expected: false,
		},
		{
			name:     "If-None-Match matches",
			headers:  map[string]string{"If-None-Match": etag},
			expected: true,
		},
		{
			name:     "If-None-Match matches with quotes",
			headers:  map[string]string{"If-None-Match": `"` + etag + `"`},
			expected: true,
		},
		{
			name:     "If-None-Match wildcard",
			headers:  map[string]string{"If-None-Match": "*"},
			expected: true,
		},
		{
			name:     "If-Modified-Since not modified",
			headers:  map[string]string{"If-Modified-Since": modTime.Add(time.Hour).Format(http.TimeFormat)},
			expected: true,
		},
		{
			name:     "If-Modified-Since modified",
			headers:  map[string]string{"If-Modified-Since": modTime.Add(-time.Hour).Format(http.TimeFormat)},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test.txt", nil)
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			result := handler.checkConditionalRequest(req, etag, modTime)
			if result != tt.expected {
				t.Errorf("Test '%s': Expected %v, got %v. ModTime: %v, Headers: %v",
					tt.name, tt.expected, result, modTime, tt.headers)
			}
		})
	}
}

func TestFileHandler_HandleGetObject(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := config.NewLogger("info")
	storage := newMockStorage()
	handler := NewFileHandler(storage, cfg, logger)

	// Add test file
	content := []byte("Hello, World!")
	modTime := time.Now().Truncate(time.Second)
	storage.addFile("test.txt", content, modTime)

	// Test successful request
	req := httptest.NewRequest("GET", "/test.txt", nil)
	w := httptest.NewRecorder()

	handler.handleGetObject(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Body.String() != string(content) {
		t.Errorf("Expected body '%s', got '%s'", string(content), w.Body.String())
	}

	// Check cache headers
	if w.Header().Get("ETag") == "" {
		t.Error("Expected ETag header to be set")
	}

	if w.Header().Get("Last-Modified") == "" {
		t.Error("Expected Last-Modified header to be set")
	}

	if w.Header().Get("Cache-Control") == "" {
		t.Error("Expected Cache-Control header to be set")
	}
}

func TestFileHandler_HandleGetObject_NotModified(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := config.NewLogger("info")
	storage := newMockStorage()
	handler := NewFileHandler(storage, cfg, logger)

	// Add test file
	content := []byte("Hello, World!")
	modTime := time.Now().Truncate(time.Second)
	storage.addFile("test.txt", content, modTime)

	// Test with If-None-Match header
	req := httptest.NewRequest("GET", "/test.txt", nil)
	req.Header.Set("If-None-Match", "test-etag")
	w := httptest.NewRecorder()

	handler.handleGetObject(w, req)

	if w.Code != http.StatusNotModified {
		t.Errorf("Expected status 304, got %d", w.Code)
	}

	if w.Body.Len() != 0 {
		t.Error("Expected empty body for 304 response")
	}
}

func TestFileHandler_HandleGetObject_FileNotFound(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := config.NewLogger("info")
	storage := newMockStorage()
	handler := NewFileHandler(storage, cfg, logger)

	// Test request for non-existent file
	req := httptest.NewRequest("GET", "/nonexistent.txt", nil)
	w := httptest.NewRecorder()

	handler.handleGetObject(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}

	if w.Header().Get("Content-Type") != "application/xml" {
		t.Error("Expected XML content type for error response")
	}

	body := w.Body.String()
	if !strings.Contains(body, "<Code>NoSuchKey</Code>") {
		t.Errorf("Expected XML error response with NoSuchKey code, got: %s", body)
	}
}

func TestFileHandler_HandleGetObject_EmptyPath(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := config.NewLogger("info")
	storage := newMockStorage()
	handler := NewFileHandler(storage, cfg, logger)

	// Test request with empty path
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.handleGetObject(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "InvalidRequest") {
		t.Error("Expected InvalidRequest error code")
	}
}

func TestFileHandler_ServeHTTP_MethodNotAllowed(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := config.NewLogger("info")
	storage := newMockStorage()
	handler := NewFileHandler(storage, cfg, logger)

	// Test unsupported HTTP method
	req := httptest.NewRequest("POST", "/test.txt", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "MethodNotAllowed") {
		t.Error("Expected MethodNotAllowed error code")
	}
}

func TestFileHandler_GetContentType(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := config.NewLogger("info")
	storage := newMockStorage()
	handler := NewFileHandler(storage, cfg, logger)

	tests := []struct {
		path     string
		expected string
	}{
		{"test.html", "text/html"},
		{"test.htm", "text/html"},
		{"style.css", "text/css"},
		{"script.js", "application/javascript"},
		{"data.json", "application/json"},
		{"image.png", "image/png"},
		{"photo.jpg", "image/jpeg"},
		{"photo.jpeg", "image/jpeg"},
		{"animation.gif", "image/gif"},
		{"icon.svg", "image/svg+xml"},
		{"document.pdf", "application/pdf"},
		{"test.txt", "text/plain"},
		{"readme.md", "text/markdown"},
		{"config.xml", "application/xml"},
		{"data.csv", "text/csv"},
		{"archive.zip", "application/zip"},
		{"unknown.xyz", "application/octet-stream"},
		{"noextension", "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := handler.getContentType(tt.path)
			if result != tt.expected {
				t.Errorf("Expected content type %s for %s, got %s", tt.expected, tt.path, result)
			}
		})
	}
}

func TestFileHandler_SetS3Headers(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := config.NewLogger("info")
	storage := newMockStorage()
	handler := NewFileHandler(storage, cfg, logger)

	w := httptest.NewRecorder()
	etag := "test-etag"
	modTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	size := int64(100)
	path := "test.txt"

	handler.setS3Headers(w, etag, modTime, size, path, "")

	// Check x-amz-request-id header (should be set to some value)
	if w.Header().Get("x-amz-request-id") == "" {
		t.Error("Expected x-amz-request-id header to be set")
	}

	// Check x-amz-id-2 header (should be set to some value)
	if w.Header().Get("x-amz-id-2") == "" {
		t.Error("Expected x-amz-id-2 header to be set")
	}

	// Check ETag header
	expectedETag := `"test-etag"`
	if w.Header().Get("ETag") != expectedETag {
		t.Errorf("Expected ETag %s, got %s", expectedETag, w.Header().Get("ETag"))
	}

	// Check Content-Type header
	expectedContentType := "text/plain"
	if w.Header().Get("Content-Type") != expectedContentType {
		t.Errorf("Expected Content-Type %s, got %s", expectedContentType, w.Header().Get("Content-Type"))
	}

	// Check Accept-Ranges header
	if w.Header().Get("Accept-Ranges") != "bytes" {
		t.Error("Expected Accept-Ranges header to be 'bytes'")
	}

	// Check CORS headers
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("Expected CORS Allow-Origin header to be '*'")
	}
}

func TestFileHandler_HandleGetObject_WithS3Headers(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := config.NewLogger("info")
	storage := newMockStorage()
	handler := NewFileHandler(storage, cfg, logger)

	// Add test file
	content := []byte("Hello, World!")
	modTime := time.Now().Truncate(time.Second)
	storage.addFile("test.txt", content, modTime)

	// Test successful request
	req := httptest.NewRequest("GET", "/test.txt", nil)
	w := httptest.NewRecorder()

	handler.handleGetObject(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Check S3 headers are present
	if w.Header().Get("x-amz-request-id") == "" {
		t.Error("Expected x-amz-request-id header to be set")
	}

	if w.Header().Get("x-amz-id-2") == "" {
		t.Error("Expected x-amz-id-2 header to be set")
	}

	if w.Header().Get("Server") == "" {
		t.Error("Expected Server header to be set")
	}

	// Check Content-Type is properly detected
	expectedContentType := "text/plain"
	if w.Header().Get("Content-Type") != expectedContentType {
		t.Errorf("Expected Content-Type %s, got %s", expectedContentType, w.Header().Get("Content-Type"))
	}
}

func TestFileHandler_ConditionalRequest_IfModifiedSince(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := config.NewLogger("info")
	storage := newMockStorage()
	handler := NewFileHandler(storage, cfg, logger)

	// File modification time
	fileModTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	etag := "test-etag"

	tests := []struct {
		name        string
		imsTime     time.Time
		expected    bool
		description string
	}{
		{
			name:        "File modified after IMS",
			imsTime:     fileModTime.Add(-time.Hour),
			expected:    false,
			description: "File is newer, should not return 304",
		},
		{
			name:        "File not modified since IMS",
			imsTime:     fileModTime.Add(time.Hour),
			expected:    true,
			description: "File is older, should return 304",
		},
		{
			name:        "File modified at same time as IMS",
			imsTime:     fileModTime,
			expected:    true,
			description: "Same time, should return 304",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test.txt", nil)
			req.Header.Set("If-Modified-Since", tt.imsTime.Format(http.TimeFormat))

			result := handler.checkConditionalRequest(req, etag, fileModTime)
			if result != tt.expected {
				t.Errorf("%s: Expected %v, got %v", tt.description, tt.expected, result)
			}
		})
	}
}

func TestFileHandler_ConditionalRequest_InvalidIfModifiedSince(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := config.NewLogger("info")
	storage := newMockStorage()
	handler := NewFileHandler(storage, cfg, logger)

	fileModTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	etag := "test-etag"

	// Test with invalid If-Modified-Since header
	req := httptest.NewRequest("GET", "/test.txt", nil)
	req.Header.Set("If-Modified-Since", "invalid-date")

	result := handler.checkConditionalRequest(req, etag, fileModTime)
	if result != false {
		t.Error("Expected false for invalid If-Modified-Since header")
	}
}

func TestHealthHandler_ServeHTTP(t *testing.T) {
	logger := config.NewLogger("info")
	storage := newMockStorage()
	handler := NewHealthHandler(storage, logger)

	// Test GET request
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Header().Get("Content-Type") != "application/json" {
		t.Error("Expected JSON content type")
	}

	body := w.Body.String()
	if !strings.Contains(body, `"status":"healthy"`) {
		t.Error("Expected healthy status in response")
	}

	if !strings.Contains(body, `"timestamp"`) {
		t.Error("Expected timestamp in response")
	}
}

func TestHealthHandler_MethodNotAllowed(t *testing.T) {
	logger := config.NewLogger("info")
	storage := newMockStorage()
	handler := NewHealthHandler(storage, logger)

	// Test POST request (not allowed)
	req := httptest.NewRequest("POST", "/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestFileHandler_UsesStorageContentType(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := config.NewLogger("info")
	storage := newMockStorage()
	handler := NewFileHandler(storage, cfg, logger)

	// Add test file with specific Content-Type in storage
	content := []byte("webm-content")
	modTime := time.Now().Truncate(time.Second)
	// .webm is currently not in the extension list, so it would default to octet-stream
	// We want it to be video/webm from storage
	storage.addFileWithContentType("video.webm", content, modTime, "video/webm")

	req := httptest.NewRequest("GET", "/video.webm", nil)
	w := httptest.NewRecorder()

	handler.handleGetObject(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	expectedContentType := "video/webm"
	if w.Header().Get("Content-Type") != expectedContentType {
		t.Errorf("Expected Content-Type '%s', got '%s'", expectedContentType, w.Header().Get("Content-Type"))
	}
}
