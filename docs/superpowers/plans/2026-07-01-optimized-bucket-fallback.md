# Optimized Bucket Fallback Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Preserve existing public asset URLs while allowing `s3-static` to serve trusted optimized image copies from a separate S3 bucket.

**Architecture:** `s3-static` remains a static-object serving gateway. It never resizes, re-encodes, scans buckets, or writes optimized objects. For ordinary image `GET` requests, it can check a configured optimized bucket for the same object key and serve that object only when metadata proves it was generated from the current source ETag and current optimization profile.

**Tech Stack:** Go 1.25, AWS SDK for Go v2, existing `net/http` handler, MinIO/S3-compatible object metadata, existing unit/integration test patterns.

## Implementation Status

- [x] Task 1: Add optimized serving config.
- [x] Task 2: Surface S3 user metadata.
- [x] Task 3: Add alternate bucket storage factory.
- [x] Task 4: Serve trusted optimized copies from handler.
- [x] Task 5: Wire optimized storage in `cmd/s3-static/main.go`.
- [x] Task 6: Document external optimizer contract in `README.md`.

---

## Scope And Non-Goals

This plan is only for `/Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static`.

In scope:
- Add optional optimized-bucket config.
- Add support for creating a second S3 storage instance pointed at the optimized bucket.
- For normal `GET` image requests without `Range`, serve an optimized object from the same key when trusted.
- Trust an optimized object only when it has:
  - `x-amz-meta-source-etag` equal to the current source object ETag.
  - `x-amz-meta-optimization-profile` equal to the configured profile.
- Keep all existing fallback behavior.

Out of scope:
- No optimizer worker in this repo.
- No image encoding, resizing, or compression libraries in this repo.
- No bucket scanning or S3 object writes in this repo.
- No WebP/AVIF content negotiation in this repo.
- No deployment changes in this repo beyond documenting the config. Deployment wiring belongs in `my-services`.

## File Structure

Modify:
- `/Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static/internal/config/config.go`
  - Add optimized serving config and validation.
- `/Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static/internal/config/config_test.go`
  - Cover defaults, env loading, and validation.
- `/Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static/pkg/interfaces/storage.go`
  - Add metadata support to `FileInfo` so handlers can validate optimized objects without depending on S3 internals.
- `/Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static/internal/storage/s3.go`
  - Populate user metadata from S3 `HeadObject` and `GetObject`.
- `/Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static/internal/storage/s3_test.go`
  - Verify metadata is surfaced from MinIO/S3.
- `/Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static/internal/storage/factory.go`
  - Add helper to create a storage instance for an alternate bucket.
- `/Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static/internal/handler/handler.go`
  - Add optimized-first serving logic with strict fallback.
- `/Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static/internal/handler/handler_test.go`
  - Cover hit, stale optimized copy, profile mismatch, missing optimized copy, `HEAD`, `?meta=1`, `Range`, and non-image behavior.
- `/Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static/internal/testutils/mocks.go`
  - Allow test fixtures to set `FileInfo.Metadata`.
- `/Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static/cmd/s3-static/main.go`
  - Wire optional optimized storage.
- `/Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static/README.md`
  - Document the optimized bucket contract for external workers.

## Metadata Contract

An external optimizer writes optimized copies to a different bucket using the same key as the source object.

Source object:

```text
bucket: logseq-assets
key: notes/a-big-photo.jpg
etag: abc123
```

Optimized object:

```text
bucket: logseq-assets-optimized
key: notes/a-big-photo.jpg
x-amz-meta-source-etag: abc123
x-amz-meta-optimization-profile: v1-jpeg82-png-best-w1920
```

`s3-static` may serve the optimized object only when both metadata values match the source object and current config.

## Task 1: Add Optimized Serving Config

**Files:**
- Modify: `/Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static/internal/config/config.go`
- Modify: `/Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static/internal/config/config_test.go`

- [ ] **Step 1: Write failing tests for defaults**

Add these assertions to `TestDefaultConfig`:

```go
if config.OptimizedImageEnabled {
	t.Error("Expected optimized image serving disabled by default")
}
if config.OptimizedBucketName != "" {
	t.Errorf("Expected empty optimized bucket by default, got %q", config.OptimizedBucketName)
}
if config.OptimizationProfile != "v1-jpeg82-png-best-w1920" {
	t.Errorf("Expected default optimization profile, got %q", config.OptimizationProfile)
}
if config.OptimizedMinBytes != 524288 {
	t.Errorf("Expected optimized min bytes 524288, got %d", config.OptimizedMinBytes)
}
```

- [ ] **Step 2: Write failing env-loading test**

Add to `TestLoadFromEnv_WithEnvironmentVariables`:

```go
os.Setenv("OPTIMIZED_IMAGE_ENABLED", "true")
os.Setenv("OPTIMIZED_BUCKET_NAME", "optimized-assets")
os.Setenv("OPTIMIZATION_PROFILE", "v2-jpeg76-w2560")
os.Setenv("OPTIMIZED_MIN_BYTES", "262144")
```

Add assertions:

```go
if !config.OptimizedImageEnabled {
	t.Error("Expected optimized image serving enabled")
}
if config.OptimizedBucketName != "optimized-assets" {
	t.Errorf("Expected optimized bucket 'optimized-assets', got %q", config.OptimizedBucketName)
}
if config.OptimizationProfile != "v2-jpeg76-w2560" {
	t.Errorf("Expected profile 'v2-jpeg76-w2560', got %q", config.OptimizationProfile)
}
if config.OptimizedMinBytes != 262144 {
	t.Errorf("Expected min bytes 262144, got %d", config.OptimizedMinBytes)
}
```

- [ ] **Step 3: Run tests and verify failure**

Run:

```bash
cd /Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static
go test ./internal/config -run 'TestDefaultConfig|TestLoadFromEnv_WithEnvironmentVariables' -count=1
```

Expected: FAIL because the new fields do not exist.

- [ ] **Step 4: Implement config fields**

Add to `Config` in `config.go`:

```go
OptimizedImageEnabled bool   `env:"OPTIMIZED_IMAGE_ENABLED"`
OptimizedBucketName   string `env:"OPTIMIZED_BUCKET_NAME"`
OptimizationProfile   string `env:"OPTIMIZATION_PROFILE"`
OptimizedMinBytes     int64  `env:"OPTIMIZED_MIN_BYTES"`
```

Add defaults in `DefaultConfig()`:

```go
OptimizedImageEnabled: false,
OptimizedBucketName:   "",
OptimizationProfile:   "v1-jpeg82-png-best-w1920",
OptimizedMinBytes:     512 * 1024,
```

Load env vars in `LoadFromEnv()` using `strconv.ParseBool` and `strconv.ParseInt`. Extend `clearEnvVars()` in `config_test.go` with the new env var names.

- [ ] **Step 5: Add validation tests**

Add test cases:

```go
func TestValidate_OptimizedServingRequiresBucket(t *testing.T) {
	config := DefaultConfig()
	config.OptimizedImageEnabled = true
	err := config.Validate()
	if err == nil {
		t.Fatal("Expected error when optimized serving is enabled without optimized bucket")
	}
}

func TestValidate_OptimizedServingValid(t *testing.T) {
	config := DefaultConfig()
	config.OptimizedImageEnabled = true
	config.OptimizedBucketName = "optimized-assets"
	err := config.Validate()
	if err != nil {
		t.Fatalf("Expected valid optimized config, got %v", err)
	}
}

func TestValidate_OptimizedMinBytesCannotBeNegative(t *testing.T) {
	config := DefaultConfig()
	config.OptimizedMinBytes = -1
	err := config.Validate()
	if err == nil {
		t.Fatal("Expected error for negative optimized min bytes")
	}
}
```

- [ ] **Step 6: Implement validation**

Add to `Validate()`:

```go
if c.OptimizedImageEnabled && c.OptimizedBucketName == "" {
	return fmt.Errorf("OPTIMIZED_BUCKET_NAME is required when OPTIMIZED_IMAGE_ENABLED is true")
}
if c.OptimizedMinBytes < 0 {
	return fmt.Errorf("OPTIMIZED_MIN_BYTES cannot be negative, got: %d", c.OptimizedMinBytes)
}
if c.OptimizationProfile == "" {
	return fmt.Errorf("OPTIMIZATION_PROFILE cannot be empty")
}
```

- [ ] **Step 7: Run config tests**

Run:

```bash
cd /Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static
go test ./internal/config -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit**

Run:

```bash
cd /Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: add optimized image serving config"
```

## Task 2: Surface S3 User Metadata

**Files:**
- Modify: `/Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static/pkg/interfaces/storage.go`
- Modify: `/Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static/internal/storage/s3.go`
- Modify: `/Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static/internal/storage/s3_test.go`
- Modify: `/Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static/internal/testutils/mocks.go`

- [ ] **Step 1: Add failing S3 metadata test**

In `s3_test.go`, add:

```go
func TestS3Storage_GetFileInfoIncludesMetadata(t *testing.T) {
	container, storage := setupMinIOContainer(t)
	defer container.Terminate(context.Background())

	_, err := storage.client.PutObject(context.Background(), &awss3.PutObjectInput{
		Bucket:      aws.String(testBucket),
		Key:         aws.String("optimized/photo.jpg"),
		Body:        strings.NewReader("content"),
		ContentType: aws.String("image/jpeg"),
		Metadata: map[string]string{
			"source-etag":          "source-123",
			"optimization-profile": "v1-jpeg82-png-best-w1920",
		},
	})
	if err != nil {
		t.Fatalf("Failed to upload object: %v", err)
	}

	info, err := storage.GetFileInfo("optimized/photo.jpg")
	if err != nil {
		t.Fatalf("GetFileInfo failed: %v", err)
	}
	if info.Metadata["source-etag"] != "source-123" {
		t.Fatalf("Expected source-etag metadata, got %#v", info.Metadata)
	}
	if info.Metadata["optimization-profile"] != "v1-jpeg82-png-best-w1920" {
		t.Fatalf("Expected optimization-profile metadata, got %#v", info.Metadata)
	}
}
```

- [ ] **Step 2: Run test and verify failure**

Run:

```bash
cd /Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static
go test ./internal/storage -run TestS3Storage_GetFileInfoIncludesMetadata -count=1
```

Expected: FAIL because `FileInfo.Metadata` does not exist.

- [ ] **Step 3: Add metadata field**

Add to `pkg/interfaces/storage.go`:

```go
Metadata map[string]string // S3-compatible user metadata without x-amz-meta- prefix
```

in `FileInfo`.

- [ ] **Step 4: Populate metadata from S3**

In `internal/storage/s3.go`, add:

```go
func normalizeMetadata(metadata map[string]string) map[string]string {
	if len(metadata) == 0 {
		return nil
	}
	result := make(map[string]string, len(metadata))
	for key, value := range metadata {
		result[strings.ToLower(key)] = value
	}
	return result
}
```

Set `Metadata: normalizeMetadata(objInfo.Metadata)` in both `fileInfoFromHeadOutput` and `fileInfoFromGetOutput`.

- [ ] **Step 5: Extend test mock helpers**

In `internal/testutils/mocks.go`, add:

```go
func (m *MockStorage) AddFileWithMetadata(path string, content []byte, modTime time.Time, etag string, metadata map[string]string) {
	m.AddFile(path, content, modTime, etag)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[path].Metadata = metadata
}
```

If `handler_test.go` uses its local `mockStorage`, add a matching local helper there instead of forcing handler tests to switch to shared testutils.

- [ ] **Step 6: Run storage and package tests**

Run:

```bash
cd /Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static
go test ./pkg/... ./internal/storage ./internal/testutils -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

Run:

```bash
cd /Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static
git add pkg/interfaces/storage.go internal/storage/s3.go internal/storage/s3_test.go internal/testutils/mocks.go
git commit -m "feat: expose s3 user metadata"
```

## Task 3: Add Alternate Bucket Storage Factory

**Files:**
- Modify: `/Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static/internal/storage/factory.go`
- Modify: `/Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static/internal/storage/factory_test.go`

- [ ] **Step 1: Add failing factory tests**

Add:

```go
func TestNewS3StorageForBucketRejectsEmptyBucket(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.S3Endpoint = "localhost:9000"
	cfg.S3AccessKeyID = "key"
	cfg.S3SecretAccessKey = "secret"
	cfg.S3Region = "us-east-1"

	_, err := NewS3StorageForBucket(cfg, "")
	if err == nil {
		t.Fatal("Expected error for empty bucket")
	}
}
```

Do not add a test that dials a fake S3 server here; existing S3 integration tests cover actual S3 behavior.

- [ ] **Step 2: Run test and verify failure**

Run:

```bash
cd /Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static
go test ./internal/storage -run TestNewS3StorageForBucketRejectsEmptyBucket -count=1
```

Expected: FAIL because `NewS3StorageForBucket` does not exist.

- [ ] **Step 3: Implement helper**

Add to `factory.go`:

```go
func NewS3StorageForBucket(cfg *config.Config, bucket string) (*S3Storage, error) {
	if bucket == "" {
		return nil, fmt.Errorf("bucket cannot be empty")
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	s3Config := S3Config{
		Endpoint:        cfg.S3Endpoint,
		AccessKeyID:     cfg.S3AccessKeyID,
		SecretAccessKey: cfg.S3SecretAccessKey,
		UseSSL:          cfg.S3UseSSL,
		Region:          cfg.S3Region,
		Bucket:          bucket,
	}
	return NewS3Storage(s3Config)
}
```

- [ ] **Step 4: Run factory tests**

Run:

```bash
cd /Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static
go test ./internal/storage -run 'TestNewStorage|TestNewS3StorageForBucket' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```bash
cd /Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static
git add internal/storage/factory.go internal/storage/factory_test.go
git commit -m "feat: support alternate s3 bucket storage"
```

## Task 4: Serve Trusted Optimized Copies

**Files:**
- Modify: `/Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static/internal/handler/handler.go`
- Modify: `/Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static/internal/handler/handler_test.go`

- [ ] **Step 1: Add handler tests for optimized hit**

Add a test that sets:
- source `photo.jpg` bytes to `original`.
- source `photo.jpg` ETag to `source-etag`.
- optimized `photo.jpg` bytes to `optimized`.
- optimized metadata `source-etag=source-etag`.
- optimized metadata `optimization-profile=v1-jpeg82-png-best-w1920`.

Expected response for `GET /photo.jpg`:
- status 200.
- body `optimized`.
- header `X-S3-Static-Optimized: hit`.

- [ ] **Step 2: Add handler tests for fallback cases**

Add tests for:
- missing optimized object returns original and `X-S3-Static-Optimized: miss`.
- optimized object with stale `source-etag` returns original and `X-S3-Static-Optimized: stale`.
- optimized object with mismatched profile returns original and `X-S3-Static-Optimized: profile-mismatch`.
- request with `Range` returns original path and does not use optimized storage.
- `HEAD /photo.jpg` returns source object metadata.
- `GET /photo.jpg?meta=1` returns source metadata JSON.
- `GET /document.pdf` does not use optimized storage.

- [ ] **Step 3: Run tests and verify failure**

Run:

```bash
cd /Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static
go test ./internal/handler -run TestFileHandler_Optimized -count=1
```

Expected: FAIL because the handler has no optimized storage path.

- [ ] **Step 4: Extend handler type and constructors**

Add to `FileHandler`:

```go
optimizedStorage interfaces.Storage
```

Keep existing constructor and add:

```go
func NewFileHandlerWithOptimizedStorage(storage interfaces.Storage, optimized interfaces.Storage, cfg *config.Config, logger *config.Logger) *FileHandler {
	return &FileHandler{
		storage:          storage,
		optimizedStorage: optimized,
		config:           cfg,
		logger:           logger,
	}
}
```

Change `NewFileHandler` to call the new constructor with `nil`.

- [ ] **Step 5: Add selection helpers**

Add constants:

```go
const (
	optimizedSourceETagMetadata = "source-etag"
	optimizedProfileMetadata    = "optimization-profile"
	optimizedStatusHeader       = "X-S3-Static-Optimized"
)
```

Add:

```go
func (h *FileHandler) shouldTryOptimized(r *http.Request, info *interfaces.FileInfo) bool {
	if !h.config.OptimizedImageEnabled || h.optimizedStorage == nil {
		return false
	}
	if r.Method != http.MethodGet || shouldServeMetadata(r) || r.Header.Get("Range") != "" {
		return false
	}
	if info.Size < h.config.OptimizedMinBytes {
		return false
	}
	return isOptimizableImage(info.Path, info.ContentType)
}

func isOptimizableImage(path, contentType string) bool {
	mediaType := contentMediaType(contentType)
	ext := strings.ToLower(fileExtension(path))
	return mediaType == "image/jpeg" || mediaType == "image/png" || ext == ".jpg" || ext == ".jpeg" || ext == ".png"
}
```

Use existing `contentMediaType` and `fileExtension` helpers from `metadata.go`.

- [ ] **Step 6: Add trusted optimized open**

Add:

```go
func (h *FileHandler) openTrustedOptimized(ctx context.Context, source *interfaces.FileInfo) (*interfaces.OpenedFile, string) {
	optimized, err := h.openFileFromStorage(ctx, h.optimizedStorage, source.Path)
	if err != nil {
		return nil, "miss"
	}
	if optimized.Info.Metadata[optimizedSourceETagMetadata] != source.ETag {
		_ = optimized.Reader.Close()
		return nil, "stale"
	}
	if optimized.Info.Metadata[optimizedProfileMetadata] != h.config.OptimizationProfile {
		_ = optimized.Reader.Close()
		return nil, "profile-mismatch"
	}
	return optimized, "hit"
}
```

Refactor current `openFile` implementation into:

```go
func (h *FileHandler) openFileFromStorage(ctx context.Context, backend interfaces.Storage, path string) (*interfaces.OpenedFile, error)
```

so it can be reused for source and optimized storage.

- [ ] **Step 7: Use optimized copy in `handleGetObject`**

After the source `fileInfo` is known and before serving content:
- Try optimized only when `shouldTryOptimized` is true.
- If hit, serve optimized opened file with optimized metadata.
- If not hit, set `X-S3-Static-Optimized` to miss/stale/profile-mismatch and continue source path.

Do not run source conditional request checks against optimized ETag. Source conditional behavior should remain source-object based to preserve existing public URL semantics.

- [ ] **Step 8: Run handler tests**

Run:

```bash
cd /Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static
go test ./internal/handler -count=1
```

Expected: PASS.

- [ ] **Step 9: Commit**

Run:

```bash
cd /Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static
git add internal/handler/handler.go internal/handler/handler_test.go
git commit -m "feat: serve trusted optimized image copies"
```

## Task 5: Wire Optimized Storage In Main

**Files:**
- Modify: `/Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static/cmd/s3-static/main.go`

- [ ] **Step 1: Build failing check**

Run:

```bash
cd /Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static
go build ./cmd/s3-static
```

Expected before implementation: PASS. This step establishes a baseline.

- [ ] **Step 2: Wire optional optimized storage**

In `main.go`, import `s3-static/pkg/interfaces` if needed and add:

```go
var optimizedStorage interfaces.Storage
if cfg.OptimizedImageEnabled {
	optimizedStorage, err = storage.NewS3StorageForBucket(cfg, cfg.OptimizedBucketName)
	if err != nil {
		logger.Error("Optimized image storage disabled", map[string]interface{}{
			"bucket": cfg.OptimizedBucketName,
			"error":  err.Error(),
		})
		optimizedStorage = nil
	}
}

fileHandler := handler.NewFileHandlerWithOptimizedStorage(storageInstance, optimizedStorage, cfg, logger)
```

Do not fail startup when optimized storage cannot initialize. Source serving must remain available.

- [ ] **Step 3: Build**

Run:

```bash
cd /Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static
go build ./cmd/s3-static
```

Expected: PASS.

- [ ] **Step 4: Commit**

Run:

```bash
cd /Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static
git add cmd/s3-static/main.go
git commit -m "feat: wire optimized bucket serving"
```

## Task 6: Document External Optimizer Contract

**Files:**
- Modify: `/Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static/README.md`

- [ ] **Step 1: Add README section**

Add:

```markdown
## Optimized Image Bucket

`s3-static` can optionally serve optimized image copies from a second S3-compatible bucket without changing public URLs. The optimized bucket must use the same object keys as the source bucket.

For a source object:

```text
bucket: logseq-assets
key: notes/photo.jpg
etag: abc123
```

the external optimizer should write:

```text
bucket: logseq-assets-optimized
key: notes/photo.jpg
x-amz-meta-source-etag: abc123
x-amz-meta-optimization-profile: v1-jpeg82-png-best-w1920
```

`s3-static` serves the optimized object only when both metadata values match the current source object and configured profile. Otherwise it falls back to the source object.
```

- [ ] **Step 2: Add env docs**

Add:

```markdown
- `OPTIMIZED_IMAGE_ENABLED` - Enable trusted optimized-bucket lookup. Default: `false`.
- `OPTIMIZED_BUCKET_NAME` - Bucket containing optimized copies.
- `OPTIMIZATION_PROFILE` - Required profile metadata value. Default: `v1-jpeg82-png-best-w1920`.
- `OPTIMIZED_MIN_BYTES` - Minimum source size before optimized lookup. Default: `524288`.
```

- [ ] **Step 3: Run docs-safe checks**

Run:

```bash
cd /Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static
go test ./internal/... ./pkg/... ./cmd/...
git diff --check
```

Expected: PASS.

- [ ] **Step 4: Commit**

Run:

```bash
cd /Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static
git add README.md
git commit -m "docs: document optimized bucket contract"
```

## Final Verification

- [ ] **Step 1: Run full local gate**

Run:

```bash
cd /Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static
go fmt ./...
go test ./internal/... ./pkg/... ./cmd/...
go vet ./...
git diff --check
```

Expected: all commands pass.

- [ ] **Step 2: Confirm no worker code exists**

Run:

```bash
cd /Users/zhaochunqi/ghq/github.com/xiaolutech/s3-static
test ! -d cmd/s3-static-optimizer
test ! -d internal/optimizer
```

Expected: both commands pass.

## Self-Review Notes

- Spec coverage: This plan preserves original URLs and implements only optimized-bucket fallback in `s3-static`.
- Placeholder scan: No worker, encoding, or bucket-scanning behavior is assigned to this repo.
- Type consistency: `FileInfo.Metadata`, optimized config names, and metadata keys are introduced before use.
