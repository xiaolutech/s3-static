package handler

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"s3-static/internal/config"
	"s3-static/internal/testutils"
)

// TestFileHandler_CacheStrategies tests all three caching strategies
func TestFileHandler_CacheStrategies(t *testing.T) {
	logger := config.NewLogger("info")
	storage := testutils.NewStorageBuilder().
		WithFile("test.txt", "test content").
		Build()

	testCases := []struct {
		name                string
		cacheStrategy       string
		cacheDuration       time.Duration
		expectedCacheHeader string
		description         string
	}{
		{
			name:                "no-cache strategy",
			cacheStrategy:       "no-cache",
			cacheDuration:       time.Hour,
			expectedCacheHeader: "no-cache",
			description:         "Should set Cache-Control to no-cache for variable content",
		},
		{
			name:                "max-age strategy",
			cacheStrategy:       "max-age",
			cacheDuration:       time.Hour,
			expectedCacheHeader: "max-age=3600",
			description:         "Should set Cache-Control to max-age with duration in seconds",
		},
		{
			name:                "immutable strategy",
			cacheStrategy:       "immutable",
			cacheDuration:       24 * time.Hour,
			expectedCacheHeader: "max-age=86400, immutable",
			description:         "Should set Cache-Control to max-age with immutable directive",
		},
		{
			name:                "invalid strategy defaults to no-cache",
			cacheStrategy:       "invalid-strategy",
			cacheDuration:       time.Hour,
			expectedCacheHeader: "no-cache",
			description:         "Should default to no-cache for invalid strategies",
		},
		{
			name:                "empty strategy defaults to no-cache",
			cacheStrategy:       "",
			cacheDuration:       time.Hour,
			expectedCacheHeader: "no-cache",
			description:         "Should default to no-cache for empty strategy",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create config with specific cache strategy
			cfg := config.DefaultConfig()
			cfg.CacheStrategy = tc.cacheStrategy
			cfg.DefaultCacheDuration = tc.cacheDuration

			handler := NewFileHandler(storage, cfg, logger)

			req := httptest.NewRequest("GET", "/test.txt", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}

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
		})
	}
}

// TestFileHandler_CacheStrategyWithConditionalRequests tests that conditional requests work with all cache strategies
func TestFileHandler_CacheStrategyWithConditionalRequests(t *testing.T) {
	logger := config.NewLogger("info")
	modTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	storage := testutils.NewStorageBuilder().
		WithFileAndTime("conditional.txt", "conditional content", modTime).
		Build()

	strategies := []string{"no-cache", "max-age", "immutable"}

	for _, strategy := range strategies {
		t.Run("strategy_"+strategy, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.CacheStrategy = strategy
			cfg.DefaultCacheDuration = time.Hour

			handler := NewFileHandler(storage, cfg, logger)

			// First request to get ETag
			req1 := httptest.NewRequest("GET", "/conditional.txt", nil)
			w1 := httptest.NewRecorder()
			handler.ServeHTTP(w1, req1)

			if w1.Code != http.StatusOK {
				t.Fatalf("First request failed with status %d", w1.Code)
			}

			etag := w1.Header().Get("ETag")
			if etag == "" {
				t.Fatal("ETag should be present in first response")
			}

			// Second request with If-None-Match
			req2 := httptest.NewRequest("GET", "/conditional.txt", nil)
			req2.Header.Set("If-None-Match", etag)
			w2 := httptest.NewRecorder()
			handler.ServeHTTP(w2, req2)

			// Should return 304 regardless of cache strategy
			if w2.Code != http.StatusNotModified {
				t.Errorf("Expected 304 Not Modified for strategy %s, got %d", strategy, w2.Code)
			}

			// Body should be empty for 304 response
			if w2.Body.Len() != 0 {
				t.Errorf("Expected empty body for 304 response with strategy %s", strategy)
			}
		})
	}
}

// TestFileHandler_CacheDurationVariations tests different cache durations
func TestFileHandler_CacheDurationVariations(t *testing.T) {
	logger := config.NewLogger("info")
	storage := testutils.NewStorageBuilder().
		WithFile("duration-test.txt", "duration test content").
		Build()

	testCases := []struct {
		name          string
		duration      time.Duration
		expectedMaxAge string
	}{
		{
			name:          "1 hour",
			duration:      time.Hour,
			expectedMaxAge: "3600",
		},
		{
			name:          "30 minutes",
			duration:      30 * time.Minute,
			expectedMaxAge: "1800",
		},
		{
			name:          "1 day",
			duration:      24 * time.Hour,
			expectedMaxAge: "86400",
		},
		{
			name:          "1 week",
			duration:      7 * 24 * time.Hour,
			expectedMaxAge: "604800",
		},
		{
			name:          "1 year",
			duration:      365 * 24 * time.Hour,
			expectedMaxAge: "31536000",
		},
	}

	for _, tc := range testCases {
		t.Run("max-age_"+tc.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.CacheStrategy = "max-age"
			cfg.DefaultCacheDuration = tc.duration

			handler := NewFileHandler(storage, cfg, logger)

			req := httptest.NewRequest("GET", "/duration-test.txt", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			expectedCacheControl := "max-age=" + tc.expectedMaxAge
			cacheControl := w.Header().Get("Cache-Control")
			if cacheControl != expectedCacheControl {
				t.Errorf("Expected Cache-Control '%s', got '%s'", expectedCacheControl, cacheControl)
			}
		})

		t.Run("immutable_"+tc.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.CacheStrategy = "immutable"
			cfg.DefaultCacheDuration = tc.duration

			handler := NewFileHandler(storage, cfg, logger)

			req := httptest.NewRequest("GET", "/duration-test.txt", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			expectedCacheControl := "max-age=" + tc.expectedMaxAge + ", immutable"
			cacheControl := w.Header().Get("Cache-Control")
			if cacheControl != expectedCacheControl {
				t.Errorf("Expected Cache-Control '%s', got '%s'", expectedCacheControl, cacheControl)
			}
		})
	}
}

// TestFileHandler_CacheStrategyBehaviorDocumentation tests and documents expected behavior
func TestFileHandler_CacheStrategyBehaviorDocumentation(t *testing.T) {
	logger := config.NewLogger("info")
	storage := testutils.NewStorageBuilder().
		WithFile("behavior-test.txt", "behavior test content").
		Build()

	t.Run("no-cache behavior", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.CacheStrategy = "no-cache"
		cfg.DefaultCacheDuration = time.Hour

		handler := NewFileHandler(storage, cfg, logger)

		req := httptest.NewRequest("GET", "/behavior-test.txt", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		// Verify no-cache behavior
		cacheControl := w.Header().Get("Cache-Control")
		if cacheControl != "no-cache" {
			t.Errorf("no-cache strategy should set Cache-Control to 'no-cache', got '%s'", cacheControl)
		}

		// ETag and Last-Modified should still be present for conditional requests
		if w.Header().Get("ETag") == "" {
			t.Error("ETag should be present with no-cache strategy for conditional requests")
		}

		if w.Header().Get("Last-Modified") == "" {
			t.Error("Last-Modified should be present with no-cache strategy for conditional requests")
		}

		t.Log("no-cache strategy: Forces browser to validate cache on every request using ETag/Last-Modified")
	})

	t.Run("max-age behavior", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.CacheStrategy = "max-age"
		cfg.DefaultCacheDuration = 2 * time.Hour

		handler := NewFileHandler(storage, cfg, logger)

		req := httptest.NewRequest("GET", "/behavior-test.txt", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		// Verify max-age behavior
		cacheControl := w.Header().Get("Cache-Control")
		expectedCacheControl := "max-age=" + strconv.Itoa(int((2 * time.Hour).Seconds()))
		if cacheControl != expectedCacheControl {
			t.Errorf("max-age strategy should set Cache-Control to '%s', got '%s'", expectedCacheControl, cacheControl)
		}

		// ETag and Last-Modified should still be present
		if w.Header().Get("ETag") == "" {
			t.Error("ETag should be present with max-age strategy")
		}

		if w.Header().Get("Last-Modified") == "" {
			t.Error("Last-Modified should be present with max-age strategy")
		}

		t.Log("max-age strategy: Browser caches for specified duration, then validates with conditional requests")
	})

	t.Run("immutable behavior", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.CacheStrategy = "immutable"
		cfg.DefaultCacheDuration = 24 * time.Hour

		handler := NewFileHandler(storage, cfg, logger)

		req := httptest.NewRequest("GET", "/behavior-test.txt", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		// Verify immutable behavior
		cacheControl := w.Header().Get("Cache-Control")
		expectedCacheControl := "max-age=" + strconv.Itoa(int((24 * time.Hour).Seconds())) + ", immutable"
		if cacheControl != expectedCacheControl {
			t.Errorf("immutable strategy should set Cache-Control to '%s', got '%s'", expectedCacheControl, cacheControl)
		}

		// ETag and Last-Modified should still be present (though less relevant for immutable content)
		if w.Header().Get("ETag") == "" {
			t.Error("ETag should be present with immutable strategy")
		}

		if w.Header().Get("Last-Modified") == "" {
			t.Error("Last-Modified should be present with immutable strategy")
		}

		t.Log("immutable strategy: Browser caches for max-age duration without any validation requests")
	})
}

// TestFileHandler_CacheStrategyEdgeCases tests edge cases for cache strategies
func TestFileHandler_CacheStrategyEdgeCases(t *testing.T) {
	logger := config.NewLogger("info")
	storage := testutils.NewStorageBuilder().
		WithFile("edge-case.txt", "edge case content").
		Build()

	t.Run("zero cache duration", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.CacheStrategy = "max-age"
		cfg.DefaultCacheDuration = 0

		handler := NewFileHandler(storage, cfg, logger)

		req := httptest.NewRequest("GET", "/edge-case.txt", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		cacheControl := w.Header().Get("Cache-Control")
		if cacheControl != "max-age=0" {
			t.Errorf("Expected Cache-Control 'max-age=0' for zero duration, got '%s'", cacheControl)
		}
	})

	t.Run("negative cache duration", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.CacheStrategy = "max-age"
		cfg.DefaultCacheDuration = -time.Hour

		handler := NewFileHandler(storage, cfg, logger)

		req := httptest.NewRequest("GET", "/edge-case.txt", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		cacheControl := w.Header().Get("Cache-Control")
		// Negative duration should result in negative max-age (which browsers treat as expired)
		if !strings.Contains(cacheControl, "max-age=-3600") {
			t.Errorf("Expected Cache-Control to contain 'max-age=-3600' for negative duration, got '%s'", cacheControl)
		}
	})

	t.Run("very large cache duration", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.CacheStrategy = "immutable"
		cfg.DefaultCacheDuration = 10 * 365 * 24 * time.Hour // 10 years

		handler := NewFileHandler(storage, cfg, logger)

		req := httptest.NewRequest("GET", "/edge-case.txt", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		cacheControl := w.Header().Get("Cache-Control")
		expectedMaxAge := strconv.Itoa(int((10 * 365 * 24 * time.Hour).Seconds()))
		expectedCacheControl := "max-age=" + expectedMaxAge + ", immutable"
		if cacheControl != expectedCacheControl {
			t.Errorf("Expected Cache-Control '%s' for large duration, got '%s'", expectedCacheControl, cacheControl)
		}
	})
}

// TestFileHandler_CacheStrategyWithDifferentFileTypes tests cache strategies with various file types
func TestFileHandler_CacheStrategyWithDifferentFileTypes(t *testing.T) {
	logger := config.NewLogger("info")
	storage := testutils.NewStorageBuilder().
		WithFile("index.html", "<html><body>Home</body></html>").
		WithFile("style.css", "body { margin: 0; }").
		WithFile("script.js", "console.log('loaded');").
		WithFile("image.png", "fake-png-content").
		WithFile("data.json", `{"status": "ok"}`).
		Build()

	fileTypes := []struct {
		path        string
		contentType string
	}{
		{"index.html", "text/html"},
		{"style.css", "text/css"},
		{"script.js", "application/javascript"},
		{"image.png", "image/png"},
		{"data.json", "application/json"},
	}

	strategies := []string{"no-cache", "max-age", "immutable"}

	for _, strategy := range strategies {
		for _, fileType := range fileTypes {
			t.Run(strategy+"_"+fileType.path, func(t *testing.T) {
				cfg := config.DefaultConfig()
				cfg.CacheStrategy = strategy
				cfg.DefaultCacheDuration = time.Hour

				handler := NewFileHandler(storage, cfg, logger)

				req := httptest.NewRequest("GET", "/"+fileType.path, nil)
				w := httptest.NewRecorder()

				handler.ServeHTTP(w, req)

				if w.Code != http.StatusOK {
					t.Errorf("Expected status 200, got %d", w.Code)
				}

				// Verify content type is correct
				if w.Header().Get("Content-Type") != fileType.contentType {
					t.Errorf("Expected Content-Type %s, got %s", fileType.contentType, w.Header().Get("Content-Type"))
				}

				// Verify cache strategy is applied regardless of file type
				cacheControl := w.Header().Get("Cache-Control")
				switch strategy {
				case "no-cache":
					if cacheControl != "no-cache" {
						t.Errorf("Expected Cache-Control 'no-cache', got '%s'", cacheControl)
					}
				case "max-age":
					if cacheControl != "max-age=3600" {
						t.Errorf("Expected Cache-Control 'max-age=3600', got '%s'", cacheControl)
					}
				case "immutable":
					if cacheControl != "max-age=3600, immutable" {
						t.Errorf("Expected Cache-Control 'max-age=3600, immutable', got '%s'", cacheControl)
					}
				}
			})
		}
	}
}

// TestFileHandler_CacheStrategyPerformanceImplications tests performance aspects of different strategies
func TestFileHandler_CacheStrategyPerformanceImplications(t *testing.T) {
	logger := config.NewLogger("info")
	storage := testutils.NewStorageBuilder().
		WithFile("perf-test.txt", "performance test content").
		Build()

	t.Run("no-cache allows 304 optimization", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.CacheStrategy = "no-cache"

		handler := NewFileHandler(storage, cfg, logger)

		// First request
		req1 := httptest.NewRequest("GET", "/perf-test.txt", nil)
		w1 := httptest.NewRecorder()
		handler.ServeHTTP(w1, req1)

		etag := w1.Header().Get("ETag")
		
		// Reset call counts to measure second request
		storage.ResetCallCounts()

		// Second request with If-None-Match
		req2 := httptest.NewRequest("GET", "/perf-test.txt", nil)
		req2.Header.Set("If-None-Match", etag)
		w2 := httptest.NewRecorder()
		handler.ServeHTTP(w2, req2)

		// Should return 304 and avoid reading file content
		if w2.Code != http.StatusNotModified {
			t.Errorf("Expected 304 Not Modified, got %d", w2.Code)
		}

		// Should still call GetFileInfo but not ReadFile
		if storage.GetCallCount("GetFileInfo") != 1 {
			t.Errorf("Expected GetFileInfo to be called once, got %d", storage.GetCallCount("GetFileInfo"))
		}

		if storage.GetCallCount("ReadFile") != 0 {
			t.Errorf("Expected ReadFile not to be called for 304 response, got %d", storage.GetCallCount("ReadFile"))
		}

		t.Log("no-cache strategy allows 304 Not Modified optimization, saving bandwidth")
	})

	t.Run("immutable strategy implications", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.CacheStrategy = "immutable"
		cfg.DefaultCacheDuration = 24 * time.Hour

		handler := NewFileHandler(storage, cfg, logger)

		req := httptest.NewRequest("GET", "/perf-test.txt", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		cacheControl := w.Header().Get("Cache-Control")
		if !strings.Contains(cacheControl, "immutable") {
			t.Errorf("Expected Cache-Control to contain 'immutable', got '%s'", cacheControl)
		}

		t.Log("immutable strategy: Browser won't send ANY requests during max-age period, best performance but requires URL versioning")
	})
}