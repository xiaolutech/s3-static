package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"s3-static/pkg/interfaces"
)

func TestOptimizedVariantResolverRequiresWebPAccept(t *testing.T) {
	cfg := optimizedTestConfig()
	source := &interfaces.FileInfo{Path: "photo.png", Size: 1024 * 1024, ETag: "source-etag", ContentType: "image/png"}
	optimizedBase := newMockStorage()
	optimized := &openFileMockStorage{mockStorage: optimizedBase}
	resolver := NewOptimizedVariantResolver(optimized, cfg)

	req := httptest.NewRequest(http.MethodGet, "/photo.png", nil)
	file, status := resolver.Resolve(context.Background(), req, source)

	if file != nil {
		t.Fatal("expected no optimized file")
	}
	if status.Code != optimizedStatusNotAccepted {
		t.Fatalf("expected not-accepted, got %#v", status)
	}
	if status.HeaderValue() != "" {
		t.Fatalf("expected empty header value for not-accepted, got %q", status.HeaderValue())
	}
	if optimized.openCalls != 0 {
		t.Fatalf("expected optimized storage not to be opened, got %d", optimized.openCalls)
	}
}

func TestOptimizedVariantResolverNeedsSourceInfo(t *testing.T) {
	cfg := optimizedTestConfig()
	resolver := NewOptimizedVariantResolver(newMockStorage(), cfg)

	req := httptest.NewRequest(http.MethodGet, "/photo.png", nil)
	req.Header.Set("Accept", "image/webp")
	if !resolver.NeedsSourceInfo(req) {
		t.Fatal("expected WebP GET to need source info")
	}

	req = httptest.NewRequest(http.MethodGet, "/photo.png", nil)
	if resolver.NeedsSourceInfo(req) {
		t.Fatal("expected request without WebP Accept not to need source info")
	}

	req = httptest.NewRequest(http.MethodGet, "/photo.png", nil)
	req.Header.Set("Accept", "image/webp")
	req.Header.Set("Range", "bytes=0-10")
	if resolver.NeedsSourceInfo(req) {
		t.Fatal("expected range request not to need optimized source info")
	}
}

func TestOptimizedVariantResolverReturnsTrustedWebPByDefault(t *testing.T) {
	cfg := optimizedTestConfig()
	cfg.OptimizationProfile = "v6-webp-q82-original"
	source := &interfaces.FileInfo{Path: "photo.png", Size: 1024 * 1024, ETag: "source-etag", ContentType: "image/png"}
	optimizedBase := newMockStorage()
	optimized := &openFileMockStorage{mockStorage: optimizedBase}
	key := optimizedVariantKey(source.Path, optimizedVariantWebP)
	optimizedBase.addFileWithMetadata(key, []byte("webp image"), time.Now().UTC(), "webp-etag", "image/webp", map[string]string{
		optimizedSourceKeyMetadata:         source.Path,
		optimizedSourceETagMetadata:        source.ETag,
		optimizedProfileMetadata:           cfg.OptimizationProfile,
		optimizedSourceContentTypeMetadata: source.ContentType,
		optimizedVariantFormatMetadata:     optimizedVariantWebP,
	})
	resolver := NewOptimizedVariantResolver(optimized, cfg)

	req := httptest.NewRequest(http.MethodGet, "/photo.png", nil)
	req.Header.Set("Accept", "image/webp,image/*,*/*")
	file, status := resolver.Resolve(context.Background(), req, source)
	defer file.Reader.Close()

	if status.Code != optimizedStatusHit {
		t.Fatalf("expected hit, got %#v", status)
	}
	if status.HeaderValue() != "hit; format=webp" {
		t.Fatalf("expected hit header value, got %q", status.HeaderValue())
	}
	if file.Info.ContentType != "image/webp" {
		t.Fatalf("expected image/webp, got %q", file.Info.ContentType)
	}
	if optimized.openCalls != 1 {
		t.Fatalf("expected one optimized open, got %d", optimized.openCalls)
	}
}

func TestOptimizedVariantResolverReturnsTrustedAVIFWhenEnabled(t *testing.T) {
	cfg := optimizedTestConfig()
	cfg.AVIFEnabled = true
	cfg.OptimizationProfile = "v7-avif-optional"
	source := &interfaces.FileInfo{Path: "photo.png", Size: 1024 * 1024, ETag: "source-etag", ContentType: "image/png"}
	optimizedBase := newMockStorage()
	optimized := &openFileMockStorage{mockStorage: optimizedBase}
	optimizedBase.addFileWithMetadata(optimizedVariantKey(source.Path, optimizedVariantAVIF), []byte("avif image"), time.Now().UTC(), "avif-etag", "image/avif", trustedAVIFMetadata(source, cfg.OptimizationProfile, nil))
	optimizedBase.addFileWithMetadata(optimizedVariantKey(source.Path, optimizedVariantWebP), []byte("webp image"), time.Now().UTC(), "webp-etag", "image/webp", trustedVariantMetadata(source, cfg.OptimizationProfile, optimizedVariantWebP, nil))
	resolver := NewOptimizedVariantResolver(optimized, cfg)

	req := httptest.NewRequest(http.MethodGet, "/photo.png", nil)
	req.Header.Set("Accept", "image/avif,image/webp,image/*,*/*")
	file, status := resolver.Resolve(context.Background(), req, source)
	defer file.Reader.Close()

	if status.HeaderValue() != "hit; format=avif" {
		t.Fatalf("expected AVIF hit header value, got %q", status.HeaderValue())
	}
	if file.Info.ContentType != "image/avif" {
		t.Fatalf("expected image/avif, got %q", file.Info.ContentType)
	}
	if optimized.openCalls != 1 {
		t.Fatalf("expected one optimized open, got %d", optimized.openCalls)
	}
}

func TestOptimizedVariantResolverFallbackStatuses(t *testing.T) {
	cfg := optimizedTestConfig()
	cfg.OptimizationProfile = "v6-webp-q82-original"
	source := &interfaces.FileInfo{Path: "photo.png", Size: 1024 * 1024, ETag: "source-etag", ContentType: "image/png"}

	tests := []struct {
		name           string
		contentType    string
		metadata       map[string]string
		expectedStatus string
	}{
		{
			name:           "missing webp object",
			expectedStatus: optimizedStatusMiss,
		},
		{
			name:        "stale source etag",
			contentType: "image/webp",
			metadata: trustedVariantMetadata(source, cfg.OptimizationProfile, optimizedVariantWebP, map[string]string{
				optimizedSourceETagMetadata: "old-etag",
			}),
			expectedStatus: optimizedStatusStale,
		},
		{
			name:        "profile mismatch",
			contentType: "image/webp",
			metadata: trustedVariantMetadata(source, cfg.OptimizationProfile, optimizedVariantWebP, map[string]string{
				optimizedProfileMetadata: "v3-avif-old",
			}),
			expectedStatus: optimizedStatusProfileMismatch,
		},
		{
			name:        "wrong variant format",
			contentType: "image/webp",
			metadata: trustedVariantMetadata(source, cfg.OptimizationProfile, optimizedVariantWebP, map[string]string{
				optimizedVariantFormatMetadata: "avif",
			}),
			expectedStatus: optimizedStatusStale,
		},
		{
			name:           "wrong optimized content type",
			contentType:    "image/avif",
			metadata:       trustedVariantMetadata(source, cfg.OptimizationProfile, optimizedVariantWebP, nil),
			expectedStatus: optimizedStatusStale,
		},
		{
			name:        "wrong source content type metadata",
			contentType: "image/webp",
			metadata: trustedVariantMetadata(source, cfg.OptimizationProfile, optimizedVariantWebP, map[string]string{
				optimizedSourceContentTypeMetadata: "image/jpeg",
			}),
			expectedStatus: optimizedStatusStale,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			optimizedBase := newMockStorage()
			optimized := &openFileMockStorage{mockStorage: optimizedBase}
			key := optimizedVariantKey(source.Path, optimizedVariantWebP)
			if tt.metadata != nil {
				optimizedBase.addFileWithMetadata(key, []byte("webp image"), time.Now().UTC(), "webp-etag", tt.contentType, tt.metadata)
			}
			resolver := NewOptimizedVariantResolver(optimized, cfg)

			req := httptest.NewRequest(http.MethodGet, "/photo.png", nil)
			req.Header.Set("Accept", "image/webp")
			file, status := resolver.Resolve(context.Background(), req, source)

			if file != nil {
				t.Fatal("expected no optimized file")
			}
			if status.Code != tt.expectedStatus {
				t.Fatalf("expected %q, got %#v", tt.expectedStatus, status)
			}
			if optimized.openCalls != 1 {
				t.Fatalf("expected one optimized open, got %d", optimized.openCalls)
			}
		})
	}
}

func TestOptimizedVariantResolverRejectsIneligibleSourceWithoutOpeningOptimizedStorage(t *testing.T) {
	cfg := optimizedTestConfig()
	source := &interfaces.FileInfo{Path: "document.pdf", Size: 1024 * 1024, ETag: "source-etag", ContentType: "application/pdf"}
	optimized := &openFileMockStorage{mockStorage: newMockStorage()}
	resolver := NewOptimizedVariantResolver(optimized, cfg)

	req := httptest.NewRequest(http.MethodGet, "/document.pdf", nil)
	req.Header.Set("Accept", "image/webp")
	file, status := resolver.Resolve(context.Background(), req, source)

	if file != nil {
		t.Fatal("expected no optimized file")
	}
	if status.Code != optimizedStatusNotAccepted {
		t.Fatalf("expected not-accepted, got %#v", status)
	}
	if optimized.openCalls != 0 {
		t.Fatalf("expected optimized storage not to be opened, got %d", optimized.openCalls)
	}
}

func TestOptimizedObjectContractVector(t *testing.T) {
	const sourceKey = "notes/photo.png"
	const expectedAVIFKey = "notes/photo.avif"
	const expectedWebPKey = "notes/photo.webp"

	if got := optimizedVariantKey(sourceKey, optimizedVariantAVIF); got != expectedAVIFKey {
		t.Fatalf("unexpected AVIF optimized key:\n got: %s\nwant: %s", got, expectedAVIFKey)
	}
	if got := optimizedVariantKey(sourceKey, optimizedVariantWebP); got != expectedWebPKey {
		t.Fatalf("unexpected WebP optimized key:\n got: %s\nwant: %s", got, expectedWebPKey)
	}

	expectedMetadataKeys := []string{
		"source-key",
		"source-etag",
		"optimization-profile",
		"source-content-type",
		"variant-format",
	}
	actualMetadataKeys := []string{
		optimizedSourceKeyMetadata,
		optimizedSourceETagMetadata,
		optimizedProfileMetadata,
		optimizedSourceContentTypeMetadata,
		optimizedVariantFormatMetadata,
	}
	for i := range expectedMetadataKeys {
		if actualMetadataKeys[i] != expectedMetadataKeys[i] {
			t.Fatalf("metadata key %d = %q, want %q", i, actualMetadataKeys[i], expectedMetadataKeys[i])
		}
	}
	if optimizedVariantAVIF != "avif" {
		t.Fatalf("variant format = %q, want avif", optimizedVariantAVIF)
	}
	if optimizedVariantWebP != "webp" {
		t.Fatalf("variant format = %q, want webp", optimizedVariantWebP)
	}
}

func TestAcceptsMediaType(t *testing.T) {
	tests := []struct {
		name   string
		accept string
		want   bool
	}{
		{name: "explicit avif", accept: "image/avif,image/webp,image/*,*/*", want: true},
		{name: "with quality", accept: "image/webp;q=0.9, image/avif;q=0.8", want: true},
		{name: "zero quality", accept: "image/avif;q=0", want: false},
		{name: "zero decimal quality", accept: "image/avif;q=0.00", want: false},
		{name: "wildcard only", accept: "image/*,*/*", want: false},
		{name: "empty", accept: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := acceptsMediaType(tt.accept, "image/avif"); got != tt.want {
				t.Fatalf("acceptsMediaType(%q, image/avif) = %t, want %t", tt.accept, got, tt.want)
			}
		})
	}
}

func trustedAVIFMetadata(source *interfaces.FileInfo, profile string, overrides map[string]string) map[string]string {
	return trustedVariantMetadata(source, profile, optimizedVariantAVIF, overrides)
}

func trustedVariantMetadata(source *interfaces.FileInfo, profile string, format string, overrides map[string]string) map[string]string {
	metadata := map[string]string{
		optimizedSourceKeyMetadata:         source.Path,
		optimizedSourceETagMetadata:        source.ETag,
		optimizedProfileMetadata:           profile,
		optimizedSourceContentTypeMetadata: source.ContentType,
		optimizedVariantFormatMetadata:     format,
	}
	for key, value := range overrides {
		metadata[key] = value
	}
	return metadata
}
