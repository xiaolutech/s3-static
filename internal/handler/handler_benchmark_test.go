package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"s3-static/internal/config"
	"s3-static/internal/testutils"
)

func BenchmarkFileHandler_ServeFile(b *testing.B) {
	cfg := config.DefaultConfig()
	logger := config.NewLogger("error") // Use error level to reduce logging overhead

	storage := testutils.NewStorageBuilder().
		WithFile("benchmark.txt", "This is benchmark content for testing performance").
		Build()

	handler := NewFileHandler(storage, cfg, logger)

	req := httptest.NewRequest("GET", "/benchmark.txt", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", w.Code)
		}
	}
}

func BenchmarkFileHandler_ConditionalRequest(b *testing.B) {
	cfg := config.DefaultConfig()
	logger := config.NewLogger("error")

	storage := testutils.NewStorageBuilder().
		WithFile("conditional.txt", "Conditional request benchmark content").
		Build()

	handler := NewFileHandler(storage, cfg, logger)

	// Get ETag first
	req := httptest.NewRequest("GET", "/conditional.txt", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	etag := w.Header().Get("ETag")

	// Benchmark conditional requests
	req = httptest.NewRequest("GET", "/conditional.txt", nil)
	req.Header.Set("If-None-Match", etag)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotModified {
			b.Fatalf("Expected status 304, got %d", w.Code)
		}
	}
}

func BenchmarkFileHandler_SmallFile(b *testing.B) {
	cfg := config.DefaultConfig()
	logger := config.NewLogger("error")

	// 1KB file
	content := strings.Repeat("x", 1024)
	storage := testutils.NewStorageBuilder().
		WithFile("small.txt", content).
		Build()

	handler := NewFileHandler(storage, cfg, logger)
	req := httptest.NewRequest("GET", "/small.txt", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", w.Code)
		}
	}
}

func BenchmarkFileHandler_LargeFile(b *testing.B) {
	cfg := config.DefaultConfig()
	logger := config.NewLogger("error")

	// 1MB file
	content := strings.Repeat("x", 1024*1024)
	storage := testutils.NewStorageBuilder().
		WithFile("large.txt", content).
		Build()

	handler := NewFileHandler(storage, cfg, logger)
	req := httptest.NewRequest("GET", "/large.txt", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", w.Code)
		}
	}
}

func BenchmarkFileHandler_ContentTypeDetection(b *testing.B) {
	cfg := config.DefaultConfig()
	logger := config.NewLogger("error")

	storage := testutils.NewStorageBuilder().
		WithFile("test.html", "<html></html>").
		WithFile("test.css", "body{}").
		WithFile("test.js", "console.log()").
		WithFile("test.json", `{"key":"value"}`).
		WithFile("test.txt", "plain text").
		Build()

	handler := NewFileHandler(storage, cfg, logger)

	files := []string{"test.html", "test.css", "test.js", "test.json", "test.txt"}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		file := files[i%len(files)]
		req := httptest.NewRequest("GET", "/"+file, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", w.Code)
		}
	}
}

func BenchmarkFileHandler_ErrorResponse(b *testing.B) {
	cfg := config.DefaultConfig()
	logger := config.NewLogger("error")

	storage := testutils.NewStorageBuilder().Build() // Empty storage
	handler := NewFileHandler(storage, cfg, logger)

	req := httptest.NewRequest("GET", "/nonexistent.txt", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			b.Fatal("Expected error status, got 200")
		}
	}
}

func BenchmarkHealthHandler_HealthCheck(b *testing.B) {
	logger := config.NewLogger("error")
	storage := testutils.NewStorageBuilder().Build()
	handler := NewHealthHandler(storage, logger)

	req := httptest.NewRequest("GET", "/health", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", w.Code)
		}
	}
}

func BenchmarkFileHandler_ConcurrentRequests(b *testing.B) {
	cfg := config.DefaultConfig()
	logger := config.NewLogger("error")

	storage := testutils.NewStorageBuilder().
		WithFile("concurrent.txt", "Concurrent benchmark content").
		Build()

	handler := NewFileHandler(storage, cfg, logger)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("GET", "/concurrent.txt", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				b.Fatalf("Expected status 200, got %d", w.Code)
			}
		}
	})
}

func BenchmarkFileHandler_HeaderGeneration(b *testing.B) {
	cfg := config.DefaultConfig()
	logger := config.NewLogger("error")

	storage := testutils.NewStorageBuilder().
		WithFile("headers.txt", "Header generation benchmark").
		Build()

	handler := NewFileHandler(storage, cfg, logger)
	req := httptest.NewRequest("GET", "/headers.txt", nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", w.Code)
		}

		// Verify headers are set
		if w.Header().Get("ETag") == "" {
			b.Fatal("ETag header not set")
		}
		if w.Header().Get("Content-Type") == "" {
			b.Fatal("Content-Type header not set")
		}
	}
}

func BenchmarkMockStorage_Operations(b *testing.B) {
	storage := testutils.NewStorageBuilder().
		WithFile("bench.txt", "Storage benchmark content").
		Build()

	b.Run("GetFileInfo", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_, err := storage.GetFileInfo("bench.txt")
			if err != nil {
				b.Fatalf("GetFileInfo failed: %v", err)
			}
		}
	})

	b.Run("ReadFile", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_, err := storage.ReadFile("bench.txt")
			if err != nil {
				b.Fatalf("ReadFile failed: %v", err)
			}
		}
	})

	b.Run("FileExists", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			exists := storage.FileExists("bench.txt")
			if !exists {
				b.Fatal("File should exist")
			}
		}
	})
}
