package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"s3-static/internal/config"
	"s3-static/internal/testutils"
)

func TestFileHandler_ComprehensiveScenarios(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := config.NewLogger("info")

	t.Run("Multiple Files Different Types", func(t *testing.T) {
		storage := testutils.NewStorageBuilder().
			WithFile("index.html", "<html><body>Home</body></html>").
			WithFile("style.css", "body { margin: 0; }").
			WithFile("script.js", "console.log('loaded');").
			WithFile("data.json", `{"status": "ok"}`).
			WithFile("readme.txt", "This is a readme file").
			Build()

		handler := NewFileHandler(storage, cfg, logger)

		testCases := []struct {
			path        string
			contentType string
			content     string
		}{
			{"index.html", "text/html", "<html><body>Home</body></html>"},
			{"style.css", "text/css", "body { margin: 0; }"},
			{"script.js", "application/javascript", "console.log('loaded');"},
			{"data.json", "application/json", `{"status": "ok"}`},
			{"readme.txt", "text/plain", "This is a readme file"},
		}

		for _, tc := range testCases {
			t.Run(tc.path, func(t *testing.T) {
				req := httptest.NewRequest("GET", "/"+tc.path, nil)
				w := httptest.NewRecorder()

				handler.ServeHTTP(w, req)

				if w.Code != http.StatusOK {
					t.Errorf("Expected status 200, got %d", w.Code)
				}

				if w.Header().Get("Content-Type") != tc.contentType {
					t.Errorf("Expected Content-Type %s, got %s", tc.contentType, w.Header().Get("Content-Type"))
				}

				if w.Body.String() != tc.content {
					t.Errorf("Expected body %s, got %s", tc.content, w.Body.String())
				}
			})
		}
	})

	t.Run("Conditional Requests Edge Cases", func(t *testing.T) {
		modTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
		storage := testutils.NewStorageBuilder().
			WithFileAndTime("test.txt", "test content", modTime).
			Build()

		handler := NewFileHandler(storage, cfg, logger)

		testCases := []struct {
			name           string
			headers        map[string]string
			expectedStatus int
		}{
			{
				name:           "If-None-Match with wildcard",
				headers:        map[string]string{"If-None-Match": "*"},
				expectedStatus: http.StatusNotModified,
			},
			{
				name:           "If-None-Match with matching ETag",
				headers:        map[string]string{"If-None-Match": `mock-etag-test.txt`},
				expectedStatus: http.StatusNotModified,
			},
			{
				name:           "If-Modified-Since with invalid date",
				headers:        map[string]string{"If-Modified-Since": "invalid-date"},
				expectedStatus: http.StatusOK,
			},
			{
				name: "Both If-None-Match and If-Modified-Since",
				headers: map[string]string{
					"If-None-Match":     "mock-etag-test.txt",
					"If-Modified-Since": modTime.Add(time.Hour).Format(http.TimeFormat),
				},
				expectedStatus: http.StatusNotModified,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				req := httptest.NewRequest("GET", "/test.txt", nil)
				for key, value := range tc.headers {
					req.Header.Set(key, value)
				}
				w := httptest.NewRecorder()

				handler.ServeHTTP(w, req)

				if w.Code != tc.expectedStatus {
					t.Errorf("Expected status %d, got %d", tc.expectedStatus, w.Code)
				}
			})
		}
	})

	t.Run("Error Handling Scenarios", func(t *testing.T) {
		storage := testutils.NewStorageBuilder().
			WithError("forbidden.txt", testutils.ErrMockForbidden).
			WithError("timeout.txt", testutils.ErrMockTimeout).
			WithError("network.txt", testutils.ErrMockNetworkError).
			Build()

		handler := NewFileHandler(storage, cfg, logger)

		testCases := []struct {
			path           string
			expectedStatus int
		}{
			{"nonexistent.txt", http.StatusInternalServerError},
			{"forbidden.txt", http.StatusInternalServerError},
			{"timeout.txt", http.StatusInternalServerError},
			{"network.txt", http.StatusInternalServerError},
		}

		for _, tc := range testCases {
			t.Run(tc.path, func(t *testing.T) {
				req := httptest.NewRequest("GET", "/"+tc.path, nil)
				w := httptest.NewRecorder()

				handler.ServeHTTP(w, req)

				if w.Code != tc.expectedStatus {
					t.Errorf("Expected status %d, got %d", tc.expectedStatus, w.Code)
				}

				// Check error response format
				if w.Header().Get("Content-Type") != "application/xml" {
					t.Error("Expected XML content type for error response")
				}

				body := w.Body.String()
				if !strings.Contains(body, "<Error>") {
					t.Error("Expected XML error response")
				}
			})
		}
	})

	t.Run("Path Normalization", func(t *testing.T) {
		storage := testutils.NewStorageBuilder().
			WithFile("normal.txt", "normal file").
			WithFile("folder/nested.txt", "nested file").
			Build()

		handler := NewFileHandler(storage, cfg, logger)

		testCases := []struct {
			requestPath string
			shouldWork  bool
		}{
			{"/normal.txt", true},
			{"//normal.txt", false}, // Double slash should be handled by URL parsing
			{"/folder/nested.txt", true},
			{"/folder/../normal.txt", false}, // Path traversal attempt
			{"/empty", false},                // Empty path (use /empty instead of empty string)
			{"/", false},                     // Root path
		}

		for _, tc := range testCases {
			t.Run("path_"+tc.requestPath, func(t *testing.T) {
				req := httptest.NewRequest("GET", tc.requestPath, nil)
				w := httptest.NewRecorder()

				handler.ServeHTTP(w, req)

				if tc.shouldWork {
					if w.Code != http.StatusOK {
						t.Errorf("Expected successful response for path %s, got %d", tc.requestPath, w.Code)
					}
				} else {
					if w.Code == http.StatusOK {
						t.Errorf("Expected error response for path %s, got %d", tc.requestPath, w.Code)
					}
				}
			})
		}
	})

	t.Run("Header Validation", func(t *testing.T) {
		storage := testutils.NewStorageBuilder().
			WithFileAndETag("test.txt", "test content", "custom-etag-123").
			Build()

		handler := NewFileHandler(storage, cfg, logger)

		req := httptest.NewRequest("GET", "/test.txt", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		// Validate all required headers are present
		requiredHeaders := []string{
			"ETag",
			"Last-Modified",
			"Cache-Control",
			"Content-Type",
			"Content-Length",
			"x-amz-request-id",
			"x-amz-id-2",
			"Server",
			"Accept-Ranges",
		}

		for _, header := range requiredHeaders {
			if w.Header().Get(header) == "" {
				t.Errorf("Required header %s is missing", header)
			}
		}

		// Validate specific header values
		if w.Header().Get("ETag") != `"custom-etag-123"` {
			t.Errorf("Expected ETag to be quoted custom ETag, got %s", w.Header().Get("ETag"))
		}

		if w.Header().Get("Server") != "S3-Static/1.0" {
			t.Errorf("Expected Server header to be 'S3-Static/1.0', got %s", w.Header().Get("Server"))
		}

		if w.Header().Get("Accept-Ranges") != "bytes" {
			t.Errorf("Expected Accept-Ranges to be 'bytes', got %s", w.Header().Get("Accept-Ranges"))
		}
	})

	t.Run("Storage Method Call Tracking", func(t *testing.T) {
		storage := testutils.NewStorageBuilder().
			WithFile("tracked.txt", "tracked content").
			Build()

		handler := NewFileHandler(storage, cfg, logger)

		// Reset call counts
		storage.ResetCallCounts()

		// Make request
		req := httptest.NewRequest("GET", "/tracked.txt", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		// Verify method calls
		if storage.GetCallCount("GetFileInfo") != 1 {
			t.Errorf("Expected GetFileInfo to be called once, got %d", storage.GetCallCount("GetFileInfo"))
		}

		if storage.GetCallCount("GetFileReader") != 1 {
			t.Errorf("Expected GetFileReader to be called once, got %d", storage.GetCallCount("GetFileReader"))
		}

		// FileExists should not be called in normal flow
		if storage.GetCallCount("FileExists") != 0 {
			t.Errorf("Expected FileExists not to be called, got %d", storage.GetCallCount("FileExists"))
		}
	})

	t.Run("Conditional Request Optimization", func(t *testing.T) {
		storage := testutils.NewStorageBuilder().
			WithFile("conditional.txt", "conditional content").
			Build()

		handler := NewFileHandler(storage, cfg, logger)

		// First request to get ETag
		req1 := httptest.NewRequest("GET", "/conditional.txt", nil)
		w1 := httptest.NewRecorder()
		handler.ServeHTTP(w1, req1)
		etag := w1.Header().Get("ETag")

		// Reset call counts
		storage.ResetCallCounts()

		// Conditional request with matching ETag
		req2 := httptest.NewRequest("GET", "/conditional.txt", nil)
		req2.Header.Set("If-None-Match", etag)
		w2 := httptest.NewRecorder()
		handler.ServeHTTP(w2, req2)

		// Should still call GetFileInfo to get metadata
		if storage.GetCallCount("GetFileInfo") != 1 {
			t.Errorf("Expected GetFileInfo to be called once for conditional request, got %d", storage.GetCallCount("GetFileInfo"))
		}

		// Should NOT call GetFileReader for 304 response
		if storage.GetCallCount("GetFileReader") != 0 {
			t.Errorf("Expected GetFileReader not to be called for 304 response, got %d", storage.GetCallCount("GetFileReader"))
		}

		if w2.Code != http.StatusNotModified {
			t.Errorf("Expected 304 status, got %d", w2.Code)
		}
	})
}

func TestHealthHandler_Comprehensive(t *testing.T) {
	logger := config.NewLogger("info")

	t.Run("Health Check Success", func(t *testing.T) {
		storage := testutils.NewStorageBuilder().Build()
		handler := NewHealthHandler(storage, logger)

		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		if w.Header().Get("Content-Type") != "application/json" {
			t.Error("Expected JSON content type")
		}

		body := w.Body.String()
		if !strings.Contains(body, `"status":"healthy"`) {
			t.Error("Expected healthy status in response")
		}

		if !strings.Contains(body, `"timestamp"`) {
			t.Error("Expected timestamp in response")
		}
	})

	t.Run("Health Check Method Validation", func(t *testing.T) {
		storage := testutils.NewStorageBuilder().Build()
		handler := NewHealthHandler(storage, logger)

		methods := []string{"POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}

		for _, method := range methods {
			t.Run("method_"+method, func(t *testing.T) {
				req := httptest.NewRequest(method, "/health", nil)
				w := httptest.NewRecorder()

				handler.ServeHTTP(w, req)

				if w.Code != http.StatusMethodNotAllowed {
					t.Errorf("Expected status 405 for method %s, got %d", method, w.Code)
				}
			})
		}
	})

	t.Run("Health Check Response Format", func(t *testing.T) {
		storage := testutils.NewStorageBuilder().Build()
		handler := NewHealthHandler(storage, logger)

		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		body := w.Body.String()

		// Validate JSON structure
		if !strings.HasPrefix(body, "{") || !strings.HasSuffix(body, "}") {
			t.Error("Response should be valid JSON object")
		}

		// Check required fields
		requiredFields := []string{`"status"`, `"timestamp"`}
		for _, field := range requiredFields {
			if !strings.Contains(body, field) {
				t.Errorf("Response should contain field %s", field)
			}
		}
	})
}

func TestFileHandler_EdgeCases(t *testing.T) {
	cfg := config.DefaultConfig()
	logger := config.NewLogger("info")

	t.Run("Empty File", func(t *testing.T) {
		storage := testutils.NewStorageBuilder().
			WithFile("empty.txt", "").
			Build()

		handler := NewFileHandler(storage, cfg, logger)

		req := httptest.NewRequest("GET", "/empty.txt", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200 for empty file, got %d", w.Code)
		}

		if w.Body.String() != "" {
			t.Error("Expected empty body for empty file")
		}

		if w.Header().Get("Content-Length") != "0" {
			t.Errorf("Expected Content-Length 0, got %s", w.Header().Get("Content-Length"))
		}
	})

	t.Run("Very Long Filename", func(t *testing.T) {
		longFilename := strings.Repeat("a", 255) + ".txt"
		storage := testutils.NewStorageBuilder().
			WithFile(longFilename, "long filename content").
			Build()

		handler := NewFileHandler(storage, cfg, logger)

		req := httptest.NewRequest("GET", "/"+longFilename, nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200 for long filename, got %d", w.Code)
		}
	})

	t.Run("Special Characters in Filename", func(t *testing.T) {
		specialFiles := []string{
			"file-with-dashes.txt",
			"file_with_underscores.txt",
			"file.with.dots.txt",
		}

		storage := testutils.NewStorageBuilder()
		for _, filename := range specialFiles {
			storage.WithFile(filename, "content for "+filename)
		}
		storageInstance := storage.Build()

		handler := NewFileHandler(storageInstance, cfg, logger)

		for _, filename := range specialFiles {
			t.Run("special_"+filename, func(t *testing.T) {
				req := httptest.NewRequest("GET", "/"+filename, nil)
				w := httptest.NewRecorder()

				handler.ServeHTTP(w, req)

				if w.Code != http.StatusOK {
					t.Errorf("Expected status 200 for filename %s, got %d", filename, w.Code)
				}

				expectedContent := "content for " + filename
				if w.Body.String() != expectedContent {
					t.Errorf("Expected content %s, got %s", expectedContent, w.Body.String())
				}
			})
		}
	})

	t.Run("Binary File Content", func(t *testing.T) {
		// Create binary content (simulated)
		binaryContent := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A} // PNG header
		storage := testutils.NewMockStorage()
		storage.AddFile("binary.png", binaryContent, time.Now(), "binary-etag")

		handler := NewFileHandler(storage, cfg, logger)

		req := httptest.NewRequest("GET", "/binary.png", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200 for binary file, got %d", w.Code)
		}

		if w.Header().Get("Content-Type") != "image/png" {
			t.Errorf("Expected Content-Type image/png, got %s", w.Header().Get("Content-Type"))
		}

		responseBody := w.Body.Bytes()
		if len(responseBody) != len(binaryContent) {
			t.Errorf("Expected body length %d, got %d", len(binaryContent), len(responseBody))
		}

		for i, b := range binaryContent {
			if responseBody[i] != b {
				t.Errorf("Binary content mismatch at byte %d: expected %x, got %x", i, b, responseBody[i])
			}
		}
	})
}
