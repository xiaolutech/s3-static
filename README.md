# S3-Compatible Static File Service

A high-performance, S3-compatible static file service built in Go. This service provides efficient static file serving with S3 API compatibility, ETag support, conditional requests, and comprehensive caching.

## Features

- **S3 Compatibility**: Compatible with S3-compatible storage backends (MinIO, AWS S3, etc.)
- **High Performance**: Optimized for serving static files with minimal latency
- **Smart Caching**: Multiple caching strategies (no-cache, max-age, immutable) following HTTP best practices
- **ETag Support**: Automatic ETag generation and validation for efficient caching
- **Conditional Requests**: Support for If-None-Match, If-Modified-Since headers
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
- `CACHE_STRATEGY` - Caching strategy: `no-cache`, `max-age`, `immutable` (default: no-cache)
- `CACHE_DURATION` - Cache duration for max-age and immutable strategies (default: 1h)

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

## API Endpoints

### File Access
```
GET /{path}
```
Serves static files from the configured S3 bucket.

**Headers:**
- `If-None-Match`: ETag-based conditional request
- `If-Modified-Since`: Time-based conditional request

**Response Headers:**
- `ETag`: File ETag for caching
- `Last-Modified`: File modification time
- `Cache-Control`: Caching directives
- `Content-Type`: MIME type based on file extension

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