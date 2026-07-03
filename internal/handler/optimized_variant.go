package handler

import (
	"context"
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
	optimizedVariantWebP               = "webp"

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
	if s.Code == "" || s.Code == optimizedStatusNotAccepted {
		return ""
	}
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

func shouldForceSourceVariant(req *http.Request) bool {
	if req == nil {
		return false
	}
	variant := strings.ToLower(strings.TrimSpace(req.URL.Query().Get("variant")))
	return variant == "source" || variant == "original"
}

func (r *OptimizedVariantResolver) NeedsSourceInfo(req *http.Request) bool {
	return r != nil &&
		r.storage != nil &&
		r.config != nil &&
		r.config.OptimizedImageEnabled &&
		req != nil &&
		(req.Method == http.MethodGet || req.Method == http.MethodHead) &&
		!shouldForceSourceVariant(req) &&
		req.Header.Get("Range") == "" &&
		len(r.acceptedVariants(req)) > 0
}

func (r *OptimizedVariantResolver) Resolve(ctx context.Context, req *http.Request, source *interfaces.FileInfo) (*interfaces.OpenedFile, VariantStatus) {
	if !r.NeedsSourceInfo(req) || !r.canResolveSource(source) {
		return nil, VariantStatus{Code: optimizedStatusNotAccepted}
	}

	status := VariantStatus{Code: optimizedStatusMiss}
	for _, variant := range r.acceptedVariants(req) {
		key := optimizedVariantKey(source.Path, variant.Format)
		opened, err := openFileFromBackend(ctx, r.storage, key)
		if err != nil {
			continue
		}
		if isTrustedVariant(opened.Info, source, r.config.OptimizationProfile, variant) {
			return opened, VariantStatus{Code: optimizedStatusHit, Format: variant.Format}
		}

		_ = opened.Reader.Close()
		if opened.Info != nil && opened.Info.Metadata != nil {
			if profile, ok := opened.Info.Metadata[optimizedProfileMetadata]; ok && profile != r.config.OptimizationProfile {
				status = VariantStatus{Code: optimizedStatusProfileMismatch}
				continue
			}
		}
		if status.Code == optimizedStatusMiss {
			status = VariantStatus{Code: optimizedStatusStale}
		}
	}
	return nil, status
}

type optimizedVariant struct {
	Format      string
	ContentType string
}

func (r *OptimizedVariantResolver) acceptedVariants(req *http.Request) []optimizedVariant {
	if req == nil {
		return nil
	}
	accept := req.Header.Get("Accept")
	var variants []optimizedVariant
	if r.config != nil && r.config.AVIFEnabled && acceptsMediaType(accept, "image/avif") {
		variants = append(variants, optimizedVariant{Format: optimizedVariantAVIF, ContentType: "image/avif"})
	}
	if acceptsMediaType(accept, "image/webp") {
		variants = append(variants, optimizedVariant{Format: optimizedVariantWebP, ContentType: "image/webp"})
	}
	return variants
}

func (r *OptimizedVariantResolver) canResolveSource(source *interfaces.FileInfo) bool {
	if source == nil || source.Size < r.config.OptimizedMinBytes {
		return false
	}

	switch contentMediaType(source.ContentType) {
	case "image/jpeg", "image/png":
		return true
	}

	switch strings.ToLower(fileExtension(source.Path)) {
	case ".jpg", ".jpeg", ".png":
		return true
	default:
		return false
	}
}

func isTrustedVariant(optimized, source *interfaces.FileInfo, profile string, variant optimizedVariant) bool {
	if optimized == nil || source == nil || optimized.Metadata == nil {
		return false
	}
	return optimized.ContentType == variant.ContentType &&
		optimized.Metadata[optimizedSourceKeyMetadata] == source.Path &&
		optimized.Metadata[optimizedSourceETagMetadata] == source.ETag &&
		optimized.Metadata[optimizedProfileMetadata] == profile &&
		optimized.Metadata[optimizedSourceContentTypeMetadata] == source.ContentType &&
		optimized.Metadata[optimizedVariantFormatMetadata] == variant.Format
}

func acceptsMediaType(accept string, want string) bool {
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
		if strings.EqualFold(mediaType, want) && acceptQuality(params["q"]) > 0 {
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

func optimizedVariantKey(sourceKey, format string) string {
	return sourceKey + "." + format
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
