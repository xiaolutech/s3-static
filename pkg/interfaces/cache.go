package interfaces

import (
	"net/http"
	"time"
)

// ETagGenerator defines the interface for ETag generation and validation
type ETagGenerator interface {
	// Generate creates an ETag based on file metadata
	Generate(filePath string, modTime time.Time, size int64) string
	
	// Validate checks if the provided ETag matches the current file state
	Validate(etag string, filePath string, modTime time.Time, size int64) bool
}

// CacheManager defines the interface for cache management operations
type CacheManager interface {
	// GenerateETag creates an ETag for the given file
	GenerateETag(filePath string, fileInfo *FileInfo) string
	
	// CheckConditionalRequest checks if the request should return 304 Not Modified
	CheckConditionalRequest(r *http.Request, etag string, modTime time.Time) bool
	
	// SetCacheHeaders sets appropriate cache headers on the response
	SetCacheHeaders(w http.ResponseWriter, etag string, modTime time.Time)
}