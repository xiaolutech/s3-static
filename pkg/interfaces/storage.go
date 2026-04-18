package interfaces

import (
	"context"
	"io"
	"time"
)

// FileInfo represents file metadata
type FileInfo struct {
	Path        string
	Size        int64
	ModTime     time.Time
	IsDir       bool
	ETag        string // S3 object ETag if available
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

// ContextStorage is an optional extension that allows storage backends
// to honor request cancellation and deadlines.
type ContextStorage interface {
	GetFileInfoContext(ctx context.Context, path string) (*FileInfo, error)
	ReadFileContext(ctx context.Context, path string) ([]byte, error)
	GetFileReaderContext(ctx context.Context, path string) (io.ReadSeekCloser, error)
	FileExistsContext(ctx context.Context, path string) bool
}

// OpenedFile bundles a reader with its metadata.
type OpenedFile struct {
	Info   *FileInfo
	Reader io.ReadSeekCloser
}

// FileOpener is an optional extension that allows a backend to fetch
// a file reader and metadata in one call.
type FileOpener interface {
	OpenFileContext(ctx context.Context, path string) (*OpenedFile, error)
}
