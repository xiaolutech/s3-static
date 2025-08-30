package interfaces

import (
	"time"
)

// FileInfo represents file metadata
type FileInfo struct {
	Path    string
	Size    int64
	ModTime time.Time
	IsDir   bool
}

// Storage defines the interface for file storage operations
type Storage interface {
	// GetFileInfo retrieves file metadata for the given path
	GetFileInfo(path string) (*FileInfo, error)
	
	// ReadFile reads the entire file content
	ReadFile(path string) ([]byte, error)
	
	// FileExists checks if a file exists at the given path
	FileExists(path string) bool
}