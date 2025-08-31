# Project Structure

## Directory Organization

```
├── cmd/s3-static/          # Application entry point
├── internal/               # Private application code
│   ├── config/            # Configuration management
│   ├── handler/           # HTTP handlers and routing
│   ├── storage/           # Storage layer implementations
│   └── testutils/         # Test utilities and mocks
├── pkg/interfaces/        # Public interfaces and contracts
├── examples/              # Usage examples and demos
├── docs/                  # Documentation
└── scripts/               # Build and utility scripts
```

## Architecture Patterns

### Clean Architecture
- **Separation of Concerns**: Clear boundaries between layers
- **Dependency Inversion**: Interfaces in `pkg/interfaces/`, implementations in `internal/`
- **Testability**: Mock implementations for all external dependencies

### Package Conventions
- `internal/`: Private application code, not importable by external packages
- `pkg/`: Public interfaces and types that could be imported
- `cmd/`: Application entry points and main functions
- `examples/`: Standalone examples with their own main functions

### File Naming
- `*_test.go`: Unit tests alongside source files
- `*_benchmark_test.go`: Benchmark tests
- `*_comprehensive_test.go`: Integration/comprehensive tests
- `main.go`: Entry point in cmd directories

### Interface Design
- Storage abstraction through `interfaces.Storage`
- Configuration through structured `config.Config`
- Logging through structured `config.Logger`

### Error Handling
- Custom error types in `internal/storage/errors.go`
- Error mapping for external service errors
- Structured error responses with S3-compatible XML format

### Testing Strategy
- Unit tests with mocks for each package
- Integration tests using testcontainers
- Benchmark tests for performance validation
- Examples with runnable tests