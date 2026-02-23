package interfaces

import (
	"io"
	"time"
)

// FileInfo represents file metadata
type FileInfo struct {
	Path    string
	Size    int64
	ModTime time.Time
	IsDir   bool
	ETag    string // S3 object ETag if available
	ContentType string // Content type from storage metadata
}

// Storage defines the interface for file storage operations
type Storage interface {
	// GetFileInfo retrieves file metadata for the given path
	GetFileInfo(path string) (*FileInfo, error)

	// ReadFile reads the entire file content
	ReadFile(path string) ([]byte, error)

	// GetFileReader returns an io.ReadSeekCloser for the given path
	GetFileReader(path string) (io.ReadSeekCloser, error)

	// FileExists checks if a file exists at the given path
	FileExists(path string) bool
}
