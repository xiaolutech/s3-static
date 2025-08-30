package main

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// BenchmarkFileServing benchmarks basic file serving performance
func BenchmarkFileServing(b *testing.B) {
	suite := SetupTestSuite(&testing.T{})
	defer suite.Cleanup()

	// Upload test file
	testContent := "Benchmark test content"
	err := suite.UploadTestFile("benchmark.txt", testContent)
	if err != nil {
		b.Fatalf("Failed to upload test file: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := suite.CreateTestRequest("GET", "/benchmark.txt", nil)
		w := suite.ExecuteRequest(req)
		if w.Code != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", w.Code)
		}
	}
}

// BenchmarkConditionalRequests benchmarks conditional request performance
func BenchmarkConditionalRequests(b *testing.B) {
	suite := SetupTestSuite(&testing.T{})
	defer suite.Cleanup()

	// Upload test file
	testContent := "Conditional request benchmark"
	err := suite.UploadTestFile("conditional-bench.txt", testContent)
	if err != nil {
		b.Fatalf("Failed to upload test file: %v", err)
	}

	// Get ETag for conditional requests
	req := suite.CreateTestRequest("GET", "/conditional-bench.txt", nil)
	w := suite.ExecuteRequest(req)
	etag := w.Header().Get("ETag")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := suite.CreateTestRequest("GET", "/conditional-bench.txt", nil)
		req.Header.Set("If-None-Match", etag)
		w := suite.ExecuteRequest(req)
		if w.Code != http.StatusNotModified {
			b.Fatalf("Expected status 304, got %d", w.Code)
		}
	}
}

// BenchmarkSmallFiles benchmarks serving small files
func BenchmarkSmallFiles(b *testing.B) {
	suite := SetupTestSuite(&testing.T{})
	defer suite.Cleanup()

	// Upload small test file (1KB)
	smallContent := strings.Repeat("x", 1024)
	err := suite.UploadTestFile("small.txt", smallContent)
	if err != nil {
		b.Fatalf("Failed to upload test file: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := suite.CreateTestRequest("GET", "/small.txt", nil)
		w := suite.ExecuteRequest(req)
		if w.Code != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", w.Code)
		}
	}
}

// BenchmarkMediumFiles benchmarks serving medium files
func BenchmarkMediumFiles(b *testing.B) {
	suite := SetupTestSuite(&testing.T{})
	defer suite.Cleanup()

	// Upload medium test file (100KB)
	mediumContent := strings.Repeat("x", 100*1024)
	err := suite.UploadTestFile("medium.txt", mediumContent)
	if err != nil {
		b.Fatalf("Failed to upload test file: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := suite.CreateTestRequest("GET", "/medium.txt", nil)
		w := suite.ExecuteRequest(req)
		if w.Code != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", w.Code)
		}
	}
}

// BenchmarkLargeFiles benchmarks serving large files
func BenchmarkLargeFiles(b *testing.B) {
	suite := SetupTestSuite(&testing.T{})
	defer suite.Cleanup()

	// Upload large test file (1MB)
	largeContent := strings.Repeat("x", 1024*1024)
	err := suite.UploadTestFile("large.txt", largeContent)
	if err != nil {
		b.Fatalf("Failed to upload test file: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := suite.CreateTestRequest("GET", "/large.txt", nil)
		w := suite.ExecuteRequest(req)
		if w.Code != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", w.Code)
		}
	}
}

// BenchmarkConcurrentRequests benchmarks concurrent request handling
func BenchmarkConcurrentRequests(b *testing.B) {
	suite := SetupTestSuite(&testing.T{})
	defer suite.Cleanup()

	// Upload test file
	testContent := "Concurrent benchmark content"
	err := suite.UploadTestFile("concurrent-bench.txt", testContent)
	if err != nil {
		b.Fatalf("Failed to upload test file: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := suite.CreateTestRequest("GET", "/concurrent-bench.txt", nil)
			w := suite.ExecuteRequest(req)
			if w.Code != http.StatusOK {
				b.Fatalf("Expected status 200, got %d", w.Code)
			}
		}
	})
}

// BenchmarkContentTypeDetection benchmarks content type detection
func BenchmarkContentTypeDetection(b *testing.B) {
	suite := SetupTestSuite(&testing.T{})
	defer suite.Cleanup()

	// Upload files with different extensions
	testFiles := map[string]string{
		"test.html": "<html></html>",
		"test.css":  "body{}",
		"test.js":   "console.log()",
		"test.json": `{"key":"value"}`,
		"test.txt":  "plain text",
		"test.png":  "fake-png",
		"test.jpg":  "fake-jpg",
	}

	for path, content := range testFiles {
		err := suite.UploadTestFile(path, content)
		if err != nil {
			b.Fatalf("Failed to upload test file %s: %v", path, err)
		}
	}

	paths := make([]string, 0, len(testFiles))
	for path := range testFiles {
		paths = append(paths, path)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		path := paths[i%len(paths)]
		req := suite.CreateTestRequest("GET", "/"+path, nil)
		w := suite.ExecuteRequest(req)
		if w.Code != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", w.Code)
		}
	}
}

// BenchmarkETagGeneration benchmarks ETag handling
func BenchmarkETagGeneration(b *testing.B) {
	suite := SetupTestSuite(&testing.T{})
	defer suite.Cleanup()

	// Upload test file
	testContent := "ETag benchmark content"
	err := suite.UploadTestFile("etag-bench.txt", testContent)
	if err != nil {
		b.Fatalf("Failed to upload test file: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := suite.CreateTestRequest("GET", "/etag-bench.txt", nil)
		w := suite.ExecuteRequest(req)
		if w.Code != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", w.Code)
		}
		if w.Header().Get("ETag") == "" {
			b.Fatal("Expected ETag header")
		}
	}
}

// BenchmarkErrorHandling benchmarks error response generation
func BenchmarkErrorHandling(b *testing.B) {
	suite := SetupTestSuite(&testing.T{})
	defer suite.Cleanup()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := suite.CreateTestRequest("GET", "/nonexistent.txt", nil)
		w := suite.ExecuteRequest(req)
		if w.Code == http.StatusOK {
			b.Fatal("Expected error status, got 200")
		}
	}
}

// BenchmarkMemoryUsage benchmarks memory usage patterns
func BenchmarkMemoryUsage(b *testing.B) {
	suite := SetupTestSuite(&testing.T{})
	defer suite.Cleanup()

	// Upload files of different sizes
	sizes := []int{1024, 10240, 102400, 1048576} // 1KB, 10KB, 100KB, 1MB
	for i, size := range sizes {
		content := strings.Repeat("x", size)
		filename := fmt.Sprintf("memory-test-%d.txt", i)
		err := suite.UploadTestFile(filename, content)
		if err != nil {
			b.Fatalf("Failed to upload test file: %v", err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		fileIndex := i % len(sizes)
		filename := fmt.Sprintf("memory-test-%d.txt", fileIndex)
		req := suite.CreateTestRequest("GET", "/"+filename, nil)
		w := suite.ExecuteRequest(req)
		if w.Code != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", w.Code)
		}
	}
}

// BenchmarkS3Operations benchmarks S3 storage operations
func BenchmarkS3Operations(b *testing.B) {
	suite := SetupTestSuite(&testing.T{})
	defer suite.Cleanup()

	// Upload test file
	testContent := "S3 operations benchmark"
	err := suite.UploadTestFile("s3-ops.txt", testContent)
	if err != nil {
		b.Fatalf("Failed to upload test file: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.Run("GetFileInfo", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := suite.Storage.GetFileInfo("s3-ops.txt")
			if err != nil {
				b.Fatalf("GetFileInfo failed: %v", err)
			}
		}
	})

	b.Run("ReadFile", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := suite.Storage.ReadFile("s3-ops.txt")
			if err != nil {
				b.Fatalf("ReadFile failed: %v", err)
			}
		}
	})

	b.Run("FileExists", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			exists := suite.Storage.FileExists("s3-ops.txt")
			if !exists {
				b.Fatal("File should exist")
			}
		}
	})
}

// BenchmarkRequestThroughput measures request throughput under load
func BenchmarkRequestThroughput(b *testing.B) {
	suite := SetupTestSuite(&testing.T{})
	defer suite.Cleanup()

	// Upload test file
	testContent := "Throughput test content"
	err := suite.UploadTestFile("throughput.txt", testContent)
	if err != nil {
		b.Fatalf("Failed to upload test file: %v", err)
	}

	// Test different concurrency levels
	concurrencyLevels := []int{1, 5, 10, 20, 50}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency-%d", concurrency), func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			var wg sync.WaitGroup
			requests := make(chan struct{}, b.N)

			// Fill the requests channel
			for i := 0; i < b.N; i++ {
				requests <- struct{}{}
			}
			close(requests)

			// Start workers
			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for range requests {
						req := suite.CreateTestRequest("GET", "/throughput.txt", nil)
						w := suite.ExecuteRequest(req)
						if w.Code != http.StatusOK {
							b.Errorf("Expected status 200, got %d", w.Code)
						}
					}
				}()
			}

			wg.Wait()
		})
	}
}

// BenchmarkLatency measures request latency
func BenchmarkLatency(b *testing.B) {
	suite := SetupTestSuite(&testing.T{})
	defer suite.Cleanup()

	// Upload test file
	testContent := "Latency test content"
	err := suite.UploadTestFile("latency.txt", testContent)
	if err != nil {
		b.Fatalf("Failed to upload test file: %v", err)
	}

	latencies := make([]time.Duration, b.N)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		start := time.Now()
		req := suite.CreateTestRequest("GET", "/latency.txt", nil)
		w := suite.ExecuteRequest(req)
		latency := time.Since(start)

		if w.Code != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", w.Code)
		}

		latencies[i] = latency
	}

	// Calculate statistics
	if b.N > 0 {
		var total time.Duration
		min := latencies[0]
		max := latencies[0]

		for _, lat := range latencies {
			total += lat
			if lat < min {
				min = lat
			}
			if lat > max {
				max = lat
			}
		}

		avg := total / time.Duration(b.N)
		b.Logf("Latency - Min: %v, Max: %v, Avg: %v", min, max, avg)
	}
}
