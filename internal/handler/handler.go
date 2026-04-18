package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	pathpkg "path"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"s3-static/internal/config"
	"s3-static/internal/storage"
	"s3-static/pkg/interfaces"
)

const storageRequestTimeout = 30 * time.Second

var requestIDCounter atomic.Uint64

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
	case http.MethodGet, http.MethodHead:
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
	if cleanedPath := strings.TrimPrefix(pathpkg.Clean("/"+path), "/"); cleanedPath != path {
		h.writeErrorResponse(w, http.StatusBadRequest, "InvalidRequest", "Invalid path")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), storageRequestTimeout)
	defer cancel()

	fileInfo, err := h.getFileInfo(ctx, path)
	if err != nil {
		h.handleStorageError(w, err, path)
		return
	}

	etag := fileInfo.ETag
	h.setConditionalHeaders(w, etag, fileInfo.ModTime, path)

	if h.checkConditionalRequest(r, etag, fileInfo.ModTime) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	stream, err := h.getFileReader(ctx, path)
	if err != nil {
		h.handleStorageError(w, err, path)
		return
	}
	defer stream.Close()

	h.setS3Headers(w, etag, fileInfo.ModTime, fileInfo.Size, path, fileInfo.ContentType)
	http.ServeContent(w, r, path, fileInfo.ModTime, stream)

	h.logger.Debug("File served",
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

func (h *FileHandler) getFileInfo(ctx context.Context, path string) (*interfaces.FileInfo, error) {
	if storageWithContext, ok := h.storage.(interfaces.ContextStorage); ok {
		return storageWithContext.GetFileInfoContext(ctx, path)
	}
	return h.storage.GetFileInfo(path)
}

func (h *FileHandler) getFileReader(ctx context.Context, path string) (io.ReadSeekCloser, error) {
	if storageWithContext, ok := h.storage.(interfaces.ContextStorage); ok {
		return storageWithContext.GetFileReaderContext(ctx, path)
	}
	return h.storage.GetFileReader(path)
}

func (h *FileHandler) setConditionalHeaders(w http.ResponseWriter, etag string, modTime time.Time, path string) {
	w.Header().Set("ETag", `"`+etag+`"`)
	w.Header().Set("Last-Modified", modTime.UTC().Format(http.TimeFormat))
	h.setCacheControlHeader(w, path)
}

// setS3Headers sets S3 compatible headers on the response
func (h *FileHandler) setS3Headers(w http.ResponseWriter, etag string, modTime time.Time, size int64, path string, contentType string) {
	// S3 标准响应头
	w.Header().Set("x-amz-request-id", h.generateRequestID())
	w.Header().Set("x-amz-id-2", h.generateRequestID2())
	w.Header().Set("Server", "S3-Static/1.0")

	h.setConditionalHeaders(w, etag, modTime, path)

	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	w.Header().Set("Accept-Ranges", "bytes")

	// CORS 支持（如果需要）
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD")
	w.Header().Set("Access-Control-Allow-Headers", "Range")
	w.Header().Set("Access-Control-Expose-Headers", "ETag, Last-Modified, Content-Length")
}

// setCacheControlHeader sets the appropriate Cache-Control header based on strategy
func (h *FileHandler) setCacheControlHeader(w http.ResponseWriter, path string) {
	switch h.config.CacheStrategy {
	case "no-cache":
		// 最佳实践：可变内容总是验证缓存
		// 浏览器会发送条件请求 (If-None-Match/If-Modified-Since)
		// 如果内容未变化，服务器返回 304 Not Modified
		w.Header().Set("Cache-Control", "no-cache")

	case "max-age":
		// 传统方式：使用 max-age（不推荐用于可变内容）
		w.Header().Set("Cache-Control", fmt.Sprintf("max-age=%d", int(h.config.DefaultCacheDuration.Seconds())))

	case "immutable":
		// 适用于永不变化的内容（如带版本号的静态资源）
		// 浏览器在 max-age 期间内完全不会发送请求
		w.Header().Set("Cache-Control", fmt.Sprintf("max-age=%d, immutable", int(h.config.DefaultCacheDuration.Seconds())))

	default:
		// 默认使用 no-cache（最安全的选择）
		w.Header().Set("Cache-Control", "no-cache")
	}
}



// handleStorageError handles storage-related errors
func (h *FileHandler) handleStorageError(w http.ResponseWriter, err error, path string) {
	if storage.IsNotFound(err) {
		h.logger.Warn("Object not found", "path", path)
		h.writeErrorResponse(w, http.StatusNotFound, "NoSuchKey", "The specified key does not exist.")
		return
	}

	h.logger.Error("Storage error", "path", path, "error", err)
	h.writeErrorResponse(w, http.StatusInternalServerError, "InternalError", err.Error())
}

// generateRequestID generates a fast unique request ID.
func (h *FileHandler) generateRequestID() string {
	id := requestIDCounter.Add(1)
	return strings.ToUpper(strconv.FormatUint(id, 16))
}

// generateRequestID2 generates a secondary request ID derived from time and counter.
func (h *FileHandler) generateRequestID2() string {
	id := requestIDCounter.Add(1)
	return fmt.Sprintf("%X%016X", time.Now().UnixNano(), id)
}

// writeErrorResponse writes an S3-compatible error response
func (h *FileHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, code, message string) {
	requestID := h.generateRequestID()

	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("x-amz-request-id", requestID)
	w.Header().Set("x-amz-id-2", h.generateRequestID2())

	w.WriteHeader(statusCode)

	errorXML := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Error>
    <Code>%s</Code>
    <Message>%s</Message>
    <RequestId>%s</RequestId>
</Error>`, code, message, requestID)

	_, _ = w.Write([]byte(errorXML))
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
