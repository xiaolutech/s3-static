package testutils

import (
	"errors"
	"strings"
	"sync"
	"time"

	"s3-static/pkg/interfaces"
)

// MockStorage provides a configurable mock implementation of interfaces.Storage
type MockStorage struct {
	mu           sync.RWMutex
	files        map[string]*interfaces.FileInfo
	data         map[string][]byte
	errors       map[string]error
	callCounts   map[string]int
	shouldFail   bool
	failureError error
}

// NewMockStorage creates a new MockStorage instance
func NewMockStorage() *MockStorage {
	return &MockStorage{
		files:      make(map[string]*interfaces.FileInfo),
		data:       make(map[string][]byte),
		errors:     make(map[string]error),
		callCounts: make(map[string]int),
	}
}

// GetFileInfo implements interfaces.Storage
func (m *MockStorage) GetFileInfo(path string) (*interfaces.FileInfo, error) {
	m.mu.Lock()
	m.callCounts["GetFileInfo"]++
	shouldFail := m.shouldFail
	failureError := m.failureError
	m.mu.Unlock()

	if shouldFail {
		return nil, failureError
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if err, exists := m.errors[path]; exists {
		return nil, err
	}

	if info, exists := m.files[path]; exists {
		return info, nil
	}

	return nil, errors.New("file not found")
}

// ReadFile implements interfaces.Storage
func (m *MockStorage) ReadFile(path string) ([]byte, error) {
	m.mu.Lock()
	m.callCounts["ReadFile"]++
	shouldFail := m.shouldFail
	failureError := m.failureError
	m.mu.Unlock()

	if shouldFail {
		return nil, failureError
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if err, exists := m.errors[path]; exists {
		return nil, err
	}

	if data, exists := m.data[path]; exists {
		return data, nil
	}

	return nil, errors.New("file not found")
}

// FileExists implements interfaces.Storage
func (m *MockStorage) FileExists(path string) bool {
	m.mu.Lock()
	m.callCounts["FileExists"]++
	shouldFail := m.shouldFail
	m.mu.Unlock()

	if shouldFail {
		return false
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.files[path]
	return exists
}



func detectMockContentType(path string) string {
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
	case "pdf":
		return "application/pdf"
	case "webm":
		return "video/webm"
	default:
		return "application/octet-stream"
	}
}

// AddFile adds a file to the mock storage
func (m *MockStorage) AddFile(path string, content []byte, modTime time.Time, etag string) {
	if etag == "" {
		etag = "mock-etag-" + path
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.files[path] = &interfaces.FileInfo{
		Path:        path,
		Size:        int64(len(content)),
		ModTime:     modTime,
		IsDir:       false,
		ETag:        etag,
		ContentType: detectMockContentType(path),
	}
	m.data[path] = content
}

// AddDirectory adds a directory to the mock storage
func (m *MockStorage) AddDirectory(path string, modTime time.Time) {
	m.files[path] = &interfaces.FileInfo{
		Path:    path,
		Size:    0,
		ModTime: modTime,
		IsDir:   true,
		ETag:    "",
	}
}

// SetError sets an error to be returned for a specific path
func (m *MockStorage) SetError(path string, err error) {
	m.errors[path] = err
}

// SetGlobalFailure makes all operations fail with the given error
func (m *MockStorage) SetGlobalFailure(err error) {
	m.shouldFail = true
	m.failureError = err
}

// ClearGlobalFailure clears the global failure state
func (m *MockStorage) ClearGlobalFailure() {
	m.shouldFail = false
	m.failureError = nil
}

// RemoveFile removes a file from the mock storage
func (m *MockStorage) RemoveFile(path string) {
	delete(m.files, path)
	delete(m.data, path)
	delete(m.errors, path)
}

// Clear removes all files and errors from the mock storage
func (m *MockStorage) Clear() {
	m.files = make(map[string]*interfaces.FileInfo)
	m.data = make(map[string][]byte)
	m.errors = make(map[string]error)
	m.callCounts = make(map[string]int)
}

// GetCallCount returns the number of times a method was called
func (m *MockStorage) GetCallCount(method string) int {
	return m.callCounts[method]
}

// GetAllCallCounts returns all call counts
func (m *MockStorage) GetAllCallCounts() map[string]int {
	result := make(map[string]int)
	for k, v := range m.callCounts {
		result[k] = v
	}
	return result
}

// ResetCallCounts resets all call counters
func (m *MockStorage) ResetCallCounts() {
	m.callCounts = make(map[string]int)
}

// GetFileCount returns the number of files in storage
func (m *MockStorage) GetFileCount() int {
	return len(m.files)
}

// GetFileList returns a list of all file paths
func (m *MockStorage) GetFileList() []string {
	paths := make([]string, 0, len(m.files))
	for path := range m.files {
		paths = append(paths, path)
	}
	return paths
}

// UpdateFileContent updates the content of an existing file
func (m *MockStorage) UpdateFileContent(path string, content []byte) {
	if info, exists := m.files[path]; exists {
		info.Size = int64(len(content))
		info.ModTime = time.Now()
		m.data[path] = content
	}
}

// UpdateFileETag updates the ETag of an existing file
func (m *MockStorage) UpdateFileETag(path string, etag string) {
	if info, exists := m.files[path]; exists {
		info.ETag = etag
	}
}

// MockError represents a mock error for testing
type MockError struct {
	Message string
	Code    string
}

// Error implements the error interface
func (e *MockError) Error() string {
	return e.Message
}

// GetCode returns the error code
func (e *MockError) GetCode() string {
	return e.Code
}

// NewMockError creates a new mock error
func NewMockError(message, code string) *MockError {
	return &MockError{
		Message: message,
		Code:    code,
	}
}

// Common mock errors
var (
	ErrMockNotFound     = NewMockError("file not found", "NotFound")
	ErrMockForbidden    = NewMockError("access denied", "Forbidden")
	ErrMockInternal     = NewMockError("internal server error", "InternalError")
	ErrMockBadRequest   = NewMockError("bad request", "BadRequest")
	ErrMockTimeout      = NewMockError("request timeout", "Timeout")
	ErrMockNetworkError = NewMockError("network error", "NetworkError")
)

// StorageBuilder provides a fluent interface for building mock storage
type StorageBuilder struct {
	storage *MockStorage
}

// NewStorageBuilder creates a new storage builder
func NewStorageBuilder() *StorageBuilder {
	return &StorageBuilder{
		storage: NewMockStorage(),
	}
}

// WithFile adds a file to the storage
func (sb *StorageBuilder) WithFile(path, content string) *StorageBuilder {
	sb.storage.AddFile(path, []byte(content), time.Now(), "")
	return sb
}

// WithFileAndETag adds a file with a specific ETag
func (sb *StorageBuilder) WithFileAndETag(path, content, etag string) *StorageBuilder {
	sb.storage.AddFile(path, []byte(content), time.Now(), etag)
	return sb
}

// WithFileAndTime adds a file with a specific modification time
func (sb *StorageBuilder) WithFileAndTime(path, content string, modTime time.Time) *StorageBuilder {
	sb.storage.AddFile(path, []byte(content), modTime, "")
	return sb
}

// WithDirectory adds a directory to the storage
func (sb *StorageBuilder) WithDirectory(path string) *StorageBuilder {
	sb.storage.AddDirectory(path, time.Now())
	return sb
}

// WithError sets an error for a specific path
func (sb *StorageBuilder) WithError(path string, err error) *StorageBuilder {
	sb.storage.SetError(path, err)
	return sb
}

// WithGlobalFailure sets a global failure
func (sb *StorageBuilder) WithGlobalFailure(err error) *StorageBuilder {
	sb.storage.SetGlobalFailure(err)
	return sb
}

// Build returns the configured mock storage
func (sb *StorageBuilder) Build() *MockStorage {
	return sb.storage
}

// TestFileInfo creates a test FileInfo with default values
func TestFileInfo(path string, size int64) *interfaces.FileInfo {
	return &interfaces.FileInfo{
		Path:    path,
		Size:    size,
		ModTime: time.Now(),
		IsDir:   false,
		ETag:    "test-etag-" + path,
	}
}

// TestDirectoryInfo creates a test FileInfo for a directory
func TestDirectoryInfo(path string) *interfaces.FileInfo {
	return &interfaces.FileInfo{
		Path:    path,
		Size:    0,
		ModTime: time.Now(),
		IsDir:   true,
		ETag:    "",
	}
}

// AssertionHelper provides common test assertions
type AssertionHelper struct{}

// NewAssertionHelper creates a new assertion helper
func NewAssertionHelper() *AssertionHelper {
	return &AssertionHelper{}
}

// AssertFileInfo asserts that two FileInfo structs are equal
func (ah *AssertionHelper) AssertFileInfo(t TestingT, actual, expected *interfaces.FileInfo) {
	if actual.Path != expected.Path {
		t.Errorf("Path mismatch: expected %s, got %s", expected.Path, actual.Path)
	}
	if actual.Size != expected.Size {
		t.Errorf("Size mismatch: expected %d, got %d", expected.Size, actual.Size)
	}
	if actual.IsDir != expected.IsDir {
		t.Errorf("IsDir mismatch: expected %t, got %t", expected.IsDir, actual.IsDir)
	}
	if actual.ETag != expected.ETag {
		t.Errorf("ETag mismatch: expected %s, got %s", expected.ETag, actual.ETag)
	}
}

// TestingT is a minimal interface for testing
type TestingT interface {
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
	Helper()
}
