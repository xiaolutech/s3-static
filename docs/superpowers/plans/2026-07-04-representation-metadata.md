# Representation Metadata Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `GET /{path}?meta=1` and `HEAD /{path}` describe the same negotiated representation that `GET /{path}` would return, while keeping an explicit source/original metadata escape hatch.

**Architecture:** Keep `s3-static` as a thin public gateway. Reuse the existing optimized variant resolver for `GET`, `HEAD`, and metadata requests, but add a source-forcing query contract so callers can ask for original-object metadata without changing the public object path. Metadata responses gain representation fields that identify whether the described object is source or optimized.

**Tech Stack:** Go `net/http`, existing `interfaces.Storage` / `interfaces.FileOpener`, existing `http.ServeContent`, existing `image.DecodeConfig` media metadata parsing, Go unit tests under `internal/handler`.

---

## File Structure

- Modify `internal/handler/metadata.go`
  - Extend `fileMetadataResponse` with `Optimized *bool` and `VariantFormat string`.
  - Keep media parsing helpers unchanged.
- Modify `internal/handler/optimized_variant.go`
  - Let `NeedsSourceInfo` consider `HEAD` and representation metadata requests.
  - Keep `Range` and explicit source metadata requests on the source-only path.
  - Add a query helper such as `shouldForceSourceVariant(req)`.
- Modify `internal/handler/handler.go`
  - Route `GET ?meta=1` through the same source-info and optimized-resolution flow as object responses.
  - Use the resolved `OpenedFile` to build metadata JSON.
  - Let `HEAD` resolve optimized variants, then serve headers without a body through existing `serveOpenedFile`.
- Modify `internal/handler/handler_test.go`
  - Replace the source-only metadata optimized test with representation-aware tests.
  - Add explicit source metadata tests.
  - Add `HEAD` optimized representation tests.
- Modify `README.md`
  - Document default representation metadata behavior.
  - Document explicit source metadata with `GET /{path}?meta=1&variant=source`.
  - Document `HEAD` content negotiation behavior.

## Contract

- `GET /photo.jpg` with `Accept: image/webp` may return optimized `photo.jpg.webp` when trusted.
- `GET /photo.jpg?meta=1` with `Accept: image/webp` should return JSON describing the optimized WebP representation when it would be served.
- `GET /photo.jpg?meta=1&variant=source` should return JSON describing the source object even if the request accepts WebP or AVIF.
- `HEAD /photo.jpg` with `Accept: image/webp` should return headers for the optimized WebP representation when it would be served.
- Conditional request semantics remain source-ETag based for optimized object access, matching the existing public URL contract.
- `Range` requests remain source-only.

### Task 1: Metadata JSON Describes Negotiated Representation

**Files:**
- Modify: `internal/handler/metadata.go`
- Modify: `internal/handler/optimized_variant.go`
- Modify: `internal/handler/handler.go`
- Test: `internal/handler/handler_test.go`

- [ ] **Step 1: Write the failing representation metadata test**

Add this test near `TestFileHandler_OptimizedImageSkipsHeadAndMetadata` in `internal/handler/handler_test.go`:

```go
func TestFileHandler_MetadataDescribesOptimizedRepresentation(t *testing.T) {
	cfg := optimizedTestConfig()
	logger := config.NewLogger("info")
	source := newMockStorage()
	optimizedBase := newMockStorage()
	optimized := &openFileMockStorage{mockStorage: optimizedBase}
	handler := NewFileHandlerWithOptimizedStorage(source, optimized, cfg, logger)

	modTime := time.Now().UTC().Truncate(time.Second)
	source.addFileWithMetadata("photo.jpg", []byte("original image"), modTime, "source-etag", "image/jpeg", nil)
	addTrustedWebPFile(optimizedBase, source.files["photo.jpg"], []byte("webp image"), modTime.Add(time.Minute), "optimized-etag", cfg.OptimizationProfile)

	req := httptest.NewRequest(http.MethodGet, "/photo.jpg?meta=1", nil)
	req.Header.Set("Accept", "image/webp")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}
	if got := w.Header().Get(optimizedStatusHeader); got != "hit; format=webp" {
		t.Fatalf("Expected optimized hit header, got %q", got)
	}

	var response fileMetadataResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Path != "photo.jpg" {
		t.Fatalf("Expected source URL path in metadata, got %q", response.Path)
	}
	if response.ContentType != "image/webp" {
		t.Fatalf("Expected optimized content type image/webp, got %q", response.ContentType)
	}
	if response.Size != int64(len("webp image")) {
		t.Fatalf("Expected optimized size %d, got %d", len("webp image"), response.Size)
	}
	if response.ETag != "optimized-etag" {
		t.Fatalf("Expected optimized ETag, got %q", response.ETag)
	}
	if response.Optimized == nil || !*response.Optimized {
		t.Fatalf("Expected optimized=true, got %#v", response.Optimized)
	}
	if response.VariantFormat != "webp" {
		t.Fatalf("Expected variant format webp, got %q", response.VariantFormat)
	}
	if optimized.openCalls != 1 {
		t.Fatalf("Expected optimized storage to be opened once, got %d", optimized.openCalls)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/handler -run TestFileHandler_MetadataDescribesOptimizedRepresentation -count=1
```

Expected: FAIL. Accept either a compile failure because `fileMetadataResponse.Optimized` or `VariantFormat` does not exist, or a behavior failure showing source `ETag` / `Content-Type` instead of optimized representation metadata.

- [ ] **Step 3: Extend metadata response type**

In `internal/handler/metadata.go`, change `fileMetadataResponse` to:

```go
type fileMetadataResponse struct {
	Path          string    `json:"path"`
	ContentType   string    `json:"contentType,omitempty"`
	Size          int64     `json:"size"`
	ETag          string    `json:"etag,omitempty"`
	LastModified  time.Time `json:"lastModified"`
	Width         *int      `json:"width"`
	Height        *int      `json:"height"`
	Optimized     *bool     `json:"optimized,omitempty"`
	VariantFormat string    `json:"variantFormat,omitempty"`
}
```

- [ ] **Step 4: Add source-forcing and metadata-aware optimized lookup helpers**

In `internal/handler/optimized_variant.go`, add this helper after `NewOptimizedVariantResolver`:

```go
func shouldForceSourceVariant(req *http.Request) bool {
	if req == nil {
		return false
	}
	variant := strings.ToLower(strings.TrimSpace(req.URL.Query().Get("variant")))
	return variant == "source" || variant == "original"
}
```

Then change `NeedsSourceInfo` to:

```go
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
```

- [ ] **Step 5: Route metadata through optimized resolution**

In `internal/handler/handler.go`, replace `handleGetMetadata` with:

```go
func (h *FileHandler) handleGetMetadata(w http.ResponseWriter, r *http.Request) {
	path, ok := validateRequestPath(r.URL.Path)
	if !ok {
		if path == "" {
			writeJSONError(w, http.StatusBadRequest, "Empty path")
			return
		}
		writeJSONError(w, http.StatusBadRequest, "Invalid path")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), storageRequestTimeout)
	defer cancel()

	sourceInfo, err := h.getFileInfo(ctx, path)
	if err != nil {
		h.handleMetadataStorageError(w, err, path)
		return
	}

	openedFile, optimizedStatus, err := h.openMetadataRepresentation(ctx, r, sourceInfo)
	if err != nil {
		h.handleMetadataStorageError(w, err, path)
		return
	}
	defer func() {
		if openedFile.Reader != nil {
			openedFile.Reader.Close()
		}
	}()

	if header := optimizedStatus.HeaderValue(); header != "" {
		w.Header().Set(optimizedStatusHeader, header)
	}

	metadata := h.buildMetadataResponse(path, openedFile, optimizedStatus)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Type, "+optimizedStatusHeader)
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(metadata); err != nil {
		h.logger.Error("Failed to write metadata response", "path", path, "error", err)
	}
}
```

Add these helpers below `handleGetMetadata` in `internal/handler/handler.go`:

```go
func (h *FileHandler) openMetadataRepresentation(ctx context.Context, r *http.Request, sourceInfo *interfaces.FileInfo) (*interfaces.OpenedFile, VariantStatus, error) {
	if sourceInfo != nil && h.optimizedResolver != nil {
		optimizedFile, status := h.optimizedResolver.Resolve(ctx, r, sourceInfo)
		if optimizedFile != nil {
			return optimizedFile, status, nil
		}
	}

	openedFile, err := h.openFile(ctx, sourceInfo.Path)
	return openedFile, VariantStatus{}, err
}

func (h *FileHandler) buildMetadataResponse(path string, openedFile *interfaces.OpenedFile, status VariantStatus) fileMetadataResponse {
	fileInfo := openedFile.Info
	optimized := status.Code == optimizedStatusHit
	metadata := fileMetadataResponse{
		Path:          path,
		ContentType:   fileInfo.ContentType,
		Size:          fileInfo.Size,
		ETag:          fileInfo.ETag,
		LastModified:  fileInfo.ModTime.UTC(),
		Optimized:     &optimized,
		VariantFormat: status.Format,
	}

	if shouldProbeMediaDimensions(path, fileInfo.ContentType) {
		width, height, format, err := extractMediaMetadata(path, fileInfo.ContentType, openedFile.Reader)
		if seekErr := seekReaderStart(openedFile.Reader); seekErr != nil {
			h.logger.Debug("Media metadata probe could not rewind reader",
				"path", path,
				"error", seekErr,
			)
		}
		if err != nil {
			h.logger.Debug("Media metadata probe skipped",
				"path", path,
				"error", err,
			)
		} else {
			metadata.Width = &width
			metadata.Height = &height
			if metadata.ContentType == "" {
				metadata.ContentType = detectContentTypeFromFormat(format)
			}
		}
	}

	return metadata
}
```

- [ ] **Step 6: Run test to verify it passes**

Run:

```bash
go test ./internal/handler -run TestFileHandler_MetadataDescribesOptimizedRepresentation -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit Task 1**

Run:

```bash
git add internal/handler/metadata.go internal/handler/optimized_variant.go internal/handler/handler.go internal/handler/handler_test.go
git commit -m "fix: make metadata describe optimized representation"
```

### Task 2: Explicit Source Metadata Escape Hatch

**Files:**
- Modify: `internal/handler/handler_test.go`
- Modify: `internal/handler/handler.go`
- Modify: `internal/handler/optimized_variant.go`

- [ ] **Step 1: Write the failing source metadata test**

Add this test near `TestFileHandler_MetadataDescribesOptimizedRepresentation`:

```go
func TestFileHandler_MetadataCanForceSourceRepresentation(t *testing.T) {
	cfg := optimizedTestConfig()
	logger := config.NewLogger("info")
	source := newMockStorage()
	optimizedBase := newMockStorage()
	optimized := &openFileMockStorage{mockStorage: optimizedBase}
	handler := NewFileHandlerWithOptimizedStorage(source, optimized, cfg, logger)

	modTime := time.Now().UTC().Truncate(time.Second)
	source.addFileWithMetadata("photo.jpg", []byte("original image"), modTime, "source-etag", "image/jpeg", nil)
	addTrustedWebPFile(optimizedBase, source.files["photo.jpg"], []byte("webp image"), modTime.Add(time.Minute), "optimized-etag", cfg.OptimizationProfile)

	req := httptest.NewRequest(http.MethodGet, "/photo.jpg?meta=1&variant=source", nil)
	req.Header.Set("Accept", "image/webp")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}
	if got := w.Header().Get(optimizedStatusHeader); got != "" {
		t.Fatalf("Expected no optimized status header, got %q", got)
	}

	var response fileMetadataResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ContentType != "image/jpeg" {
		t.Fatalf("Expected source content type image/jpeg, got %q", response.ContentType)
	}
	if response.Size != int64(len("original image")) {
		t.Fatalf("Expected source size %d, got %d", len("original image"), response.Size)
	}
	if response.ETag != "source-etag" {
		t.Fatalf("Expected source ETag, got %q", response.ETag)
	}
	if response.Optimized == nil || *response.Optimized {
		t.Fatalf("Expected optimized=false, got %#v", response.Optimized)
	}
	if response.VariantFormat != "" {
		t.Fatalf("Expected empty variant format for source metadata, got %q", response.VariantFormat)
	}
	if optimized.openCalls != 0 {
		t.Fatalf("Expected optimized storage not to be opened, got %d", optimized.openCalls)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/handler -run TestFileHandler_MetadataCanForceSourceRepresentation -count=1
```

Expected before Task 1: compile failure for new response fields. Expected after Task 1: PASS if `shouldForceSourceVariant` is already implemented correctly. If it fails, fix the helper so `variant=source` and `variant=original` skip optimized lookup.

- [ ] **Step 3: Add original alias test**

Add this table-style check after the source test or convert the source test to a table:

```go
func TestFileHandler_MetadataCanForceOriginalAlias(t *testing.T) {
	cfg := optimizedTestConfig()
	logger := config.NewLogger("info")
	source := newMockStorage()
	optimizedBase := newMockStorage()
	optimized := &openFileMockStorage{mockStorage: optimizedBase}
	handler := NewFileHandlerWithOptimizedStorage(source, optimized, cfg, logger)

	modTime := time.Now().UTC().Truncate(time.Second)
	source.addFileWithMetadata("photo.jpg", []byte("original image"), modTime, "source-etag", "image/jpeg", nil)
	addTrustedWebPFile(optimizedBase, source.files["photo.jpg"], []byte("webp image"), modTime.Add(time.Minute), "optimized-etag", cfg.OptimizationProfile)

	req := httptest.NewRequest(http.MethodGet, "/photo.jpg?meta=1&variant=original", nil)
	req.Header.Set("Accept", "image/webp")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	var response fileMetadataResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ETag != "source-etag" {
		t.Fatalf("Expected original alias to return source ETag, got %q", response.ETag)
	}
	if response.Optimized == nil || *response.Optimized {
		t.Fatalf("Expected optimized=false, got %#v", response.Optimized)
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run:

```bash
go test ./internal/handler -run 'TestFileHandler_MetadataCanForce(SourceRepresentation|OriginalAlias)' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit Task 2**

Run:

```bash
git add internal/handler/handler_test.go internal/handler/handler.go internal/handler/optimized_variant.go
git commit -m "feat: add source metadata variant"
```

### Task 3: HEAD Describes Negotiated Representation

**Files:**
- Modify: `internal/handler/handler_test.go`
- Modify: `internal/handler/handler.go`
- Modify: `internal/handler/optimized_variant.go`

- [ ] **Step 1: Write failing optimized HEAD test**

Replace the `head` case inside `TestFileHandler_OptimizedImageSkipsHeadAndMetadata` or add this standalone test:

```go
func TestFileHandler_HeadDescribesOptimizedRepresentation(t *testing.T) {
	cfg := optimizedTestConfig()
	logger := config.NewLogger("info")
	source := newMockStorage()
	optimizedBase := newMockStorage()
	optimized := &openFileMockStorage{mockStorage: optimizedBase}
	handler := NewFileHandlerWithOptimizedStorage(source, optimized, cfg, logger)

	modTime := time.Now().UTC().Truncate(time.Second)
	source.addFileWithMetadata("photo.jpg", []byte("original image"), modTime, "source-etag", "image/jpeg", nil)
	addTrustedWebPFile(optimizedBase, source.files["photo.jpg"], []byte("webp image"), modTime.Add(time.Minute), "optimized-etag", cfg.OptimizationProfile)

	req := httptest.NewRequest(http.MethodHead, "/photo.jpg", nil)
	req.Header.Set("Accept", "image/webp")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Fatalf("Expected empty HEAD body, got %q", w.Body.String())
	}
	if got := w.Header().Get(optimizedStatusHeader); got != "hit; format=webp" {
		t.Fatalf("Expected optimized hit header, got %q", got)
	}
	if got := w.Header().Get("Content-Type"); got != "image/webp" {
		t.Fatalf("Expected optimized content type image/webp, got %q", got)
	}
	if got := w.Header().Get("ETag"); got != `"optimized-etag"` {
		t.Fatalf("Expected optimized ETag, got %s", got)
	}
	if got := w.Header().Get("Vary"); got != "Accept" {
		t.Fatalf("Expected Vary: Accept, got %q", got)
	}
	if optimized.openCalls != 1 {
		t.Fatalf("Expected optimized storage to be opened once, got %d", optimized.openCalls)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/handler -run TestFileHandler_HeadDescribesOptimizedRepresentation -count=1
```

Expected: FAIL because current optimized resolution only allows `GET` and the existing test expects source headers for `HEAD`.

- [ ] **Step 3: Ensure handler resolves optimized for HEAD**

If Task 1 already changed `NeedsSourceInfo` to allow `HEAD`, verify `handleGetObject` requires no additional branching. It already calls `serveOpenedFile`, and `http.ServeContent` handles `HEAD` with no body.

If the test still fails because `sourceInfo` is not loaded, change this block in `internal/handler/handler.go`:

```go
if h.hasConditionalHeaders(r) || h.optimizedResolver.NeedsSourceInfo(r) {
```

to keep the same expression but ensure `h.optimizedResolver` cannot be nil:

```go
if h.hasConditionalHeaders(r) || (h.optimizedResolver != nil && h.optimizedResolver.NeedsSourceInfo(r)) {
```

- [ ] **Step 4: Run optimized HEAD test**

Run:

```bash
go test ./internal/handler -run TestFileHandler_HeadDescribesOptimizedRepresentation -count=1
```

Expected: PASS.

- [ ] **Step 5: Run existing optimized handler tests**

Run:

```bash
go test ./internal/handler -run 'TestFileHandler_OptimizedImage|TestOptimizedVariantResolver' -count=1
```

Expected: PASS after updating or removing `TestFileHandler_OptimizedImageSkipsHeadAndMetadata` expectations that required source-only `HEAD` / metadata.

- [ ] **Step 6: Commit Task 3**

Run:

```bash
git add internal/handler/handler.go internal/handler/optimized_variant.go internal/handler/handler_test.go
git commit -m "fix: make head describe optimized representation"
```

### Task 4: Documentation and Full Verification

**Files:**
- Modify: `README.md`
- Test: `internal/handler/handler_test.go`

- [ ] **Step 1: Update optimized image docs**

In `README.md`, replace the paragraph around the current optimized skip list with:

```markdown
If the request does not advertise a supported optimized format, or if the optimized
object is missing, stale, profile-mismatched, unreadable, or has unexpected metadata, the source object is
served. `Range` requests, non-image files, and images smaller than `OPTIMIZED_MIN_BYTES`
continue to use the source object path directly.

`GET /{path}?meta=1` and `HEAD /{path}` follow the same `Accept` negotiation as
`GET /{path}`. When a trusted optimized variant would be served, metadata and
headers describe that optimized representation and include `X-S3-Static-Optimized`.
Use `GET /{path}?meta=1&variant=source` to inspect the original source object.
```

- [ ] **Step 2: Update File Metadata docs**

In `README.md`, change the File Metadata section so it includes:

```markdown
GET /{path}?meta=1
GET /{path}?meta=1&variant=source
```

and extend the response example:

```json
{
  "path": "images/logo.png",
  "contentType": "image/webp",
  "size": 6789,
  "etag": "optimized-etag",
  "lastModified": "2026-05-17T02:01:00Z",
  "width": 512,
  "height": 512,
  "optimized": true,
  "variantFormat": "webp"
}
```

Add these response field bullets:

```markdown
- `optimized`: `true` when the metadata describes a trusted optimized representation, `false` for the source object
- `variantFormat`: Optimized representation format such as `webp` or `avif`; omitted for source objects
```

- [ ] **Step 3: Run focused tests**

Run:

```bash
go test ./internal/handler -run 'TestFileHandler_(Metadata|Head|OptimizedImage)|TestOptimizedVariantResolver' -count=1
```

Expected: PASS.

- [ ] **Step 4: Run full test suite**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 5: Commit Task 4**

Run:

```bash
git add README.md internal/handler/handler_test.go internal/handler/handler.go internal/handler/metadata.go internal/handler/optimized_variant.go
git commit -m "docs: clarify representation metadata contract"
```

## Self-Review

- Spec coverage: The plan covers `?meta=1` purpose, default metadata fidelity to returned image, explicit source/original metadata, `HEAD` parity with `GET`, docs, focused tests, and full verification.
- Placeholder scan: No `TBD`, generic TODO, or unspecified "write tests" steps remain. Every code-changing step includes concrete code.
- Type consistency: `fileMetadataResponse.Optimized`, `fileMetadataResponse.VariantFormat`, `VariantStatus`, `HeaderValue`, `shouldForceSourceVariant`, `openMetadataRepresentation`, and `buildMetadataResponse` are named consistently across tasks.
