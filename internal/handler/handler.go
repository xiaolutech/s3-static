package handler

import (
	"crypto/rand"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"s3-static/internal/config"
	"s3-static/pkg/interfaces"
)

// FileHandler handles HTTP requests for static files
type FileHandler struct {
	storage interfaces.Storage
	config  *config.Config
	logger  *config.Logger
}

// NewFileHandler creates a new FileHandler instance
func NewFileHandler(storage interfaces.Storage, cfg *config.Config, logger *config.Logger) *FileHandler {
	return &FileHandler{
		storage: storage,
		config:  cfg,
		logger:  logger,
	}
}

// ServeHTTP handles HTTP requests
func (h *FileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleGetObject(w, r)
	default:
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "MethodNotAllowed", "Method not allowed")
	}
}

// handleGetObject handles GET requests for objects
func (h *FileHandler) handleGetObject(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "InvalidRequest", "Empty path")
		return
	}

	// Get file info from storage
	fileInfo, err := h.storage.GetFileInfo(path)
	if err != nil {
		h.handleStorageError(w, err, path)
		return
	}

	// Use ETag from storage (S3 provides this)
	etag := fileInfo.ETag

	// Check conditional requests
	if h.checkConditionalRequest(r, etag, fileInfo.ModTime) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	// Read file content
	content, err := h.storage.ReadFile(path)
	if err != nil {
		h.handleStorageError(w, err, path)
		return
	}

	// Set S3 compatible headers
	h.setS3Headers(w, etag, fileInfo.ModTime, fileInfo.Size, path)

	// Write response
	w.WriteHeader(http.StatusOK)
	w.Write(content)

	h.logger.Info("File served",
		"path", path,
		"size", fileInfo.Size,
		"etag", etag,
	)
}



// checkConditionalRequest checks if the request should return 304 Not Modified
func (h *FileHandler) checkConditionalRequest(r *http.Request, etag string, modTime time.Time) bool {
	// Check If-None-Match header
	if inm := r.Header.Get("If-None-Match"); inm != "" {
		// Handle both quoted and unquoted ETags
		if inm == etag || inm == `"`+etag+`"` || inm == "*" {
			return true
		}
	}

	// Check If-Modified-Since header
	if ims := r.Header.Get("If-Modified-Since"); ims != "" {
		if t, err := http.ParseTime(ims); err == nil {
			// If file modification time is not after the If-Modified-Since time,
			// then the file hasn't been modified since that time
			fileModTime := modTime.Truncate(time.Second)
			imsTime := t.Truncate(time.Second)
			if !fileModTime.After(imsTime) {
				return true
			}
		}
	}

	return false
}

// setS3Headers sets S3 compatible headers on the response
func (h *FileHandler) setS3Headers(w http.ResponseWriter, etag string, modTime time.Time, size int64, path string) {
	// S3 标准响应头
	w.Header().Set("x-amz-request-id", h.generateRequestID())
	w.Header().Set("x-amz-id-2", h.generateRequestID2())
	w.Header().Set("Server", "S3-Static/1.0")
	
	// 缓存相关头
	w.Header().Set("ETag", `"`+etag+`"`)
	w.Header().Set("Last-Modified", modTime.UTC().Format(http.TimeFormat))
	w.Header().Set("Cache-Control", fmt.Sprintf("max-age=%d", int(h.config.DefaultCacheDuration.Seconds())))
	
	// 内容相关头
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.Header().Set("Content-Type", h.getContentType(path))
	w.Header().Set("Accept-Ranges", "bytes")
	
	// CORS 支持（如果需要）
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD")
	w.Header().Set("Access-Control-Allow-Headers", "Range")
	w.Header().Set("Access-Control-Expose-Headers", "ETag, Last-Modified, Content-Length")
}

// getContentType determines the content type based on file extension
func (h *FileHandler) getContentType(path string) string {
	// Handle files without extensions
	if !strings.Contains(path, ".") {
		return "application/octet-stream"
	}
	
	// Simple content type detection based on file extension
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
	default:
		return "application/octet-stream"
	}
}

// handleStorageError handles storage-related errors
func (h *FileHandler) handleStorageError(w http.ResponseWriter, err error, path string) {
	// This would use the error mapping from storage package
	h.writeErrorResponse(w, http.StatusInternalServerError, "InternalError", err.Error())
}

// generateRequestID generates a unique request ID for x-amz-request-id
func (h *FileHandler) generateRequestID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%X", b)
}

// generateRequestID2 generates a unique request ID for x-amz-id-2
func (h *FileHandler) generateRequestID2() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%X", b)
}

// writeErrorResponse writes an S3-compatible error response
func (h *FileHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, code, message string) {
	// 设置 S3 标准错误响应头
	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("x-amz-request-id", h.generateRequestID())
	w.Header().Set("x-amz-id-2", h.generateRequestID2())
	
	w.WriteHeader(statusCode)
	
	errorXML := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Error>
    <Code>%s</Code>
    <Message>%s</Message>
    <RequestId>%s</RequestId>
</Error>`, code, message, h.generateRequestID())
	
	w.Write([]byte(errorXML))
}

// HealthHandler handles health check requests
type HealthHandler struct {
	storage interfaces.Storage
	logger  *config.Logger
}

// NewHealthHandler creates a new HealthHandler instance
func NewHealthHandler(storage interfaces.Storage, logger *config.Logger) *HealthHandler {
	return &HealthHandler{
		storage: storage,
		logger:  logger,
	}
}

// ServeHTTP handles health check requests
func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Simple health check - could be enhanced to check storage connectivity
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"healthy","timestamp":"` + time.Now().UTC().Format(time.RFC3339) + `"}`))
}