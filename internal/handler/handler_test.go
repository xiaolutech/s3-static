package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"s3-static/internal/storage"
	"strings"
	"testing"
	"time"

	"s3-static/internal/config"
	"s3-static/pkg/interfaces"

	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
)

// mockStorage implements interfaces.Storage for testing
type mockStorage struct {
	files map[string]*interfaces.FileInfo
	data  map[string][]byte
}

type openFileMockStorage struct {
	*mockStorage
	openCalls int
	infoCalls int
	readCalls int
}

func newMockStorage() *mockStorage {
	return &mockStorage{
		files: make(map[string]*interfaces.FileInfo),
		data:  make(map[string][]byte),
	}
}

func detectTestContentType(path string) string {
	ext := strings.ToLower(path[strings.LastIndex(path, ".")+1:])
	switch ext {
	case "html", "htm":
		return "text/html"
	case "css":
		return "text/css"
	case "js":
		return "application/javascript"
	case "json":
		return "application/json"
	case "txt":
		return "text/plain"
	case "md":
		return "text/markdown"
	case "xml":
		return "application/xml"
	case "csv":
		return "text/csv"
	case "zip":
		return "application/zip"
	case "png":
		return "image/png"
	case "jpg", "jpeg":
		return "image/jpeg"
	case "gif":
		return "image/gif"
	case "svg":
		return "image/svg+xml"
	case "webp":
		return "image/webp"
	case "bmp":
		return "image/bmp"
	case "tif", "tiff":
		return "image/tiff"
	case "mp4":
		return "video/mp4"
	case "mov":
		return "video/quicktime"
	case "m4v":
		return "video/x-m4v"
	case "pdf":
		return "application/pdf"
	// specific for the new test case, ensuring it doesn't conflict
	case "webm":
		return "video/webm"
	default:
		return "application/octet-stream"
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

type mockReadSeekCloser struct {
	*bytes.Reader
}

func (m *mockReadSeekCloser) Close() error {
	return nil
}

func (m *mockStorage) GetFileReader(path string) (io.ReadSeekCloser, error) {
	if data, exists := m.data[path]; exists {
		return &mockReadSeekCloser{bytes.NewReader(data)}, nil
	}
	return nil, &storage.StorageError{Type: storage.ErrorNotFound, Message: "file not found"}
}

func (m *mockStorage) FileExists(path string) bool {
	_, exists := m.files[path]
	return exists
}

func (m *openFileMockStorage) GetFileInfo(path string) (*interfaces.FileInfo, error) {
	m.infoCalls++
	return m.mockStorage.GetFileInfo(path)
}

func (m *openFileMockStorage) GetFileReader(path string) (io.ReadSeekCloser, error) {
	m.readCalls++
	return m.mockStorage.GetFileReader(path)
}

func (m *openFileMockStorage) OpenFileContext(_ context.Context, path string) (*interfaces.OpenedFile, error) {
	m.openCalls++
	info, err := m.mockStorage.GetFileInfo(path)
	if err != nil {
		return nil, err
	}
	reader, err := m.mockStorage.GetFileReader(path)
	if err != nil {
		return nil, err
	}
	return &interfaces.OpenedFile{Info: info, Reader: reader}, nil
}

func (m *mockStorage) addFile(path string, content []byte, modTime time.Time) {
	m.files[path] = &interfaces.FileInfo{
		Path:    path,
		Size:    int64(len(content)),
		ModTime: modTime,
		IsDir:   false,
		ETag:    "test-etag",
		// Simulate S3 behavior: auto-detect content type on upload
		ContentType: detectTestContentType(path),
	}
	m.data[path] = content
}

func (m *mockStorage) addFileWithContentType(path string, content []byte, modTime time.Time, contentType string) {
	m.addFile(path, content, modTime)
	m.files[path].ContentType = contentType
}

func (m *mockStorage) addFileWithMetadata(path string, content []byte, modTime time.Time, etag, contentType string, metadata map[string]string) {
	m.addFileWithContentType(path, content, modTime, contentType)
	m.files[path].ETag = etag
	if metadata != nil {
		m.files[path].Metadata = make(map[string]string, len(metadata))
		for key, value := range metadata {
			m.files[path].Metadata[strings.ToLower(key)] = value
		}
	}
}

func optimizedTestConfig() *config.Config {
	cfg := config.DefaultConfig()
	cfg.OptimizedImageEnabled = true
	cfg.OptimizedBucketName = "optimized"
	cfg.OptimizedMinBytes = 0
	return cfg
}

func addTrustedAVIFFile(storage *mockStorage, source *interfaces.FileInfo, content []byte, modTime time.Time, etag, profile string) {
	key := avifOptimizedKey(source.Path, profile)
	storage.addFileWithMetadata(key, content, modTime, etag, "image/avif", trustedAVIFMetadata(source, profile, nil))
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

	handler.setS3Headers(w, etag, modTime, size, path, "text/plain")

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

func TestFileHandler_ServeHTTP_HEAD(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := config.NewLogger("info")
	storage := newMockStorage()
	handler := NewFileHandler(storage, cfg, logger)

	content := []byte("Hello, HEAD!")
	modTime := time.Now().Truncate(time.Second)
	storage.addFile("head.txt", content, modTime)

	req := httptest.NewRequest(http.MethodHead, "/head.txt", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Fatalf("Expected empty body for HEAD, got %q", w.Body.String())
	}
	if w.Header().Get("Content-Length") != "12" {
		t.Fatalf("Expected Content-Length 12, got %s", w.Header().Get("Content-Length"))
	}
	if w.Header().Get("ETag") == "" {
		t.Fatal("Expected ETag header for HEAD response")
	}
}

func TestFileHandler_ServeHTTP_HEADIncludesMediaMetadataHeaders(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := config.NewLogger("info")
	storage := newMockStorage()
	handler := NewFileHandler(storage, cfg, logger)

	var pngBuffer bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	if err := png.Encode(&pngBuffer, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	modTime := time.Now().UTC().Truncate(time.Second)
	storage.addFileWithContentType("image.png", pngBuffer.Bytes(), modTime, "image/png")

	req := httptest.NewRequest(http.MethodHead, "/image.png", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Fatalf("Expected empty body for HEAD, got %q", w.Body.String())
	}
	if got := w.Header().Get(mediaWidthHeader); got != "3" {
		t.Fatalf("Expected %s 3, got %s", mediaWidthHeader, got)
	}
	if got := w.Header().Get(mediaHeightHeader); got != "2" {
		t.Fatalf("Expected %s 2, got %s", mediaHeightHeader, got)
	}
}

func TestFileHandler_ServeHTTP_GETIncludesMediaMetadataHeadersAndBody(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := config.NewLogger("info")
	storage := newMockStorage()
	handler := NewFileHandler(storage, cfg, logger)

	var pngBuffer bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	if err := png.Encode(&pngBuffer, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	modTime := time.Now().UTC().Truncate(time.Second)
	storage.addFileWithContentType("image.png", pngBuffer.Bytes(), modTime, "image/png")

	req := httptest.NewRequest(http.MethodGet, "/image.png", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}
	if got := w.Header().Get(mediaWidthHeader); got != "3" {
		t.Fatalf("Expected %s 3, got %s", mediaWidthHeader, got)
	}
	if got := w.Header().Get(mediaHeightHeader); got != "2" {
		t.Fatalf("Expected %s 2, got %s", mediaHeightHeader, got)
	}
	if !bytes.Equal(w.Body.Bytes(), pngBuffer.Bytes()) {
		t.Fatal("Expected GET response body to remain the original image bytes")
	}
}

func TestFileHandler_RangeRequest(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := config.NewLogger("info")
	storage := newMockStorage()
	handler := NewFileHandler(storage, cfg, logger)

	content := []byte("Hello, Range!")
	modTime := time.Now().Truncate(time.Second)
	storage.addFile("range.txt", content, modTime)

	req := httptest.NewRequest(http.MethodGet, "/range.txt", nil)
	req.Header.Set("Range", "bytes=0-4")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusPartialContent {
		t.Fatalf("Expected status 206, got %d", w.Code)
	}
	if w.Body.String() != "Hello" {
		t.Fatalf("Expected partial body Hello, got %q", w.Body.String())
	}
	if got := w.Header().Get("Content-Range"); got != "bytes 0-4/13" {
		t.Fatalf("Expected Content-Range bytes 0-4/13, got %s", got)
	}
}

func TestFileHandler_InvalidRangeRequest(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := config.NewLogger("info")
	storage := newMockStorage()
	handler := NewFileHandler(storage, cfg, logger)

	content := []byte("Hello, Range!")
	modTime := time.Now().Truncate(time.Second)
	storage.addFile("invalid-range.txt", content, modTime)

	req := httptest.NewRequest(http.MethodGet, "/invalid-range.txt", nil)
	req.Header.Set("Range", "bytes=100-200")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Fatalf("Expected status 416, got %d", w.Code)
	}
}

func TestFileHandler_NotModifiedStillReturnsValidators(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := config.NewLogger("info")
	storage := newMockStorage()
	handler := NewFileHandler(storage, cfg, logger)

	content := []byte("Hello, Cache!")
	modTime := time.Now().Truncate(time.Second)
	storage.addFile("cache.txt", content, modTime)

	req := httptest.NewRequest(http.MethodGet, "/cache.txt", nil)
	req.Header.Set("If-None-Match", `"test-etag"`)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotModified {
		t.Fatalf("Expected status 304, got %d", w.Code)
	}
	if w.Header().Get("ETag") != `"test-etag"` {
		t.Fatalf("Expected ETag header on 304, got %s", w.Header().Get("ETag"))
	}
	if w.Header().Get("Last-Modified") == "" {
		t.Fatal("Expected Last-Modified header on 304")
	}
}

func TestFileHandler_ErrorRequestIDConsistency(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := config.NewLogger("info")
	storage := newMockStorage()
	handler := NewFileHandler(storage, cfg, logger)

	req := httptest.NewRequest(http.MethodGet, "/missing.txt", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	requestID := w.Header().Get("x-amz-request-id")
	if requestID == "" {
		t.Fatal("Expected x-amz-request-id header")
	}
	if !strings.Contains(w.Body.String(), "<RequestId>"+requestID+"</RequestId>") {
		t.Fatalf("Expected response body to reuse header request id %s, got %s", requestID, w.Body.String())
	}
}

func TestFileHandler_UsesOpenFileWhenAvailable(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := config.NewLogger("info")
	base := newMockStorage()
	content := []byte("Hello, OpenFile!")
	modTime := time.Now().Truncate(time.Second)
	base.addFile("open.txt", content, modTime)
	storage := &openFileMockStorage{mockStorage: base}
	handler := NewFileHandler(storage, cfg, logger)

	req := httptest.NewRequest(http.MethodGet, "/open.txt", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}
	if storage.openCalls != 1 {
		t.Fatalf("Expected OpenFileContext to be called once, got %d", storage.openCalls)
	}
	if storage.infoCalls != 0 {
		t.Fatalf("Expected GetFileInfo fallback not to be used, got %d", storage.infoCalls)
	}
	if storage.readCalls != 0 {
		t.Fatalf("Expected GetFileReader fallback not to be used, got %d", storage.readCalls)
	}
}

func TestFileHandler_OptimizedImageHit(t *testing.T) {
	cfg := optimizedTestConfig()
	logger := config.NewLogger("info")
	source := newMockStorage()
	optimizedBase := newMockStorage()
	optimized := &openFileMockStorage{mockStorage: optimizedBase}
	handler := NewFileHandlerWithOptimizedStorage(source, optimized, cfg, logger)

	sourceTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	optimizedTime := sourceTime.Add(time.Minute)
	source.addFileWithMetadata("photo.png", []byte("original image"), sourceTime, "source-etag", "image/png", nil)
	sourceInfo := source.files["photo.png"]
	key := avifOptimizedKey(sourceInfo.Path, cfg.OptimizationProfile)
	optimizedBase.addFileWithMetadata(key, []byte("avif image"), optimizedTime, "optimized-etag", "image/avif", trustedAVIFMetadata(sourceInfo, cfg.OptimizationProfile, nil))

	req := httptest.NewRequest(http.MethodGet, "/photo.png", nil)
	req.Header.Set("Accept", "image/avif,image/webp,image/*,*/*")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}
	if w.Body.String() != "avif image" {
		t.Fatalf("Expected AVIF body, got %q", w.Body.String())
	}
	if got := w.Header().Get(optimizedStatusHeader); got != "hit; format=avif" {
		t.Fatalf("Expected optimized hit header, got %q", got)
	}
	if got := w.Header().Get("Content-Type"); got != "image/avif" {
		t.Fatalf("Expected AVIF content type, got %q", got)
	}
	if got := w.Header().Get("Vary"); got != "Accept" {
		t.Fatalf("Expected Vary Accept, got %q", got)
	}
	if got := w.Header().Get("ETag"); got != `"optimized-etag"` {
		t.Fatalf("Expected optimized ETag, got %s", got)
	}
	if optimized.openCalls != 1 {
		t.Fatalf("Expected optimized storage to be opened once, got %d", optimized.openCalls)
	}
}

func TestFileHandler_OptimizedImageRequiresAVIFAccept(t *testing.T) {
	cfg := optimizedTestConfig()
	logger := config.NewLogger("info")
	source := newMockStorage()
	optimizedBase := newMockStorage()
	optimized := &openFileMockStorage{mockStorage: optimizedBase}
	handler := NewFileHandlerWithOptimizedStorage(source, optimized, cfg, logger)

	modTime := time.Now().UTC().Truncate(time.Second)
	source.addFileWithMetadata("photo.png", []byte("original image"), modTime, "source-etag", "image/png", nil)
	sourceInfo := source.files["photo.png"]
	key := avifOptimizedKey(sourceInfo.Path, cfg.OptimizationProfile)
	optimizedBase.addFileWithMetadata(key, []byte("avif image"), modTime, "optimized-etag", "image/avif", trustedAVIFMetadata(sourceInfo, cfg.OptimizationProfile, nil))

	req := httptest.NewRequest(http.MethodGet, "/photo.png", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}
	if w.Body.String() != "original image" {
		t.Fatalf("Expected original body, got %q", w.Body.String())
	}
	if got := w.Header().Get(optimizedStatusHeader); got != "" {
		t.Fatalf("Expected no optimized status header, got %q", got)
	}
	if optimized.openCalls != 0 || optimized.infoCalls != 0 || optimized.readCalls != 0 {
		t.Fatalf("Expected optimized storage not to be used, got open=%d info=%d read=%d", optimized.openCalls, optimized.infoCalls, optimized.readCalls)
	}
}

func TestFileHandler_OptimizedImageFallbackStatuses(t *testing.T) {
	logger := config.NewLogger("info")
	sourceTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name             string
		optimizedETag    string
		optimizedProfile string
		expectedStatus   string
	}{
		{
			name:           "missing optimized object",
			expectedStatus: "miss",
		},
		{
			name:             "stale source etag",
			optimizedETag:    "old-source-etag",
			optimizedProfile: "v1-jpeg82-png-best-w1920",
			expectedStatus:   "stale",
		},
		{
			name:             "profile mismatch",
			optimizedETag:    "source-etag",
			optimizedProfile: "v0-old-profile",
			expectedStatus:   "profile-mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := optimizedTestConfig()
			source := newMockStorage()
			optimizedBase := newMockStorage()
			optimized := &openFileMockStorage{mockStorage: optimizedBase}
			handler := NewFileHandlerWithOptimizedStorage(source, optimized, cfg, logger)

			source.addFileWithMetadata("photo.png", []byte("original image"), sourceTime, "source-etag", "image/png", nil)
			sourceInfo := source.files["photo.png"]
			if tt.optimizedETag != "" {
				key := avifOptimizedKey(sourceInfo.Path, cfg.OptimizationProfile)
				optimizedBase.addFileWithMetadata(key, []byte("avif image"), sourceTime.Add(time.Minute), "optimized-etag", "image/avif", trustedAVIFMetadata(sourceInfo, cfg.OptimizationProfile, map[string]string{
					optimizedSourceETagMetadata: tt.optimizedETag,
					optimizedProfileMetadata:    tt.optimizedProfile,
				}))
			}

			req := httptest.NewRequest(http.MethodGet, "/photo.png", nil)
			req.Header.Set("Accept", "image/avif")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("Expected status 200, got %d", w.Code)
			}
			if w.Body.String() != "original image" {
				t.Fatalf("Expected original body, got %q", w.Body.String())
			}
			if got := w.Header().Get(optimizedStatusHeader); got != tt.expectedStatus {
				t.Fatalf("Expected optimized status %q, got %q", tt.expectedStatus, got)
			}
			if got := w.Header().Get("ETag"); got != `"source-etag"` {
				t.Fatalf("Expected source ETag, got %s", got)
			}
			if optimized.openCalls != 1 {
				t.Fatalf("Expected optimized storage to be opened once, got %d", optimized.openCalls)
			}
		})
	}
}

func TestFileHandler_OptimizedImageSkipsRangeRequest(t *testing.T) {
	cfg := optimizedTestConfig()
	logger := config.NewLogger("info")
	source := newMockStorage()
	optimizedBase := newMockStorage()
	optimized := &openFileMockStorage{mockStorage: optimizedBase}
	handler := NewFileHandlerWithOptimizedStorage(source, optimized, cfg, logger)

	modTime := time.Now().UTC().Truncate(time.Second)
	source.addFileWithMetadata("photo.jpg", []byte("Hello, Range!"), modTime, "source-etag", "image/jpeg", nil)
	addTrustedAVIFFile(optimizedBase, source.files["photo.jpg"], []byte("avif image"), modTime, "optimized-etag", cfg.OptimizationProfile)

	req := httptest.NewRequest(http.MethodGet, "/photo.jpg", nil)
	req.Header.Set("Accept", "image/avif")
	req.Header.Set("Range", "bytes=0-4")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusPartialContent {
		t.Fatalf("Expected status 206, got %d", w.Code)
	}
	if w.Body.String() != "Hello" {
		t.Fatalf("Expected source range body, got %q", w.Body.String())
	}
	if got := w.Header().Get(optimizedStatusHeader); got != "" {
		t.Fatalf("Expected no optimized status header, got %q", got)
	}
	if optimized.openCalls != 0 || optimized.infoCalls != 0 || optimized.readCalls != 0 {
		t.Fatalf("Expected optimized storage not to be used, got open=%d info=%d read=%d", optimized.openCalls, optimized.infoCalls, optimized.readCalls)
	}
}

func TestFileHandler_OptimizedImageSkipsHeadAndMetadata(t *testing.T) {
	logger := config.NewLogger("info")
	modTime := time.Now().UTC().Truncate(time.Second)

	tests := []struct {
		name   string
		method string
		url    string
		assert func(t *testing.T, w *httptest.ResponseRecorder)
	}{
		{
			name:   "head",
			method: http.MethodHead,
			url:    "/photo.jpg",
			assert: func(t *testing.T, w *httptest.ResponseRecorder) {
				t.Helper()
				if w.Code != http.StatusOK {
					t.Fatalf("Expected status 200, got %d", w.Code)
				}
				if w.Body.Len() != 0 {
					t.Fatalf("Expected empty HEAD body, got %q", w.Body.String())
				}
				if got := w.Header().Get("ETag"); got != `"source-etag"` {
					t.Fatalf("Expected source ETag, got %s", got)
				}
			},
		},
		{
			name:   "metadata",
			method: http.MethodGet,
			url:    "/photo.jpg?meta=1",
			assert: func(t *testing.T, w *httptest.ResponseRecorder) {
				t.Helper()
				if w.Code != http.StatusOK {
					t.Fatalf("Expected status 200, got %d", w.Code)
				}
				var response fileMetadataResponse
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if response.ETag != "source-etag" {
					t.Fatalf("Expected source ETag in metadata, got %s", response.ETag)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := optimizedTestConfig()
			source := newMockStorage()
			optimizedBase := newMockStorage()
			optimized := &openFileMockStorage{mockStorage: optimizedBase}
			handler := NewFileHandlerWithOptimizedStorage(source, optimized, cfg, logger)

			source.addFileWithMetadata("photo.jpg", []byte("original image"), modTime, "source-etag", "image/jpeg", nil)
			addTrustedAVIFFile(optimizedBase, source.files["photo.jpg"], []byte("avif image"), modTime, "optimized-etag", cfg.OptimizationProfile)

			req := httptest.NewRequest(tt.method, tt.url, nil)
			req.Header.Set("Accept", "image/avif")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			tt.assert(t, w)
			if got := w.Header().Get(optimizedStatusHeader); got != "" {
				t.Fatalf("Expected no optimized status header, got %q", got)
			}
			if optimized.openCalls != 0 || optimized.infoCalls != 0 || optimized.readCalls != 0 {
				t.Fatalf("Expected optimized storage not to be used, got open=%d info=%d read=%d", optimized.openCalls, optimized.infoCalls, optimized.readCalls)
			}
		})
	}
}

func TestFileHandler_OptimizedImageSkipsNonImage(t *testing.T) {
	cfg := optimizedTestConfig()
	logger := config.NewLogger("info")
	source := newMockStorage()
	optimizedBase := newMockStorage()
	optimized := &openFileMockStorage{mockStorage: optimizedBase}
	handler := NewFileHandlerWithOptimizedStorage(source, optimized, cfg, logger)

	modTime := time.Now().UTC().Truncate(time.Second)
	source.addFileWithMetadata("document.pdf", []byte("original pdf"), modTime, "source-etag", "application/pdf", nil)
	addTrustedAVIFFile(optimizedBase, source.files["document.pdf"], []byte("avif pdf"), modTime, "optimized-etag", cfg.OptimizationProfile)

	req := httptest.NewRequest(http.MethodGet, "/document.pdf", nil)
	req.Header.Set("Accept", "image/avif")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}
	if w.Body.String() != "original pdf" {
		t.Fatalf("Expected source body, got %q", w.Body.String())
	}
	if got := w.Header().Get(optimizedStatusHeader); got != "" {
		t.Fatalf("Expected no optimized status header, got %q", got)
	}
	if optimized.openCalls != 0 || optimized.infoCalls != 0 || optimized.readCalls != 0 {
		t.Fatalf("Expected optimized storage not to be used, got open=%d info=%d read=%d", optimized.openCalls, optimized.infoCalls, optimized.readCalls)
	}
}

func TestFileHandler_OptimizedImageConditionalsUseSourceETag(t *testing.T) {
	cfg := optimizedTestConfig()
	logger := config.NewLogger("info")
	modTime := time.Now().UTC().Truncate(time.Second)

	t.Run("optimized etag does not drive not-modified response", func(t *testing.T) {
		source := newMockStorage()
		optimizedBase := newMockStorage()
		optimized := &openFileMockStorage{mockStorage: optimizedBase}
		handler := NewFileHandlerWithOptimizedStorage(source, optimized, cfg, logger)

		source.addFileWithMetadata("photo.jpg", []byte("original image"), modTime, "source-etag", "image/jpeg", nil)
		addTrustedAVIFFile(optimizedBase, source.files["photo.jpg"], []byte("avif image"), modTime, "optimized-etag", cfg.OptimizationProfile)

		req := httptest.NewRequest(http.MethodGet, "/photo.jpg", nil)
		req.Header.Set("Accept", "image/avif")
		req.Header.Set("If-None-Match", `"optimized-etag"`)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}
		if w.Body.String() != "avif image" {
			t.Fatalf("Expected AVIF body, got %q", w.Body.String())
		}
		if got := w.Header().Get(optimizedStatusHeader); got != "hit; format=avif" {
			t.Fatalf("Expected optimized hit header, got %q", got)
		}
	})

	t.Run("source etag drives not-modified response", func(t *testing.T) {
		source := newMockStorage()
		optimizedBase := newMockStorage()
		optimized := &openFileMockStorage{mockStorage: optimizedBase}
		handler := NewFileHandlerWithOptimizedStorage(source, optimized, cfg, logger)

		source.addFileWithMetadata("photo.jpg", []byte("original image"), modTime, "source-etag", "image/jpeg", nil)
		addTrustedAVIFFile(optimizedBase, source.files["photo.jpg"], []byte("avif image"), modTime, "optimized-etag", cfg.OptimizationProfile)

		req := httptest.NewRequest(http.MethodGet, "/photo.jpg", nil)
		req.Header.Set("Accept", "image/avif")
		req.Header.Set("If-None-Match", `"source-etag"`)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotModified {
			t.Fatalf("Expected status 304, got %d", w.Code)
		}
		if optimized.openCalls != 0 {
			t.Fatalf("Expected optimized storage not to be opened after source 304, got %d", optimized.openCalls)
		}
	})

	t.Run("if match source etag permits optimized response", func(t *testing.T) {
		source := newMockStorage()
		optimizedBase := newMockStorage()
		optimized := &openFileMockStorage{mockStorage: optimizedBase}
		handler := NewFileHandlerWithOptimizedStorage(source, optimized, cfg, logger)

		source.addFileWithMetadata("photo.jpg", []byte("original image"), modTime, "source-etag", "image/jpeg", nil)
		addTrustedAVIFFile(optimizedBase, source.files["photo.jpg"], []byte("avif image"), modTime, "optimized-etag", cfg.OptimizationProfile)

		req := httptest.NewRequest(http.MethodGet, "/photo.jpg", nil)
		req.Header.Set("Accept", "image/avif")
		req.Header.Set("If-Match", `"source-etag"`)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", w.Code)
		}
		if w.Body.String() != "avif image" {
			t.Fatalf("Expected AVIF body, got %q", w.Body.String())
		}
	})

	t.Run("if match optimized etag does not permit response", func(t *testing.T) {
		source := newMockStorage()
		optimizedBase := newMockStorage()
		optimized := &openFileMockStorage{mockStorage: optimizedBase}
		handler := NewFileHandlerWithOptimizedStorage(source, optimized, cfg, logger)

		source.addFileWithMetadata("photo.jpg", []byte("original image"), modTime, "source-etag", "image/jpeg", nil)
		addTrustedAVIFFile(optimizedBase, source.files["photo.jpg"], []byte("avif image"), modTime, "optimized-etag", cfg.OptimizationProfile)

		req := httptest.NewRequest(http.MethodGet, "/photo.jpg", nil)
		req.Header.Set("Accept", "image/avif")
		req.Header.Set("If-Match", `"optimized-etag"`)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusPreconditionFailed {
			t.Fatalf("Expected status 412, got %d", w.Code)
		}
		if optimized.openCalls != 0 {
			t.Fatalf("Expected optimized storage not to be opened after source 412, got %d", optimized.openCalls)
		}
	})

	t.Run("weak if none match source etag drives not-modified response", func(t *testing.T) {
		source := newMockStorage()
		optimizedBase := newMockStorage()
		optimized := &openFileMockStorage{mockStorage: optimizedBase}
		handler := NewFileHandlerWithOptimizedStorage(source, optimized, cfg, logger)

		source.addFileWithMetadata("photo.jpg", []byte("original image"), modTime, "source-etag", "image/jpeg", nil)
		addTrustedAVIFFile(optimizedBase, source.files["photo.jpg"], []byte("avif image"), modTime, "optimized-etag", cfg.OptimizationProfile)

		req := httptest.NewRequest(http.MethodGet, "/photo.jpg", nil)
		req.Header.Set("Accept", "image/avif")
		req.Header.Set("If-None-Match", `"other", W/"source-etag"`)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotModified {
			t.Fatalf("Expected status 304, got %d", w.Code)
		}
		if optimized.openCalls != 0 {
			t.Fatalf("Expected optimized storage not to be opened after source 304, got %d", optimized.openCalls)
		}
	})
}

func TestFileHandler_GetMetadataForRasterImages(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := config.NewLogger("info")
	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})

	tests := []struct {
		name        string
		path        string
		contentType string
		encode      func(*bytes.Buffer) error
		width       int
		height      int
	}{
		{
			name:        "png",
			path:        "image.png",
			contentType: "image/png",
			encode:      func(buf *bytes.Buffer) error { return png.Encode(buf, img) },
			width:       3,
			height:      2,
		},
		{
			name:        "jpeg",
			path:        "image.jpg",
			contentType: "image/jpeg",
			encode:      func(buf *bytes.Buffer) error { return jpeg.Encode(buf, img, nil) },
			width:       3,
			height:      2,
		},
		{
			name:        "gif",
			path:        "image.gif",
			contentType: "image/gif",
			encode:      func(buf *bytes.Buffer) error { return gif.Encode(buf, img, nil) },
			width:       3,
			height:      2,
		},
		{
			name:        "bmp",
			path:        "image.bmp",
			contentType: "image/bmp",
			encode:      func(buf *bytes.Buffer) error { return bmp.Encode(buf, img) },
			width:       3,
			height:      2,
		},
		{
			name:        "tiff",
			path:        "image.tiff",
			contentType: "image/tiff",
			encode:      func(buf *bytes.Buffer) error { return tiff.Encode(buf, img, nil) },
			width:       3,
			height:      2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := newMockStorage()
			storage := &openFileMockStorage{mockStorage: base}
			handler := NewFileHandler(storage, cfg, logger)

			var encoded bytes.Buffer
			if err := tt.encode(&encoded); err != nil {
				t.Fatalf("encode %s: %v", tt.name, err)
			}

			modTime := time.Now().UTC().Truncate(time.Second)
			base.addFileWithContentType(tt.path, encoded.Bytes(), modTime, tt.contentType)

			req := httptest.NewRequest(http.MethodGet, "/"+tt.path+"?meta=1", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("Expected status 200, got %d", w.Code)
			}
			if got := w.Header().Get("Content-Type"); got != "application/json" {
				t.Fatalf("Expected json content type, got %s", got)
			}

			var response fileMetadataResponse
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if response.Path != tt.path {
				t.Fatalf("Expected path %s, got %s", tt.path, response.Path)
			}
			if response.Width == nil || *response.Width != tt.width {
				t.Fatalf("Expected width %d, got %#v", tt.width, response.Width)
			}
			if response.Height == nil || *response.Height != tt.height {
				t.Fatalf("Expected height %d, got %#v", tt.height, response.Height)
			}
			if response.ContentType != tt.contentType {
				t.Fatalf("Expected %s content type, got %s", tt.contentType, response.ContentType)
			}
			if storage.readCalls != 1 {
				t.Fatalf("Expected one reader call, got %d", storage.readCalls)
			}
		})
	}
}

func TestFileHandler_GetMetadataForNonImageDoesNotReadBody(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := config.NewLogger("info")
	base := newMockStorage()
	storage := &openFileMockStorage{mockStorage: base}
	handler := NewFileHandler(storage, cfg, logger)

	modTime := time.Now().UTC().Truncate(time.Second)
	base.addFileWithContentType("notes.txt", []byte("hello"), modTime, "text/plain")

	req := httptest.NewRequest(http.MethodGet, "/notes.txt?meta=1", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var response fileMetadataResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Width != nil || response.Height != nil {
		t.Fatalf("Expected nil dimensions, got width=%v height=%v", response.Width, response.Height)
	}
	if storage.readCalls != 0 {
		t.Fatalf("Expected no reader calls, got %d", storage.readCalls)
	}
}

func TestFileHandler_GetMetadataForSVG(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := config.NewLogger("info")
	storage := newMockStorage()
	handler := NewFileHandler(storage, cfg, logger)

	modTime := time.Now().UTC().Truncate(time.Second)
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0, 0, 120, 80"></svg>`)
	storage.addFileWithContentType("vector", svg, modTime, "image/svg+xml; charset=utf-8")

	req := httptest.NewRequest(http.MethodGet, "/vector?meta=1", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var response fileMetadataResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Width == nil || *response.Width != 120 {
		t.Fatalf("Expected width 120, got %#v", response.Width)
	}
	if response.Height == nil || *response.Height != 80 {
		t.Fatalf("Expected height 80, got %#v", response.Height)
	}
}

func TestFileHandler_GetMetadataForWebP(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := config.NewLogger("info")
	storage := newMockStorage()
	handler := NewFileHandler(storage, cfg, logger)

	const webpBase64 = "UklGRrIBAABXRUJQVlA4TKUBAAAvSsAYAA8w//M///MfeJAkbXvaSG7m8Q3GfYSBJekwQztm/IcZlgwnmWImn2BK7aFmBtnVir6q//8VOkFE/xm4baTIu8c48ArEo6+B3zFKYln3pqClSCKX0begFTAXFOLXHSyF8cCNcZEG4OywuA4KVVfJCiArU7GAgJI8+lJP/OKMT/fBAjevg1cYB7YVkFuWga2lyPi5I0HFy5YTpWIHg0RZpkniRVW9odHAKOwosWuOGdxIyn2OvaCDvhg/we6TwadPBPbqBV58MsLmMJ8yZnOWk8SRz4N+QoyPL+MnamzMvcE1rHNEr91F9GKZPVUcS9w7PhhH36suB9qPeYb/oLk6cuTiJ0wOK3m5h1cKjW6EVZCYMK7dxcKCBdgP9HkKr9gkAO2P8GKZGWVdIAatQa+1IDpt6qyorVwdy01xdW8Jkfk6xjEXmVQQ+HQdFr6OKhIN34dXWq0+0qr6EJSCeeVLH9+gvGTLyqM65PQ44ihzlTXxQKjKbAvshXgir7Lil9w4L2bvMycmjQcqXaMCO6BlY28i+FOLzbfI1vEqxAhotocAAA=="

	webpBytes, err := base64.StdEncoding.DecodeString(webpBase64)
	if err != nil {
		t.Fatalf("decode webp base64: %v", err)
	}

	modTime := time.Now().UTC().Truncate(time.Second)
	storage.addFileWithContentType("sample.webp", webpBytes, modTime, "image/webp")

	req := httptest.NewRequest(http.MethodGet, "/sample.webp?meta=1", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var response fileMetadataResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Width == nil || *response.Width != 75 {
		t.Fatalf("Expected width 75, got %#v", response.Width)
	}
	if response.Height == nil || *response.Height != 100 {
		t.Fatalf("Expected height 100, got %#v", response.Height)
	}
	if response.ContentType != "image/webp" {
		t.Fatalf("Expected image/webp content type, got %s", response.ContentType)
	}
}

func TestFileHandler_GetMetadataForMP4Video(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := config.NewLogger("info")
	storage := newMockStorage()
	handler := NewFileHandler(storage, cfg, logger)

	modTime := time.Now().UTC().Truncate(time.Second)
	storage.addFileWithContentType("clip.mp4", makeMP4WithTrackDimensions(1920, 1080), modTime, "video/mp4")

	req := httptest.NewRequest(http.MethodGet, "/clip.mp4?meta=1", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var response fileMetadataResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Width == nil || *response.Width != 1920 {
		t.Fatalf("Expected width 1920, got %#v", response.Width)
	}
	if response.Height == nil || *response.Height != 1080 {
		t.Fatalf("Expected height 1080, got %#v", response.Height)
	}
	if response.ContentType != "video/mp4" {
		t.Fatalf("Expected video/mp4 content type, got %s", response.ContentType)
	}
}

func makeMP4WithTrackDimensions(width, height int) []byte {
	tkhdPayload := make([]byte, 84)
	binary.BigEndian.PutUint32(tkhdPayload[76:80], uint32(width)<<16)
	binary.BigEndian.PutUint32(tkhdPayload[80:84], uint32(height)<<16)
	return mp4Box("moov", mp4Box("trak", mp4Box("tkhd", tkhdPayload)))
}

func mp4Box(boxType string, payload []byte) []byte {
	box := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint32(box[0:4], uint32(len(box)))
	copy(box[4:8], boxType)
	copy(box[8:], payload)
	return box
}
