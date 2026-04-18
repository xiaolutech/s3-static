package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestIntegration_FileServing(t *testing.T) {
	suite := SetupTestSuite(t)
	defer suite.Cleanup()

	// Upload test files
	testFiles := map[string]string{
		"test.txt":           "Hello, World!",
		"folder/nested.html": "<html><body>Nested file</body></html>",
		"image.png":          "fake-png-data",
		"style.css":          "body { color: red; }",
		"script.js":          "console.log('test');",
	}

	for path, content := range testFiles {
		err := suite.UploadTestFile(path, content)
		if err != nil {
			t.Fatalf("Failed to upload test file %s: %v", path, err)
		}
	}

	// Test file serving
	for path, expectedContent := range testFiles {
		t.Run("serve_"+path, func(t *testing.T) {
			req := suite.CreateTestRequest("GET", "/"+path, nil)
			w := suite.ExecuteRequest(req)

			suite.AssertStatusCode(t, w, http.StatusOK)
			suite.AssertBodyEquals(t, w, expectedContent)
			suite.AssertHeaderExists(t, w, "ETag")
			suite.AssertHeaderExists(t, w, "Last-Modified")
			suite.AssertHeaderExists(t, w, "Cache-Control")
			suite.AssertHeaderExists(t, w, "Content-Type")
		})
	}
}

func TestIntegration_ConditionalRequests(t *testing.T) {
	suite := SetupTestSuite(t)
	defer suite.Cleanup()

	// Upload test file
	testContent := "Test content for conditional requests"
	err := suite.UploadTestFile("conditional.txt", testContent)
	if err != nil {
		t.Fatalf("Failed to upload test file: %v", err)
	}

	// First request to get ETag and Last-Modified
	req := suite.CreateTestRequest("GET", "/conditional.txt", nil)
	w := suite.ExecuteRequest(req)
	suite.AssertStatusCode(t, w, http.StatusOK)

	etag := w.Header().Get("ETag")
	lastModified := w.Header().Get("Last-Modified")

	// Test If-None-Match with matching ETag
	t.Run("if_none_match_matching", func(t *testing.T) {
		req := suite.CreateTestRequest("GET", "/conditional.txt", nil)
		req.Header.Set("If-None-Match", etag)
		w := suite.ExecuteRequest(req)

		suite.AssertStatusCode(t, w, http.StatusNotModified)
		if w.Body.Len() != 0 {
			t.Error("Expected empty body for 304 response")
		}
	})

	// Test If-None-Match with non-matching ETag
	t.Run("if_none_match_non_matching", func(t *testing.T) {
		req := suite.CreateTestRequest("GET", "/conditional.txt", nil)
		req.Header.Set("If-None-Match", `"different-etag"`)
		w := suite.ExecuteRequest(req)

		suite.AssertStatusCode(t, w, http.StatusOK)
		suite.AssertBodyEquals(t, w, testContent)
	})

	// Test If-Modified-Since with future date
	t.Run("if_modified_since_future", func(t *testing.T) {
		futureTime := time.Now().Add(time.Hour).UTC().Format(http.TimeFormat)
		req := suite.CreateTestRequest("GET", "/conditional.txt", nil)
		req.Header.Set("If-Modified-Since", futureTime)
		w := suite.ExecuteRequest(req)

		suite.AssertStatusCode(t, w, http.StatusNotModified)
	})

	// Test If-Modified-Since with past date
	t.Run("if_modified_since_past", func(t *testing.T) {
		pastTime := time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat)
		req := suite.CreateTestRequest("GET", "/conditional.txt", nil)
		req.Header.Set("If-Modified-Since", pastTime)
		w := suite.ExecuteRequest(req)

		suite.AssertStatusCode(t, w, http.StatusOK)
		suite.AssertBodyEquals(t, w, testContent)
	})

	// Test If-Modified-Since with same time as Last-Modified
	t.Run("if_modified_since_same", func(t *testing.T) {
		req := suite.CreateTestRequest("GET", "/conditional.txt", nil)
		req.Header.Set("If-Modified-Since", lastModified)
		w := suite.ExecuteRequest(req)

		suite.AssertStatusCode(t, w, http.StatusNotModified)
	})
}

func TestIntegration_ContentTypes(t *testing.T) {
	suite := SetupTestSuite(t)
	defer suite.Cleanup()

	// Test files with different extensions
	testFiles := map[string]struct {
		content     string
		contentType string
	}{
		"test.html":    {"<html></html>", "text/html"},
		"test.css":     {"body{}", "text/css"},
		"test.js":      {"console.log()", "application/javascript"},
		"test.json":    {`{"key":"value"}`, "application/json"},
		"test.txt":     {"plain text", "text/plain"},
		"test.xml":     {"<xml></xml>", "application/xml"},
		"test.png":     {"fake-png", "image/png"},
		"test.jpg":     {"fake-jpg", "image/jpeg"},
		"test.gif":     {"fake-gif", "image/gif"},
		"test.svg":     {"<svg></svg>", "image/svg+xml"},
		"test.pdf":     {"fake-pdf", "application/pdf"},
		"test.zip":     {"fake-zip", "application/zip"},
		"test.unknown": {"unknown", "application/octet-stream"},
		"noextension":  {"no ext", "application/octet-stream"},
	}

	for path, fileData := range testFiles {
		err := suite.UploadTestFile(path, fileData.content)
		if err != nil {
			t.Fatalf("Failed to upload test file %s: %v", path, err)
		}
	}

	for path, fileData := range testFiles {
		t.Run("content_type_"+path, func(t *testing.T) {
			req := suite.CreateTestRequest("GET", "/"+path, nil)
			w := suite.ExecuteRequest(req)

			suite.AssertStatusCode(t, w, http.StatusOK)
			suite.AssertHeader(t, w, "Content-Type", fileData.contentType)
		})
	}
}

func TestIntegration_WebMContentType(t *testing.T) {
	suite := SetupTestSuite(t)
	defer suite.Cleanup()

	// Upload a .webm file with explicit Content-Type
	// .webm was previously not supported and would default to application/octet-stream
	path := "video.webm"
	contentType := "video/webm"
	content := "fake-webm-content"

	err := suite.UploadTestFileWithContentType(path, content, contentType)
	if err != nil {
		t.Fatalf("Failed to upload test file: %v", err)
	}

	req := suite.CreateTestRequest("GET", "/"+path, nil)
	w := suite.ExecuteRequest(req)

	suite.AssertStatusCode(t, w, http.StatusOK)
	suite.AssertHeader(t, w, "Content-Type", contentType)
}

func TestIntegration_HEADRequest(t *testing.T) {
	suite := SetupTestSuite(t)
	defer suite.Cleanup()

	err := suite.UploadTestFile("head.txt", "Hello, HEAD!")
	if err != nil {
		t.Fatalf("Failed to upload test file: %v", err)
	}

	req := suite.CreateTestRequest(http.MethodHead, "/head.txt", nil)
	w := suite.ExecuteRequest(req)

	suite.AssertStatusCode(t, w, http.StatusOK)
	if w.Body.Len() != 0 {
		t.Fatalf("Expected empty body for HEAD, got %q", w.Body.String())
	}
	suite.AssertHeaderExists(t, w, "ETag")
	suite.AssertHeaderExists(t, w, "Last-Modified")
	suite.AssertHeaderExists(t, w, "Content-Length")
}

func TestIntegration_RangeRequests(t *testing.T) {
	suite := SetupTestSuite(t)
	defer suite.Cleanup()

	content := "Hello, Range Requests!"
	err := suite.UploadTestFile("range.txt", content)
	if err != nil {
		t.Fatalf("Failed to upload test file: %v", err)
	}

	t.Run("partial content", func(t *testing.T) {
		req := suite.CreateTestRequest(http.MethodGet, "/range.txt", nil)
		req.Header.Set("Range", "bytes=0-4")
		w := suite.ExecuteRequest(req)

		suite.AssertStatusCode(t, w, http.StatusPartialContent)
		suite.AssertBodyEquals(t, w, "Hello")
		suite.AssertHeader(t, w, "Content-Range", "bytes 0-4/22")
	})

	t.Run("invalid range", func(t *testing.T) {
		req := suite.CreateTestRequest(http.MethodGet, "/range.txt", nil)
		req.Header.Set("Range", "bytes=100-200")
		w := suite.ExecuteRequest(req)

		suite.AssertStatusCode(t, w, http.StatusRequestedRangeNotSatisfiable)
	})
}

func TestIntegration_S3Headers(t *testing.T) {
	suite := SetupTestSuite(t)
	defer suite.Cleanup()

	// Upload test file
	err := suite.UploadTestFile("s3headers.txt", "Test S3 headers")
	if err != nil {
		t.Fatalf("Failed to upload test file: %v", err)
	}

	req := suite.CreateTestRequest("GET", "/s3headers.txt", nil)
	w := suite.ExecuteRequest(req)

	suite.AssertStatusCode(t, w, http.StatusOK)

	// Check S3-specific headers
	suite.AssertHeaderExists(t, w, "x-amz-request-id")
	suite.AssertHeaderExists(t, w, "x-amz-id-2")
	suite.AssertHeader(t, w, "Server", "S3-Static/1.0")
	suite.AssertHeader(t, w, "Accept-Ranges", "bytes")

	// Check CORS headers
	suite.AssertHeader(t, w, "Access-Control-Allow-Origin", "*")
	suite.AssertHeader(t, w, "Access-Control-Allow-Methods", "GET, HEAD")
	suite.AssertHeader(t, w, "Access-Control-Allow-Headers", "Range")
	suite.AssertHeaderExists(t, w, "Access-Control-Expose-Headers")
}

func TestIntegration_NotModifiedHeaders(t *testing.T) {
	suite := SetupTestSuite(t)
	defer suite.Cleanup()

	err := suite.UploadTestFile("validators.txt", "validator content")
	if err != nil {
		t.Fatalf("Failed to upload test file: %v", err)
	}

	req1 := suite.CreateTestRequest(http.MethodGet, "/validators.txt", nil)
	w1 := suite.ExecuteRequest(req1)
	suite.AssertStatusCode(t, w1, http.StatusOK)

	etag := w1.Header().Get("ETag")
	req2 := suite.CreateTestRequest(http.MethodGet, "/validators.txt", nil)
	req2.Header.Set("If-None-Match", etag)
	w2 := suite.ExecuteRequest(req2)

	suite.AssertStatusCode(t, w2, http.StatusNotModified)
	suite.AssertHeader(t, w2, "ETag", etag)
	suite.AssertHeaderExists(t, w2, "Last-Modified")
	suite.AssertHeaderExists(t, w2, "Cache-Control")
}

func TestIntegration_ErrorHandling(t *testing.T) {
	suite := SetupTestSuite(t)
	defer suite.Cleanup()

	// Test file not found
	t.Run("file_not_found", func(t *testing.T) {
		req := suite.CreateTestRequest("GET", "/nonexistent.txt", nil)
		w := suite.ExecuteRequest(req)

		suite.AssertStatusCode(t, w, http.StatusNotFound)
		suite.AssertHeader(t, w, "Content-Type", "application/xml")
		suite.AssertBodyContains(t, w, "<Error>")
		suite.AssertBodyContains(t, w, "<Code>")
		suite.AssertBodyContains(t, w, "<Message>")

		requestID := w.Header().Get("x-amz-request-id")
		if requestID == "" {
			t.Fatal("Expected x-amz-request-id header")
		}
		if !strings.Contains(w.Body.String(), "<RequestId>"+requestID+"</RequestId>") {
			t.Fatalf("Expected error XML to reuse request id %s, got %s", requestID, w.Body.String())
		}
	})

	// Test empty path
	t.Run("empty_path", func(t *testing.T) {
		req := suite.CreateTestRequest("GET", "/", nil)
		w := suite.ExecuteRequest(req)

		suite.AssertStatusCode(t, w, http.StatusBadRequest)
		suite.AssertBodyContains(t, w, "InvalidRequest")
	})

	// Test invalid normalized path
	t.Run("invalid_path", func(t *testing.T) {
		req := suite.CreateTestRequest("GET", "/folder/../secret.txt", nil)
		w := suite.ExecuteRequest(req)

		suite.AssertStatusCode(t, w, http.StatusBadRequest)
		suite.AssertBodyContains(t, w, "InvalidRequest")
	})

	// Test method not allowed
	t.Run("method_not_allowed", func(t *testing.T) {
		req := suite.CreateTestRequest("POST", "/test.txt", nil)
		w := suite.ExecuteRequest(req)

		suite.AssertStatusCode(t, w, http.StatusMethodNotAllowed)
		suite.AssertBodyContains(t, w, "MethodNotAllowed")
	})
}

func TestIntegration_CacheHeaders(t *testing.T) {
	suite := SetupTestSuite(t)
	defer suite.Cleanup()

	// Upload test file
	err := suite.UploadTestFile("cache.txt", "Cache test content")
	if err != nil {
		t.Fatalf("Failed to upload test file: %v", err)
	}

	req := suite.CreateTestRequest("GET", "/cache.txt", nil)
	w := suite.ExecuteRequest(req)

	suite.AssertStatusCode(t, w, http.StatusOK)

	// Check cache headers - default strategy is now no-cache
	cacheControl := w.Header().Get("Cache-Control")
	if cacheControl != "no-cache" {
		t.Errorf("Expected Cache-Control to be 'no-cache' (default strategy), got: %s", cacheControl)
	}

	etag := w.Header().Get("ETag")
	if etag == "" {
		t.Error("Expected ETag header to be set")
	}

	// ETag should be quoted
	if !strings.HasPrefix(etag, `"`) || !strings.HasSuffix(etag, `"`) {
		t.Errorf("Expected ETag to be quoted, got: %s", etag)
	}

	lastModified := w.Header().Get("Last-Modified")
	if lastModified == "" {
		t.Error("Expected Last-Modified header to be set")
	}

	// Parse Last-Modified to ensure it's valid
	_, err = http.ParseTime(lastModified)
	if err != nil {
		t.Errorf("Invalid Last-Modified format: %s", lastModified)
	}
}

func TestIntegration_CacheStrategies(t *testing.T) {
	testCases := []struct {
		name                string
		cacheStrategy       string
		cacheDuration       string
		expectedCacheHeader string
		description         string
	}{
		{
			name:                "no-cache strategy",
			cacheStrategy:       "no-cache",
			cacheDuration:       "1h",
			expectedCacheHeader: "no-cache",
			description:         "Should use no-cache for variable content",
		},
		{
			name:                "max-age strategy",
			cacheStrategy:       "max-age",
			cacheDuration:       "2h",
			expectedCacheHeader: "max-age=7200",
			description:         "Should use max-age with duration in seconds",
		},
		{
			name:                "immutable strategy",
			cacheStrategy:       "immutable",
			cacheDuration:       "24h",
			expectedCacheHeader: "max-age=86400, immutable",
			description:         "Should use max-age with immutable directive",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test suite with specific cache strategy
			suite := SetupTestSuiteWithEnv(t, map[string]string{
				"CACHE_STRATEGY": tc.cacheStrategy,
				"CACHE_DURATION": tc.cacheDuration,
			})
			defer suite.Cleanup()

			// Upload test file
			err := suite.UploadTestFile("strategy-test.txt", "Cache strategy test content")
			if err != nil {
				t.Fatalf("Failed to upload test file: %v", err)
			}

			req := suite.CreateTestRequest("GET", "/strategy-test.txt", nil)
			w := suite.ExecuteRequest(req)

			suite.AssertStatusCode(t, w, http.StatusOK)

			// Check cache strategy is applied correctly
			cacheControl := w.Header().Get("Cache-Control")
			if cacheControl != tc.expectedCacheHeader {
				t.Errorf("%s: Expected Cache-Control '%s', got '%s'",
					tc.description, tc.expectedCacheHeader, cacheControl)
			}

			// Verify other cache-related headers are still present
			if w.Header().Get("ETag") == "" {
				t.Error("ETag header should be present regardless of cache strategy")
			}

			if w.Header().Get("Last-Modified") == "" {
				t.Error("Last-Modified header should be present regardless of cache strategy")
			}

			// Test conditional requests work with all strategies
			etag := w.Header().Get("ETag")
			req2 := suite.CreateTestRequest("GET", "/strategy-test.txt", nil)
			req2.Header.Set("If-None-Match", etag)
			w2 := suite.ExecuteRequest(req2)

			// Should return 304 regardless of cache strategy
			if w2.Code != http.StatusNotModified {
				t.Errorf("Expected 304 Not Modified for strategy %s, got %d", tc.cacheStrategy, w2.Code)
			}
		})
	}
}

func TestIntegration_PathHandling(t *testing.T) {
	suite := SetupTestSuite(t)
	defer suite.Cleanup()

	// Upload files with different path patterns
	testPaths := []string{
		"simple.txt",
		"folder/nested.txt",
		"deep/folder/structure/file.txt",
		"with-dashes.txt",
		"with_underscores.txt",
		"with.dots.txt",
	}

	for _, path := range testPaths {
		content := "Content for " + path
		err := suite.UploadTestFile(path, content)
		if err != nil {
			t.Fatalf("Failed to upload test file %s: %v", path, err)
		}
	}

	// Test accessing files with different path formats
	for _, path := range testPaths {
		t.Run("path_"+strings.ReplaceAll(path, "/", "_"), func(t *testing.T) {
			// Test with leading slash
			req := suite.CreateTestRequest("GET", "/"+path, nil)
			w := suite.ExecuteRequest(req)
			suite.AssertStatusCode(t, w, http.StatusOK)
			suite.AssertBodyEquals(t, w, "Content for "+path)
		})
	}
}

func TestIntegration_ConcurrentRequests(t *testing.T) {
	suite := SetupTestSuite(t)
	defer suite.Cleanup()

	// Upload test file
	testContent := "Concurrent access test"
	err := suite.UploadTestFile("concurrent.txt", testContent)
	if err != nil {
		t.Fatalf("Failed to upload test file: %v", err)
	}

	// Make multiple concurrent requests
	const numRequests = 10
	results := make(chan *httptest.ResponseRecorder, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			req := suite.CreateTestRequest("GET", "/concurrent.txt", nil)
			w := suite.ExecuteRequest(req)
			results <- w
		}()
	}

	// Collect and verify results
	for i := 0; i < numRequests; i++ {
		w := <-results
		suite.AssertStatusCode(t, w, http.StatusOK)
		suite.AssertBodyEquals(t, w, testContent)
		suite.AssertHeaderExists(t, w, "ETag")
	}
}

func TestIntegration_LargeFile(t *testing.T) {
	suite := SetupTestSuite(t)
	defer suite.Cleanup()

	// Create a larger test file (1MB)
	largeContent := strings.Repeat("This is a large file content. ", 32768) // ~1MB
	err := suite.UploadTestFile("large.txt", largeContent)
	if err != nil {
		t.Fatalf("Failed to upload large test file: %v", err)
	}

	req := suite.CreateTestRequest("GET", "/large.txt", nil)
	w := suite.ExecuteRequest(req)

	suite.AssertStatusCode(t, w, http.StatusOK)
	suite.AssertBodyEquals(t, w, largeContent)
	suite.AssertHeaderExists(t, w, "Content-Length")

	// Verify Content-Length header
	expectedLength := len(largeContent)
	contentLength := w.Header().Get("Content-Length")
	if contentLength != string(rune(expectedLength)) {
		// Convert to string properly
		expectedLengthStr := ""
		for expectedLength > 0 {
			expectedLengthStr = string(rune(expectedLength%10+'0')) + expectedLengthStr
			expectedLength /= 10
		}
		if contentLength != expectedLengthStr {
			t.Errorf("Expected Content-Length to match file size")
		}
	}
}
