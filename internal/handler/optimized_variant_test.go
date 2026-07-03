package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"s3-static/pkg/interfaces"
)

func TestOptimizedVariantResolverRequiresAVIFAccept(t *testing.T) {
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
	req.Header.Set("Accept", "image/avif")
	if !resolver.NeedsSourceInfo(req) {
		t.Fatal("expected AVIF GET to need source info")
	}

	req = httptest.NewRequest(http.MethodGet, "/photo.png", nil)
	if resolver.NeedsSourceInfo(req) {
		t.Fatal("expected request without AVIF Accept not to need source info")
	}

	req = httptest.NewRequest(http.MethodGet, "/photo.png", nil)
	req.Header.Set("Accept", "image/avif")
	req.Header.Set("Range", "bytes=0-10")
	if resolver.NeedsSourceInfo(req) {
		t.Fatal("expected range request not to need optimized source info")
	}
}

func TestOptimizedVariantResolverReturnsTrustedAVIF(t *testing.T) {
	cfg := optimizedTestConfig()
	cfg.OptimizationProfile = "v4-avif-target1m-original"
	source := &interfaces.FileInfo{Path: "photo.png", Size: 1024 * 1024, ETag: "source-etag", ContentType: "image/png"}
	optimizedBase := newMockStorage()
	optimized := &openFileMockStorage{mockStorage: optimizedBase}
	key := avifOptimizedKey(source.Path, cfg.OptimizationProfile)
	optimizedBase.addFileWithMetadata(key, []byte("avif image"), time.Now().UTC(), "avif-etag", "image/avif", map[string]string{
		optimizedSourceKeyMetadata:         source.Path,
		optimizedSourceETagMetadata:        source.ETag,
		optimizedProfileMetadata:           cfg.OptimizationProfile,
		optimizedSourceContentTypeMetadata: source.ContentType,
		optimizedVariantFormatMetadata:     optimizedVariantAVIF,
	})
	resolver := NewOptimizedVariantResolver(optimized, cfg)

	req := httptest.NewRequest(http.MethodGet, "/photo.png", nil)
	req.Header.Set("Accept", "image/avif,image/webp,image/*,*/*")
	file, status := resolver.Resolve(context.Background(), req, source)
	defer file.Reader.Close()

	if status.Code != optimizedStatusHit {
		t.Fatalf("expected hit, got %#v", status)
	}
	if status.HeaderValue() != "hit; format=avif" {
		t.Fatalf("expected hit header value, got %q", status.HeaderValue())
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
	cfg.OptimizationProfile = "v4-avif-target1m-original"
	source := &interfaces.FileInfo{Path: "photo.png", Size: 1024 * 1024, ETag: "source-etag", ContentType: "image/png"}

	tests := []struct {
		name           string
		contentType    string
		metadata       map[string]string
		expectedStatus string
	}{
		{
			name:           "missing avif object",
			expectedStatus: optimizedStatusMiss,
		},
		{
			name:        "stale source etag",
			contentType: "image/avif",
			metadata: trustedAVIFMetadata(source, cfg.OptimizationProfile, map[string]string{
				optimizedSourceETagMetadata: "old-etag",
			}),
			expectedStatus: optimizedStatusStale,
		},
		{
			name:        "profile mismatch",
			contentType: "image/avif",
			metadata: trustedAVIFMetadata(source, cfg.OptimizationProfile, map[string]string{
				optimizedProfileMetadata: "v3-avif-old",
			}),
			expectedStatus: optimizedStatusProfileMismatch,
		},
		{
			name:        "wrong variant format",
			contentType: "image/avif",
			metadata: trustedAVIFMetadata(source, cfg.OptimizationProfile, map[string]string{
				optimizedVariantFormatMetadata: "webp",
			}),
			expectedStatus: optimizedStatusStale,
		},
		{
			name:           "wrong optimized content type",
			contentType:    "image/webp",
			metadata:       trustedAVIFMetadata(source, cfg.OptimizationProfile, nil),
			expectedStatus: optimizedStatusStale,
		},
		{
			name:        "wrong source content type metadata",
			contentType: "image/avif",
			metadata: trustedAVIFMetadata(source, cfg.OptimizationProfile, map[string]string{
				optimizedSourceContentTypeMetadata: "image/jpeg",
			}),
			expectedStatus: optimizedStatusStale,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			optimizedBase := newMockStorage()
			optimized := &openFileMockStorage{mockStorage: optimizedBase}
			key := avifOptimizedKey(source.Path, cfg.OptimizationProfile)
			if tt.metadata != nil {
				optimizedBase.addFileWithMetadata(key, []byte("avif image"), time.Now().UTC(), "avif-etag", tt.contentType, tt.metadata)
			}
			resolver := NewOptimizedVariantResolver(optimized, cfg)

			req := httptest.NewRequest(http.MethodGet, "/photo.png", nil)
			req.Header.Set("Accept", "image/avif")
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
	req.Header.Set("Accept", "image/avif")
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

func TestAVIFOptimizedObjectContractVector(t *testing.T) {
	const sourceKey = "notes/photo.png"
	const profile = "v4-avif-target1m-original"
	const expectedKey = ".s3-image-optimizer/avif/905b8d229b111ac9fe99f099872a2fcda398a8b06005c36412154b5dd19c85f4/v4-avif-target1m-original/image.avif"

	if got := avifOptimizedKey(sourceKey, profile); got != expectedKey {
		t.Fatalf("unexpected AVIF optimized key:\n got: %s\nwant: %s", got, expectedKey)
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
}

func TestAcceptsAVIF(t *testing.T) {
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
			if got := acceptsAVIF(tt.accept); got != tt.want {
				t.Fatalf("acceptsAVIF(%q) = %t, want %t", tt.accept, got, tt.want)
			}
		})
	}
}

func trustedAVIFMetadata(source *interfaces.FileInfo, profile string, overrides map[string]string) map[string]string {
	metadata := map[string]string{
		optimizedSourceKeyMetadata:         source.Path,
		optimizedSourceETagMetadata:        source.ETag,
		optimizedProfileMetadata:           profile,
		optimizedSourceContentTypeMetadata: source.ContentType,
		optimizedVariantFormatMetadata:     optimizedVariantAVIF,
	}
	for key, value := range overrides {
		metadata[key] = value
	}
	return metadata
}
