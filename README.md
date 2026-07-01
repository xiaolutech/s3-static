# S3-Compatible Static File Service

A high-performance, S3-compatible static file service built in Go. This service provides efficient static file serving with S3 API compatibility, ETag support, conditional requests, and comprehensive caching. The current S3 client implementation uses AWS SDK for Go v2 and works with S3-compatible backends such as AWS S3 and MinIO.

## Features

- **S3 Compatibility**: Compatible with S3-compatible storage backends (MinIO, AWS S3, etc.)
- **High Performance**: Optimized for serving static files with minimal latency
- **Smart Caching**: Multiple caching strategies (no-cache, max-age, immutable) following HTTP best practices
- **ETag Support**: Automatic ETag generation and validation for efficient caching
- **Conditional Requests**: Support for If-None-Match, If-Modified-Since headers
- **Streaming Support**: Efficient streaming for large files with HTTP Range request support (206 Partial Content)
- **Optimized Image Bucket**: Optional trusted fallback to pre-optimized JPEG/PNG copies without changing public URLs
- **Health Monitoring**: Built-in health check endpoint
- **Structured Logging**: Configurable log levels with structured output
- **Docker Support**: Ready-to-use Docker container
- **Comprehensive Testing**: Unit tests, integration tests, and benchmarks

## Quick Start

### Using Docker

```bash
# Build the Docker image
make docker-build

# Run with environment variables
docker run -p 8080:8080 \
  -e S3_ENDPOINT=localhost:9000 \
  -e S3_ACCESS_KEY_ID=minioadmin \
  -e S3_SECRET_ACCESS_KEY=minioadmin \
  -e BUCKET_NAME=static-files \
  -e S3_USE_SSL=false \
  s3-static
```

### Using Go

```bash
# Install dependencies
go mod download

# Build the application
make build

# Set environment variables and run
export S3_ENDPOINT=localhost:9000
export S3_ACCESS_KEY_ID=minioadmin
export S3_SECRET_ACCESS_KEY=minioadmin
export BUCKET_NAME=static-files
export S3_USE_SSL=false
./s3-static
```

## Configuration

The service is configured via environment variables:

### Server Configuration
- `PORT` - Server port (default: 8080)
- `HOST` - Server host (default: 0.0.0.0)
- `LOG_LEVEL` - Log level: debug, info, warn, error, fatal (default: info)

### Storage Configuration
- `S3_ENDPOINT` - S3 endpoint URL (required)
- `S3_REGION` - S3 region (default: us-east-1)
- `S3_ACCESS_KEY_ID` - S3 access key (required)
- `S3_SECRET_ACCESS_KEY` - S3 secret key (required)
- `S3_USE_SSL` - Use SSL for S3 connections (default: true)
- `BUCKET_NAME` - S3 bucket name (required)

### Cache Configuration
- `CACHE_STRATEGY` - Caching strategy: `no-cache`, `max-age`, `immutable` (default: immutable)
- `CACHE_DURATION` - Cache duration for max-age and immutable strategies (default: 8760h)

### Optimized Image Configuration
- `OPTIMIZED_IMAGE_ENABLED` - Enable trusted optimized-bucket lookup (default: false)
- `OPTIMIZED_BUCKET_NAME` - Bucket containing optimized copies
- `OPTIMIZATION_PROFILE` - Required profile metadata value (default: v1-jpeg82-png-best-w1920)
- `OPTIMIZED_MIN_BYTES` - Minimum source size before optimized lookup (default: 524288)
- `OPTIMIZER_TRIGGER_URL` - Optional internal URL that receives `POST /optimize?key=...` when an optimized image is missing, stale, or built with a different profile. Empty disables on-demand triggers.
- `OPTIMIZER_TRIGGER_TIMEOUT` - Timeout for the non-blocking optimizer trigger HTTP request. Default: `2s`.

## Caching Strategies

The service supports three caching strategies optimized for different use cases:

### immutable (Recommended, Default)
```bash
export CACHE_STRATEGY=immutable
export CACHE_DURATION=8760h  # 1 year
```
- **Best for**: Static files that don't change after creation (99.9% of uploads)
- **Behavior**: Browser never requests file again during cache period
- **Benefits**: Maximum performance, zero network requests, best user experience

### no-cache (For Variable Content)
```bash
export CACHE_STRATEGY=no-cache
```
- **Best for**: Content that may change (0.1% of files)
- **Behavior**: Browser validates cache on every request using ETag/Last-Modified
- **Benefits**: Always serves fresh content while leveraging cache efficiency

### max-age (Not Recommended)
```bash
export CACHE_STRATEGY=max-age
export CACHE_DURATION=1h
```
- **Best for**: Testing or special requirements only
- **Behavior**: Browser may serve stale content during cache period
- **Warning**: Can cause version mismatch issues with related files

For detailed caching documentation, see [docs/CACHING.md](docs/CACHING.md).

## Optimized Image Bucket

`s3-static` can optionally serve optimized image copies from a second S3-compatible
bucket without changing public URLs. The optimized bucket must use the same object
keys as the source bucket.

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

`s3-static` serves the optimized object only when both metadata values match the
current source object and configured profile. Otherwise it falls back to the
source object.

When `OPTIMIZER_TRIGGER_URL` is configured, a trusted optimized-bucket miss does
not change the public response. `s3-static` serves the source object immediately
and triggers optimization in the background. A subsequent request can serve the
optimized object after the sidecar writes it with matching `source-etag` and
`optimization-profile` metadata.

Optimized lookup is only attempted for ordinary `GET` requests for JPEG/PNG images.
`HEAD`, `GET /{path}?meta=1`, `Range` requests, non-image files, and images smaller
than `OPTIMIZED_MIN_BYTES` continue to use the source object path directly.

## API Endpoints

### File Access
```
GET /{path}
```
Serves static files from the configured S3 bucket.

**Headers:**
- `If-None-Match`: ETag-based conditional request
- `If-Modified-Since`: Time-based conditional request
- `Range`: Byte range request for streaming (e.g., `bytes=0-1023`)

**Response Headers:**
- `ETag`: File ETag for caching
- `Last-Modified`: File modification time
- `Cache-Control`: Caching directives
- `Content-Type`: MIME type based on file extension
- `Content-Length`: File size
- `Content-Range`: Range information (for 206 Partial Content responses)

### File Metadata
```
GET /{path}?meta=1
```
Returns object metadata as JSON. This is an extension endpoint for callers that need
media dimensions in addition to standard object headers.

Use `HEAD /{path}` when you only need standard HTTP metadata such as `Content-Type`,
`Content-Length`, `ETag`, or `Last-Modified`. Use `GET /{path}?meta=1` when you also
need parsed metadata such as image or video `width` and `height`.

**Behavior:**
- Reuses the same object path as file access; no separate metadata route is required
- Returns base metadata for any object
- Attempts to parse dimensions for supported image and video types
- Currently supports PNG, JPEG, GIF, WebP, BMP, TIFF, SVG `width`/`height` or `viewBox`,
  and MP4-family video containers such as MP4, M4V, and MOV

**Example response:**
```json
{
  "path": "images/logo.png",
  "contentType": "image/png",
  "size": 12345,
  "etag": "abc123",
  "lastModified": "2026-05-17T02:00:00Z",
  "width": 512,
  "height": 512
}
```

**Response Fields:**
- `path`: Object key path
- `contentType`: Stored content type if available
- `size`: Object size in bytes
- `etag`: Object ETag
- `lastModified`: Last modification time in RFC3339 format
- `width`: Parsed media width, or `null` when unavailable
- `height`: Parsed media height, or `null` when unavailable

For regular `GET /{path}` and `HEAD /{path}` object responses, parsed dimensions are
also exposed as S3-compatible user metadata headers when available:
- `x-amz-meta-width`
- `x-amz-meta-height`

**Notes:**
- `width` and `height` are best-effort parsed values, not S3-native metadata
- Non-image files return metadata with `width` and `height` as `null`
- If media metadata parsing fails, the endpoint still returns base object metadata

### Health Check
```
GET /health
```
Returns service health status and storage connectivity.

## Development

### Prerequisites
- Go 1.21 or later
- Docker (for integration tests)
- Make

### Setup Development Environment
```bash
# Install development tools
make install-tools

# Setup development environment
make setup
```

### Running Tests
```bash
# Run all tests
make test

# Run unit tests only
make test-unit

# Run integration tests (requires Docker)
make test-integration

# Run with coverage
make test-coverage

# Run benchmarks
make test-benchmark
```

### Code Quality
```bash
# Format code
make fmt

# Run linter
make lint

# Run security scan
make security

# Full validation (CI pipeline)
make validate
```

### Development Workflow
```bash
# Quick development cycle
make dev

# Run in development mode
make run-dev
```

## Architecture

The service follows a clean architecture pattern with clear separation of concerns:

```
├── cmd/s3-static/          # Application entry point
├── internal/
│   ├── config/             # Configuration management
│   ├── handler/            # HTTP handlers
│   ├── storage/            # Storage layer (S3 implementation)
│   └── testutils/          # Test utilities
├── pkg/interfaces/         # Public interfaces
└── examples/               # Usage examples
```

### Key Components

- **Storage Layer**: Abstracted storage interface with S3 implementation
- **HTTP Handlers**: File serving and health check handlers
- **Configuration**: Environment-based configuration with validation
- **Error Handling**: Structured error handling with proper HTTP status mapping

## Performance

The service is optimized for high-performance static file serving:

- **Efficient Memory Usage**: Streaming file transfers for large files
- **Range Requests**: HTTP Range support for partial content (206 Partial Content), enables video seeking and chunked downloads
- **Conditional Requests**: Reduces bandwidth with ETag and Last-Modified support
- **Connection Pooling**: Reuses S3 connections for better performance
- **Minimal Allocations**: Optimized code paths to reduce GC pressure

### Benchmarks

Run benchmarks to measure performance:

```bash
make test-benchmark
```

## Deployment

### Docker Deployment

```bash
# Build multi-architecture image
docker buildx build --platform linux/amd64,linux/arm64 -t s3-static .

# Run with docker-compose
version: '3.8'
services:
  s3-static:
    image: s3-static
    ports:
      - "8080:8080"
    environment:
      - S3_ENDPOINT=minio:9000
      - S3_ACCESS_KEY_ID=minioadmin
      - S3_SECRET_ACCESS_KEY=minioadmin
      - BUCKET_NAME=static-files
      - S3_USE_SSL=false
    depends_on:
      - minio
```

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: s3-static
spec:
  replicas: 3
  selector:
    matchLabels:
      app: s3-static
  template:
    metadata:
      labels:
        app: s3-static
    spec:
      containers:
      - name: s3-static
        image: s3-static:latest
        ports:
        - containerPort: 8080
        env:
        - name: S3_ENDPOINT
          value: "minio-service:9000"
        - name: BUCKET_NAME
          value: "static-files"
        envFrom:
        - secretRef:
            name: s3-credentials
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30
```

## Monitoring

### Health Checks

The service provides a health endpoint at `/health` that checks:
- Service availability
- S3 storage connectivity
- Configuration validity

### Logging

Structured logging with configurable levels:
- Request/response logging
- Error tracking
- Performance metrics
- Storage operation logs

### Metrics

The service logs performance metrics suitable for monitoring:
- Request duration
- File size statistics
- Cache hit/miss ratios
- Error rates

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Run the full test suite: `make validate`
6. Submit a pull request

### Code Standards

- Follow Go best practices and idioms
- Maintain test coverage above 80%
- Use structured logging
- Document public APIs
- Handle errors appropriately

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Support

For questions, issues, or contributions:
- Open an issue on GitHub
- Check the documentation in the `docs/` directory
- Review the examples in the `examples/` directory
