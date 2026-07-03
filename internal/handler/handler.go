package handler

import (
	"context"
	"encoding/json"
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

const (
	storageRequestTimeout = 30 * time.Second

	optimizedSourceETagMetadata = "source-etag"
	optimizedProfileMetadata    = "optimization-profile"
	optimizedStatusHeader       = "X-S3-Static-Optimized"
)

var requestIDCounter atomic.Uint64

// FileHandler handles HTTP requests for static files
type FileHandler struct {
	storage           interfaces.Storage
	optimizedResolver *OptimizedVariantResolver
	config            *config.Config
	logger            *config.Logger
}

// NewFileHandler creates a new FileHandler instance
func NewFileHandler(storage interfaces.Storage, cfg *config.Config, logger *config.Logger) *FileHandler {
	return NewFileHandlerWithOptimizedStorage(storage, nil, cfg, logger)
}

// NewFileHandlerWithOptimizedStorage creates a new FileHandler instance with an optional optimized image storage backend.
func NewFileHandlerWithOptimizedStorage(storage interfaces.Storage, optimized interfaces.Storage, cfg *config.Config, logger *config.Logger) *FileHandler {
	var resolver *OptimizedVariantResolver
	if optimized != nil {
		resolver = NewOptimizedVariantResolver(optimized, cfg)
	}
	return &FileHandler{
		storage:           storage,
		optimizedResolver: resolver,
		config:            cfg,
		logger:            logger,
	}
}

// ServeHTTP handles HTTP requests
func (h *FileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		if r.Method == http.MethodGet && shouldServeMetadata(r) {
			h.handleGetMetadata(w, r)
			return
		}
		h.handleGetObject(w, r)
	default:
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "MethodNotAllowed", "Method not allowed")
	}
}

// handleGetObject handles GET requests for objects
func (h *FileHandler) handleGetObject(w http.ResponseWriter, r *http.Request) {
	path, ok := validateRequestPath(r.URL.Path)
	if !ok {
		if path == "" {
			h.writeErrorResponse(w, http.StatusBadRequest, "InvalidRequest", "Empty path")
			return
		}
		h.writeErrorResponse(w, http.StatusBadRequest, "InvalidRequest", "Invalid path")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), storageRequestTimeout)
	defer cancel()

	var sourceInfo *interfaces.FileInfo
	if h.hasConditionalHeaders(r) || h.optimizedResolver.NeedsSourceInfo(r) {
		var err error
		sourceInfo, err = h.getFileInfo(ctx, path)
		if err != nil {
			h.handleStorageError(w, err, path)
			return
		}
	}

	if h.hasConditionalHeaders(r) {
		etag := sourceInfo.ETag
		h.setConditionalHeaders(w, etag, sourceInfo.ModTime, path)
		if status, handled := h.sourceConditionalStatus(r, path, sourceInfo); handled {
			w.WriteHeader(status)
			return
		}
	}

	if sourceInfo != nil && h.optimizedResolver != nil {
		optimizedFile, status := h.optimizedResolver.Resolve(ctx, r, sourceInfo)
		if header := status.HeaderValue(); header != "" {
			w.Header().Set(optimizedStatusHeader, header)
		}
		if optimizedFile != nil {
			defer func() {
				if optimizedFile.Reader != nil {
					optimizedFile.Reader.Close()
				}
			}()

			w.Header().Set("Vary", "Accept")
			h.serveOpenedFile(w, r, path, optimizedFile)
			h.logger.Debug("File served",
				"path", path,
				"size", optimizedFile.Info.Size,
				"etag", optimizedFile.Info.ETag,
				"optimized", true,
			)
			return
		}
	}

	openedFile, err := h.openFile(ctx, path)
	if err != nil {
		h.handleStorageError(w, err, path)
		return
	}
	defer func() {
		if openedFile.Reader != nil {
			openedFile.Reader.Close()
		}
	}()

	h.serveOpenedFile(w, r, path, openedFile)

	h.logger.Debug("File served",
		"path", path,
		"size", openedFile.Info.Size,
		"etag", openedFile.Info.ETag,
	)
}

func (h *FileHandler) handleGetMetadata(w http.ResponseWriter, r *http.Request) {
	path, ok := validateRequestPath(r.URL.Path)
	if !ok {
		if path == "" {
			writeJSONError(w, http.StatusBadRequest, "Empty path")
			return
		}
		writeJSONError(w, http.StatusBadRequest, "Invalid path")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), storageRequestTimeout)
	defer cancel()

	fileInfo, err := h.getFileInfo(ctx, path)
	if err != nil {
		h.handleMetadataStorageError(w, err, path)
		return
	}

	metadata := fileMetadataResponse{
		Path:         path,
		ContentType:  fileInfo.ContentType,
		Size:         fileInfo.Size,
		ETag:         fileInfo.ETag,
		LastModified: fileInfo.ModTime.UTC(),
	}

	if shouldProbeMediaDimensions(path, fileInfo.ContentType) {
		reader, err := h.getFileReader(ctx, path)
		if err != nil {
			h.handleMetadataStorageError(w, err, path)
			return
		}
		defer reader.Close()

		width, height, format, err := extractMediaMetadata(path, fileInfo.ContentType, reader)
		if err != nil {
			h.logger.Debug("Media metadata probe skipped",
				"path", path,
				"error", err,
			)
		} else {
			metadata.Width = &width
			metadata.Height = &height
			if metadata.ContentType == "" {
				metadata.ContentType = detectContentTypeFromFormat(format)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Type")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(metadata); err != nil {
		h.logger.Error("Failed to write metadata response", "path", path, "error", err)
	}
}

func (h *FileHandler) hasConditionalHeaders(r *http.Request) bool {
	return r.Header.Get("If-Match") != "" ||
		r.Header.Get("If-None-Match") != "" ||
		r.Header.Get("If-Unmodified-Since") != "" ||
		r.Header.Get("If-Modified-Since") != ""
}

// checkConditionalRequest checks if the request should return 304 Not Modified
func (h *FileHandler) checkConditionalRequest(r *http.Request, etag string, modTime time.Time) bool {
	status, handled := h.sourceConditionalStatus(r, "", &interfaces.FileInfo{ETag: etag, ModTime: modTime})
	return handled && status == http.StatusNotModified
}

func (h *FileHandler) getFileInfo(ctx context.Context, path string) (*interfaces.FileInfo, error) {
	return h.getFileInfoFromStorage(ctx, h.storage, path)
}

func (h *FileHandler) getFileInfoFromStorage(ctx context.Context, backend interfaces.Storage, path string) (*interfaces.FileInfo, error) {
	if storageWithContext, ok := backend.(interfaces.ContextStorage); ok {
		return storageWithContext.GetFileInfoContext(ctx, path)
	}
	return backend.GetFileInfo(path)
}

func (h *FileHandler) getFileReader(ctx context.Context, path string) (io.ReadSeekCloser, error) {
	return h.getFileReaderFromStorage(ctx, h.storage, path)
}

func (h *FileHandler) getFileReaderFromStorage(ctx context.Context, backend interfaces.Storage, path string) (io.ReadSeekCloser, error) {
	if storageWithContext, ok := backend.(interfaces.ContextStorage); ok {
		return storageWithContext.GetFileReaderContext(ctx, path)
	}
	return backend.GetFileReader(path)
}

func (h *FileHandler) openFile(ctx context.Context, path string) (*interfaces.OpenedFile, error) {
	return h.openFileFromStorage(ctx, h.storage, path)
}

func (h *FileHandler) openFileFromStorage(ctx context.Context, backend interfaces.Storage, path string) (*interfaces.OpenedFile, error) {
	if opener, ok := backend.(interfaces.FileOpener); ok {
		return opener.OpenFileContext(ctx, path)
	}

	fileInfo, err := h.getFileInfoFromStorage(ctx, backend, path)
	if err != nil {
		return nil, err
	}

	reader, err := h.getFileReaderFromStorage(ctx, backend, path)
	if err != nil {
		return nil, err
	}

	return &interfaces.OpenedFile{
		Info:   fileInfo,
		Reader: reader,
	}, nil
}

func (h *FileHandler) serveOpenedFile(w http.ResponseWriter, r *http.Request, path string, openedFile *interfaces.OpenedFile) {
	fileInfo := openedFile.Info
	etag := fileInfo.ETag
	h.setConditionalHeaders(w, etag, fileInfo.ModTime, path)

	h.setMediaMetadataHeaders(w, path, fileInfo.ContentType, openedFile.Reader)
	h.setS3Headers(w, etag, fileInfo.ModTime, fileInfo.Size, path, fileInfo.ContentType)
	http.ServeContent(w, requestWithoutHandledConditionals(r), path, fileInfo.ModTime, openedFile.Reader)
}

func (h *FileHandler) sourceConditionalStatus(r *http.Request, path string, source *interfaces.FileInfo) (int, bool) {
	if !h.hasConditionalHeaders(r) {
		return 0, false
	}

	probe := &conditionalProbeResponseWriter{header: make(http.Header)}
	h.setConditionalHeaders(probe, source.ETag, source.ModTime, path)
	http.ServeContent(probe, requestForSourceConditionalProbe(r, source.ETag), path, source.ModTime, strings.NewReader(""))

	switch probe.statusCode {
	case http.StatusNotModified, http.StatusPreconditionFailed:
		return probe.statusCode, true
	default:
		return 0, false
	}
}

func requestWithoutHandledConditionals(r *http.Request) *http.Request {
	if !hasHandledConditionalHeaders(r) {
		return r
	}

	withoutConditionals := new(http.Request)
	*withoutConditionals = *r
	withoutConditionals.Header = r.Header.Clone()
	withoutConditionals.Header.Del("If-Match")
	withoutConditionals.Header.Del("If-None-Match")
	withoutConditionals.Header.Del("If-Unmodified-Since")
	withoutConditionals.Header.Del("If-Modified-Since")
	return withoutConditionals
}

func hasHandledConditionalHeaders(r *http.Request) bool {
	return r.Header.Get("If-Match") != "" ||
		r.Header.Get("If-None-Match") != "" ||
		r.Header.Get("If-Unmodified-Since") != "" ||
		r.Header.Get("If-Modified-Since") != ""
}

func requestForSourceConditionalProbe(r *http.Request, sourceETag string) *http.Request {
	bareIfNoneMatch := strings.TrimSpace(r.Header.Get("If-None-Match")) == sourceETag
	hasRangeHeaders := r.Header.Get("Range") != "" || r.Header.Get("If-Range") != ""
	if !bareIfNoneMatch && !hasRangeHeaders {
		return r
	}

	probeRequest := new(http.Request)
	*probeRequest = *r
	probeRequest.Header = r.Header.Clone()
	if bareIfNoneMatch {
		probeRequest.Header.Set("If-None-Match", `"`+sourceETag+`"`)
	}
	probeRequest.Header.Del("Range")
	probeRequest.Header.Del("If-Range")
	return probeRequest
}

type conditionalProbeResponseWriter struct {
	header     http.Header
	statusCode int
}

func (w *conditionalProbeResponseWriter) Header() http.Header {
	return w.header
}

func (w *conditionalProbeResponseWriter) WriteHeader(statusCode int) {
	if w.statusCode == 0 {
		w.statusCode = statusCode
	}
}

func (w *conditionalProbeResponseWriter) Write(p []byte) (int, error) {
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}
	return len(p), nil
}

func (h *FileHandler) setMediaMetadataHeaders(w http.ResponseWriter, path, contentType string, reader io.ReadSeeker) {
	if !shouldProbeMediaDimensions(path, contentType) {
		return
	}

	width, height, _, err := extractMediaMetadata(path, contentType, reader)
	if seekErr := seekReaderStart(reader); seekErr != nil {
		h.logger.Debug("Media metadata probe could not rewind reader",
			"path", path,
			"error", seekErr,
		)
	}
	if err != nil {
		h.logger.Debug("Media metadata header probe skipped",
			"path", path,
			"error", err,
		)
		return
	}

	w.Header().Set(mediaWidthHeader, strconv.Itoa(width))
	w.Header().Set(mediaHeightHeader, strconv.Itoa(height))
}

func seekReaderStart(reader io.Seeker) error {
	_, err := reader.Seek(0, io.SeekStart)
	return err
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
	w.Header().Set("Access-Control-Expose-Headers", "ETag, Last-Modified, Content-Length, "+mediaWidthHeader+", "+mediaHeightHeader)
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

func (h *FileHandler) handleMetadataStorageError(w http.ResponseWriter, err error, path string) {
	if storage.IsNotFound(err) {
		h.logger.Warn("Object not found", "path", path)
		writeJSONError(w, http.StatusNotFound, "The specified key does not exist.")
		return
	}

	h.logger.Error("Metadata storage error", "path", path, "error", err)
	writeJSONError(w, http.StatusInternalServerError, err.Error())
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

func shouldServeMetadata(r *http.Request) bool {
	return r.URL.Query().Has("meta")
}

func validateRequestPath(requestPath string) (string, bool) {
	path := strings.TrimPrefix(requestPath, "/")
	if path == "" {
		return "", false
	}
	if cleanedPath := strings.TrimPrefix(pathpkg.Clean("/"+path), "/"); cleanedPath != path {
		return path, false
	}
	return path, true
}

func writeJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
