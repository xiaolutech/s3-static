package storage

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/minio/minio-go/v7"
)

// StorageError represents different types of storage errors
type StorageError struct {
	Type    ErrorType
	Message string
	Path    string
	Err     error
}

// ErrorType defines the type of storage error
type ErrorType int

const (
	ErrorNotFound ErrorType = iota
	ErrorForbidden
	ErrorInternalServer
	ErrorBadRequest
)

// Error implements the error interface
func (e *StorageError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("%s: %s (path: %s)", e.Type.String(), e.Message, e.Path)
	}
	return fmt.Sprintf("%s: %s", e.Type.String(), e.Message)
}

// Unwrap returns the underlying error
func (e *StorageError) Unwrap() error {
	return e.Err
}

// String returns a string representation of the error type
func (et ErrorType) String() string {
	switch et {
	case ErrorNotFound:
		return "NotFound"
	case ErrorForbidden:
		return "Forbidden"
	case ErrorInternalServer:
		return "InternalServer"
	case ErrorBadRequest:
		return "BadRequest"
	default:
		return "Unknown"
	}
}

// ToHTTPStatus converts storage error type to HTTP status code
func (et ErrorType) ToHTTPStatus() int {
	switch et {
	case ErrorNotFound:
		return http.StatusNotFound
	case ErrorForbidden:
		return http.StatusForbidden
	case ErrorInternalServer:
		return http.StatusInternalServerError
	case ErrorBadRequest:
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// MapMinIOError converts MinIO errors to storage errors
func MapMinIOError(err error, path string) error {
	if err == nil {
		return nil
	}

	// Check if it's a MinIO error response
	var minioErr minio.ErrorResponse
	if errors.As(err, &minioErr) {
		switch minioErr.Code {
		case "NoSuchKey", "NoSuchBucket":
			return &StorageError{
				Type:    ErrorNotFound,
				Message: "Object not found",
				Path:    path,
				Err:     err,
			}
		case "AccessDenied":
			return &StorageError{
				Type:    ErrorForbidden,
				Message: "Access denied",
				Path:    path,
				Err:     err,
			}
		case "InvalidRequest", "InvalidArgument":
			return &StorageError{
				Type:    ErrorBadRequest,
				Message: "Invalid request",
				Path:    path,
				Err:     err,
			}
		default:
			return &StorageError{
				Type:    ErrorInternalServer,
				Message: fmt.Sprintf("S3 error: %s", minioErr.Code),
				Path:    path,
				Err:     err,
			}
		}
	}

	// For other types of errors (network, etc.)
	return &StorageError{
		Type:    ErrorInternalServer,
		Message: "Storage operation failed",
		Path:    path,
		Err:     err,
	}
}

// IsNotFound checks if the error is a not found error
func IsNotFound(err error) bool {
	var storageErr *StorageError
	if errors.As(err, &storageErr) {
		return storageErr.Type == ErrorNotFound
	}
	return false
}

// IsForbidden checks if the error is a forbidden error
func IsForbidden(err error) bool {
	var storageErr *StorageError
	if errors.As(err, &storageErr) {
		return storageErr.Type == ErrorForbidden
	}
	return false
}

// IsInternalServer checks if the error is an internal server error
func IsInternalServer(err error) bool {
	var storageErr *StorageError
	if errors.As(err, &storageErr) {
		return storageErr.Type == ErrorInternalServer
	}
	return false
}
