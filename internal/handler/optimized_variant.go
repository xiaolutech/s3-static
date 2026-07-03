package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"s3-static/internal/config"
	"s3-static/pkg/interfaces"
)

const (
	optimizedSourceKeyMetadata         = "source-key"
	optimizedSourceContentTypeMetadata = "source-content-type"
	optimizedVariantFormatMetadata     = "variant-format"
	optimizedVariantAVIF               = "avif"

	optimizedStatusHit             = "hit"
	optimizedStatusMiss            = "miss"
	optimizedStatusStale           = "stale"
	optimizedStatusProfileMismatch = "profile-mismatch"
	optimizedStatusNotAccepted     = "not-accepted"
)

type VariantStatus struct {
	Code   string
	Format string
}

func (s VariantStatus) HeaderValue() string {
	if s.Code == optimizedStatusHit && s.Format != "" {
		return s.Code + "; format=" + s.Format
	}
	return s.Code
}

type OptimizedVariantResolver struct {
	storage interfaces.Storage
	config  *config.Config
}

func NewOptimizedVariantResolver(storage interfaces.Storage, cfg *config.Config) *OptimizedVariantResolver {
	return &OptimizedVariantResolver{storage: storage, config: cfg}
}

func (r *OptimizedVariantResolver) Resolve(ctx context.Context, req *http.Request, source *interfaces.FileInfo) (*interfaces.OpenedFile, VariantStatus) {
	if r == nil || r.storage == nil || r.config == nil || source == nil || !acceptsAVIF(req.Header.Get("Accept")) {
		return nil, VariantStatus{Code: optimizedStatusNotAccepted}
	}

	key := avifOptimizedKey(source.Path, r.config.OptimizationProfile)
	opened, err := openFileFromBackend(ctx, r.storage, key)
	if err != nil {
		return nil, VariantStatus{Code: optimizedStatusMiss}
	}
	if !isTrustedAVIFVariant(opened.Info, source, r.config.OptimizationProfile) {
		_ = opened.Reader.Close()
		if opened.Info != nil && opened.Info.Metadata[optimizedProfileMetadata] != r.config.OptimizationProfile {
			return nil, VariantStatus{Code: optimizedStatusProfileMismatch}
		}
		return nil, VariantStatus{Code: optimizedStatusStale}
	}
	return opened, VariantStatus{Code: optimizedStatusHit, Format: optimizedVariantAVIF}
}

func isTrustedAVIFVariant(optimized, source *interfaces.FileInfo, profile string) bool {
	if optimized == nil || source == nil || optimized.Metadata == nil {
		return false
	}
	return optimized.ContentType == "image/avif" &&
		optimized.Metadata[optimizedSourceKeyMetadata] == source.Path &&
		optimized.Metadata[optimizedSourceETagMetadata] == source.ETag &&
		optimized.Metadata[optimizedProfileMetadata] == profile &&
		optimized.Metadata[optimizedSourceContentTypeMetadata] == source.ContentType &&
		optimized.Metadata[optimizedVariantFormatMetadata] == optimizedVariantAVIF
}

func acceptsAVIF(accept string) bool {
	for _, part := range strings.Split(accept, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		mediaType, params, err := mime.ParseMediaType(part)
		if err != nil {
			mediaType = strings.TrimSpace(strings.Split(part, ";")[0])
			params = nil
		}
		if strings.EqualFold(mediaType, "image/avif") && acceptQuality(params["q"]) > 0 {
			return true
		}
	}
	return false
}

func acceptQuality(raw string) float64 {
	if raw == "" {
		return 1
	}
	q, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 1
	}
	return q
}

func avifOptimizedKey(sourceKey, profile string) string {
	sum := sha256.Sum256([]byte(sourceKey))
	return ".s3-image-optimizer/avif/" + hex.EncodeToString(sum[:]) + "/" + profile + "/image.avif"
}

func openFileFromBackend(ctx context.Context, backend interfaces.Storage, path string) (*interfaces.OpenedFile, error) {
	if opener, ok := backend.(interfaces.FileOpener); ok {
		return opener.OpenFileContext(ctx, path)
	}

	var info *interfaces.FileInfo
	var err error
	if storageWithContext, ok := backend.(interfaces.ContextStorage); ok {
		info, err = storageWithContext.GetFileInfoContext(ctx, path)
	} else {
		info, err = backend.GetFileInfo(path)
	}
	if err != nil {
		return nil, err
	}

	var reader io.ReadSeekCloser
	if storageWithContext, ok := backend.(interfaces.ContextStorage); ok {
		reader, err = storageWithContext.GetFileReaderContext(ctx, path)
	} else {
		reader, err = backend.GetFileReader(path)
	}
	if err != nil {
		return nil, err
	}

	return &interfaces.OpenedFile{Info: info, Reader: reader}, nil
}
