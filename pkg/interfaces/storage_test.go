package interfaces

import (
	"testing"
	"time"
)

func TestFileInfo_Struct(t *testing.T) {
	// Test FileInfo struct creation and field access
	modTime := time.Now()
	fileInfo := &FileInfo{
		Path:    "/test/file.txt",
		Size:    1024,
		ModTime: modTime,
		IsDir:   false,
		ETag:    "test-etag-123",
	}

	if fileInfo.Path != "/test/file.txt" {
		t.Errorf("Expected path '/test/file.txt', got '%s'", fileInfo.Path)
	}

	if fileInfo.Size != 1024 {
		t.Errorf("Expected size 1024, got %d", fileInfo.Size)
	}

	if !fileInfo.ModTime.Equal(modTime) {
		t.Errorf("Expected modTime %v, got %v", modTime, fileInfo.ModTime)
	}

	if fileInfo.IsDir {
		t.Error("Expected IsDir to be false")
	}

	if fileInfo.ETag != "test-etag-123" {
		t.Errorf("Expected ETag 'test-etag-123', got '%s'", fileInfo.ETag)
	}
}

func TestFileInfo_ZeroValues(t *testing.T) {
	// Test FileInfo with zero values
	fileInfo := &FileInfo{}

	if fileInfo.Path != "" {
		t.Errorf("Expected empty path, got '%s'", fileInfo.Path)
	}

	if fileInfo.Size != 0 {
		t.Errorf("Expected size 0, got %d", fileInfo.Size)
	}

	if !fileInfo.ModTime.IsZero() {
		t.Errorf("Expected zero time, got %v", fileInfo.ModTime)
	}

	if fileInfo.IsDir {
		t.Error("Expected IsDir to be false")
	}

	if fileInfo.ETag != "" {
		t.Errorf("Expected empty ETag, got '%s'", fileInfo.ETag)
	}
}

func TestFileInfo_DirectoryInfo(t *testing.T) {
	// Test FileInfo for directory
	fileInfo := &FileInfo{
		Path:    "/test/directory/",
		Size:    0,
		ModTime: time.Now(),
		IsDir:   true,
		ETag:    "",
	}

	if !fileInfo.IsDir {
		t.Error("Expected IsDir to be true for directory")
	}

	if fileInfo.Size != 0 {
		t.Errorf("Expected directory size to be 0, got %d", fileInfo.Size)
	}

	if fileInfo.ETag != "" {
		t.Errorf("Expected empty ETag for directory, got '%s'", fileInfo.ETag)
	}
}

// MockStorage is a test implementation of the Storage interface
type MockStorage struct {
	files map[string]*FileInfo
	data  map[string][]byte
}

func NewMockStorage() *MockStorage {
	return &MockStorage{
		files: make(map[string]*FileInfo),
		data:  make(map[string][]byte),
	}
}

func (m *MockStorage) GetFileInfo(path string) (*FileInfo, error) {
	if info, exists := m.files[path]; exists {
		return info, nil
	}
	return nil, &MockError{message: "file not found"}
}

func (m *MockStorage) ReadFile(path string) ([]byte, error) {
	if data, exists := m.data[path]; exists {
		return data, nil
	}
	return nil, &MockError{message: "file not found"}
}

func (m *MockStorage) FileExists(path string) bool {
	_, exists := m.files[path]
	return exists
}

func (m *MockStorage) AddFile(path string, content []byte, modTime time.Time) {
	m.files[path] = &FileInfo{
		Path:    path,
		Size:    int64(len(content)),
		ModTime: modTime,
		IsDir:   false,
		ETag:    "mock-etag",
	}
	m.data[path] = content
}

type MockError struct {
	message string
}

func (e *MockError) Error() string {
	return e.message
}

func TestMockStorage_Implementation(t *testing.T) {
	// Test that MockStorage properly implements Storage interface
	var storage Storage = NewMockStorage()

	// Test with empty storage
	if storage.FileExists("nonexistent") {
		t.Error("FileExists should return false for non-existent file")
	}

	_, err := storage.GetFileInfo("nonexistent")
	if err == nil {
		t.Error("GetFileInfo should return error for non-existent file")
	}

	_, err = storage.ReadFile("nonexistent")
	if err == nil {
		t.Error("ReadFile should return error for non-existent file")
	}

	// Add a file and test
	mockStorage := storage.(*MockStorage)
	content := []byte("test content")
	modTime := time.Now()
	mockStorage.AddFile("test.txt", content, modTime)

	if !storage.FileExists("test.txt") {
		t.Error("FileExists should return true for existing file")
	}

	info, err := storage.GetFileInfo("test.txt")
	if err != nil {
		t.Errorf("GetFileInfo should not return error for existing file: %v", err)
	}

	if info.Path != "test.txt" {
		t.Errorf("Expected path 'test.txt', got '%s'", info.Path)
	}

	if info.Size != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), info.Size)
	}

	data, err := storage.ReadFile("test.txt")
	if err != nil {
		t.Errorf("ReadFile should not return error for existing file: %v", err)
	}

	if string(data) != string(content) {
		t.Errorf("Expected content '%s', got '%s'", string(content), string(data))
	}
}
