# Product Overview

S3-Compatible Static File Service is a high-performance Go application that serves static files with full S3 API compatibility. The service acts as a bridge between HTTP clients and S3-compatible storage backends (AWS S3, MinIO, etc.), providing efficient static file serving with enterprise-grade features.

## Core Features

- **S3 Compatibility**: Full compatibility with S3-compatible storage backends
- **High Performance**: Optimized for minimal latency static file serving
- **Caching**: ETag support and conditional requests (If-None-Match, If-Modified-Since)
- **Health Monitoring**: Built-in health check endpoint at `/health`
- **Structured Logging**: Configurable log levels with structured output
- **Docker Ready**: Production-ready containerized deployment

## Use Cases

- Static website hosting with S3 backend
- CDN origin server for S3-stored assets
- High-performance file serving proxy for S3-compatible storage
- Development/testing proxy for S3 services

## Architecture Philosophy

The service follows clean architecture principles with clear separation between HTTP handling, storage abstraction, and configuration management. All components are designed for testability and maintainability.