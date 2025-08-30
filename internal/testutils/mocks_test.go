package testutils

import (
	"errors"
	"testing"
	"time"
)

func TestMockStorage_BasicOperations(t *testing.T) {
	storage := NewMockStorage()

	// Test empty storage
	if storage.FileExists("nonexistent.txt") {
		t.Error("FileExists should return false for empty storage")
	}

	_, err := storage.GetFileInfo("nonexistent.txt")
	if err == nil {
		t.Error("GetFileInfo should return error for non-existent file")
	}

	_, err = storage.ReadFile("nonexistent.txt")
	if err == nil {
		t.Error("ReadFile should return error for non-existent file")
	}

	// Add a file
	content := []byte("test content")
	modTime := time.Now()
	storage.AddFile("test.txt", content, modTime, "test-etag")

	// Test file operations
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

	if info.ETag != "test-etag" {
		t.Errorf("Expected ETag 'test-etag', got '%s'", info.ETag)
	}

	data, err := storage.ReadFile("test.txt")
	if err != nil {
		t.Errorf("ReadFile should not return error for existing file: %v", err)
	}

	if string(data) != string(content) {
		t.Errorf("Expected content '%s', got '%s'", string(content), string(data))
	}
}

func TestMockStorage_CallCounting(t *testing.T) {
	storage := NewMockStorage()
	storage.AddFile("test.txt", []byte("content"), time.Now(), "etag")

	// Initial call counts should be zero
	if storage.GetCallCount("GetFileInfo") != 0 {
		t.Error("Initial GetFileInfo call count should be 0")
	}

	// Make some calls
	storage.GetFileInfo("test.txt")
	storage.GetFileInfo("nonexistent.txt")
	storage.ReadFile("test.txt")
	storage.FileExists("test.txt")

	// Check call counts
	if storage.GetCallCount("GetFileInfo") != 2 {
		t.Errorf("Expected GetFileInfo call count 2, got %d", storage.GetCallCount("GetFileInfo"))
	}

	if storage.GetCallCount("ReadFile") != 1 {
		t.Errorf("Expected ReadFile call count 1, got %d", storage.GetCallCount("ReadFile"))
	}

	if storage.GetCallCount("FileExists") != 1 {
		t.Errorf("Expected FileExists call count 1, got %d", storage.GetCallCount("FileExists"))
	}

	// Reset and verify
	storage.ResetCallCounts()
	if storage.GetCallCount("GetFileInfo") != 0 {
		t.Error("Call count should be 0 after reset")
	}
}

func TestMockStorage_ErrorHandling(t *testing.T) {
	storage := NewMockStorage()

	// Set specific error for a path
	testErr := errors.New("specific error")
	storage.SetError("error.txt", testErr)

	_, err := storage.GetFileInfo("error.txt")
	if err != testErr {
		t.Errorf("Expected specific error, got %v", err)
	}

	// Set global failure
	globalErr := errors.New("global failure")
	storage.SetGlobalFailure(globalErr)

	_, err = storage.GetFileInfo("any.txt")
	if err != globalErr {
		t.Errorf("Expected global error, got %v", err)
	}

	if storage.FileExists("any.txt") {
		t.Error("FileExists should return false during global failure")
	}

	// Clear global failure
	storage.ClearGlobalFailure()
	storage.AddFile("test.txt", []byte("content"), time.Now(), "etag")

	if !storage.FileExists("test.txt") {
		t.Error("FileExists should work after clearing global failure")
	}
}

func TestStorageBuilder(t *testing.T) {
	storage := NewStorageBuilder().
		WithFile("file1.txt", "content1").
		WithFileAndETag("file2.txt", "content2", "custom-etag").
		WithDirectory("dir1").
		WithError("error.txt", errors.New("test error")).
		Build()

	// Test file1
	if !storage.FileExists("file1.txt") {
		t.Error("file1.txt should exist")
	}

	data, err := storage.ReadFile("file1.txt")
	if err != nil || string(data) != "content1" {
		t.Error("file1.txt should have correct content")
	}

	// Test file2 with custom ETag
	info, err := storage.GetFileInfo("file2.txt")
	if err != nil {
		t.Errorf("file2.txt should exist: %v", err)
	}

	if info.ETag != "custom-etag" {
		t.Errorf("Expected custom ETag, got %s", info.ETag)
	}

	// Test directory
	dirInfo, err := storage.GetFileInfo("dir1")
	if err != nil {
		t.Errorf("dir1 should exist: %v", err)
	}

	if !dirInfo.IsDir {
		t.Error("dir1 should be marked as directory")
	}

	// Test error
	_, err = storage.GetFileInfo("error.txt")
	if err == nil {
		t.Error("error.txt should return error")
	}
}

func TestMockError(t *testing.T) {
	err := NewMockError("test message", "TestCode")

	if err.Error() != "test message" {
		t.Errorf("Expected message 'test message', got '%s'", err.Error())
	}

	if err.GetCode() != "TestCode" {
		t.Errorf("Expected code 'TestCode', got '%s'", err.GetCode())
	}
}

func TestPredefinedErrors(t *testing.T) {
	errors := []*MockError{
		ErrMockNotFound,
		ErrMockForbidden,
		ErrMockInternal,
		ErrMockBadRequest,
		ErrMockTimeout,
		ErrMockNetworkError,
	}

	for _, err := range errors {
		if err.Error() == "" {
			t.Error("Predefined error should have non-empty message")
		}

		if err.GetCode() == "" {
			t.Error("Predefined error should have non-empty code")
		}
	}
}

func TestMockStorage_FileManagement(t *testing.T) {
	storage := NewMockStorage()

	// Add multiple files
	storage.AddFile("file1.txt", []byte("content1"), time.Now(), "etag1")
	storage.AddFile("file2.txt", []byte("content2"), time.Now(), "etag2")
	storage.AddDirectory("dir1", time.Now())

	// Check file count
	if storage.GetFileCount() != 3 {
		t.Errorf("Expected 3 files, got %d", storage.GetFileCount())
	}

	// Get file list
	files := storage.GetFileList()
	if len(files) != 3 {
		t.Errorf("Expected 3 files in list, got %d", len(files))
	}

	// Update file content
	newContent := []byte("updated content")
	storage.UpdateFileContent("file1.txt", newContent)

	data, err := storage.ReadFile("file1.txt")
	if err != nil {
		t.Errorf("ReadFile failed: %v", err)
	}

	if string(data) != string(newContent) {
		t.Errorf("Expected updated content, got %s", string(data))
	}

	// Update ETag
	storage.UpdateFileETag("file1.txt", "new-etag")
	info, err := storage.GetFileInfo("file1.txt")
	if err != nil {
		t.Errorf("GetFileInfo failed: %v", err)
	}

	if info.ETag != "new-etag" {
		t.Errorf("Expected updated ETag, got %s", info.ETag)
	}

	// Remove file
	storage.RemoveFile("file1.txt")
	if storage.FileExists("file1.txt") {
		t.Error("file1.txt should not exist after removal")
	}

	if storage.GetFileCount() != 2 {
		t.Errorf("Expected 2 files after removal, got %d", storage.GetFileCount())
	}

	// Clear all
	storage.Clear()
	if storage.GetFileCount() != 0 {
		t.Errorf("Expected 0 files after clear, got %d", storage.GetFileCount())
	}
}

func TestTestFileInfo(t *testing.T) {
	info := TestFileInfo("test.txt", 1024)

	if info.Path != "test.txt" {
		t.Errorf("Expected path 'test.txt', got '%s'", info.Path)
	}

	if info.Size != 1024 {
		t.Errorf("Expected size 1024, got %d", info.Size)
	}

	if info.IsDir {
		t.Error("Expected IsDir to be false")
	}

	if info.ETag != "test-etag-test.txt" {
		t.Errorf("Expected ETag 'test-etag-test.txt', got '%s'", info.ETag)
	}
}

func TestTestDirectoryInfo(t *testing.T) {
	info := TestDirectoryInfo("testdir")

	if info.Path != "testdir" {
		t.Errorf("Expected path 'testdir', got '%s'", info.Path)
	}

	if info.Size != 0 {
		t.Errorf("Expected size 0, got %d", info.Size)
	}

	if !info.IsDir {
		t.Error("Expected IsDir to be true")
	}

	if info.ETag != "" {
		t.Errorf("Expected empty ETag, got '%s'", info.ETag)
	}
}
