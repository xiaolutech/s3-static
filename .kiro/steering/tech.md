# Technology Stack

## Language & Runtime
- **Go 1.25.0**: Primary language with modern Go features
- **Standard Library**: Extensive use of net/http, context, time packages

## Key Dependencies
- **minio-go/v7**: S3-compatible client library for storage operations
- **testcontainers-go**: Integration testing with real services

## Build System
- **Just**: Task runner for development workflows (justfile)
- **Go Modules**: Dependency management
- **Docker**: Containerization and deployment

## Common Commands

### Development
```bash
# Build application
just build

# Run in development mode
just run-dev

# Format code
just fmt

# Run linter
just lint
```

### Testing
```bash
# Run all tests
just test

# Run unit tests only
just test-unit

# Run integration tests
just test-integration

# Generate coverage report
just test-coverage

# Run benchmarks
just test-benchmark
```

### Docker
```bash
# Build Docker image
just docker-build

# Run Docker container
just docker-run

# Run Docker tests
just test-docker
```

### Quality Assurance
```bash
# Full validation (CI pipeline)
just validate

# Security scan
just security

# Check dependencies
just deps-check
```

## Environment Configuration
All configuration via environment variables:
- Server: `PORT`, `HOST`, `LOG_LEVEL`
- S3: `S3_ENDPOINT`, `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY`, `BUCKET_NAME`
- Cache: `CACHE_DURATION`