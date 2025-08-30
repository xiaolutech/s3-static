package storage

import (
	"errors"
	"net/http"
	"testing"

	"github.com/minio/minio-go/v7"
)

func TestErrorType_String(t *testing.T) {
	tests := []struct {
		errorType ErrorType
		expected  string
	}{
		{ErrorNotFound, "NotFound"},
		{ErrorForbidden, "Forbidden"},
		{ErrorInternalServer, "InternalServer"},
		{ErrorBadRequest, "BadRequest"},
		{ErrorType(999), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.errorType.String(); got != tt.expected {
				t.Errorf("ErrorType.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestErrorType_ToHTTPStatus(t *testing.T) {
	tests := []struct {
		errorType ErrorType
		expected  int
	}{
		{ErrorNotFound, http.StatusNotFound},
		{ErrorForbidden, http.StatusForbidden},
		{ErrorInternalServer, http.StatusInternalServerError},
		{ErrorBadRequest, http.StatusBadRequest},
		{ErrorType(999), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.errorType.String(), func(t *testing.T) {
			if got := tt.errorType.ToHTTPStatus(); got != tt.expected {
				t.Errorf("ErrorType.ToHTTPStatus() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestStorageError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *StorageError
		expected string
	}{
		{
			name: "error with path",
			err: &StorageError{
				Type:    ErrorNotFound,
				Message: "Object not found",
				Path:    "/test/file.txt",
			},
			expected: "NotFound: Object not found (path: /test/file.txt)",
		},
		{
			name: "error without path",
			err: &StorageError{
				Type:    ErrorInternalServer,
				Message: "Internal error",
			},
			expected: "InternalServer: Internal error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("StorageError.Error() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMapMinIOError(t *testing.T) {
	tests := []struct {
		name         string
		inputErr     error
		path         string
		expectedType ErrorType
	}{
		{
			name: "NoSuchKey error",
			inputErr: minio.ErrorResponse{
				Code: "NoSuchKey",
			},
			path:         "/test/file.txt",
			expectedType: ErrorNotFound,
		},
		{
			name: "NoSuchBucket error",
			inputErr: minio.ErrorResponse{
				Code: "NoSuchBucket",
			},
			path:         "/test/file.txt",
			expectedType: ErrorNotFound,
		},
		{
			name: "AccessDenied error",
			inputErr: minio.ErrorResponse{
				Code: "AccessDenied",
			},
			path:         "/test/file.txt",
			expectedType: ErrorForbidden,
		},
		{
			name: "InvalidRequest error",
			inputErr: minio.ErrorResponse{
				Code: "InvalidRequest",
			},
			path:         "/test/file.txt",
			expectedType: ErrorBadRequest,
		},
		{
			name: "Unknown MinIO error",
			inputErr: minio.ErrorResponse{
				Code: "UnknownError",
			},
			path:         "/test/file.txt",
			expectedType: ErrorInternalServer,
		},
		{
			name:         "Generic error",
			inputErr:     errors.New("network error"),
			path:         "/test/file.txt",
			expectedType: ErrorInternalServer,
		},
		{
			name:         "Nil error",
			inputErr:     nil,
			path:         "/test/file.txt",
			expectedType: ErrorType(-1), // Should return nil
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := MapMinIOError(tt.inputErr, tt.path)

			if tt.inputErr == nil {
				if err != nil {
					t.Errorf("MapMinIOError() should return nil for nil input, got %v", err)
				}
				return
			}

			var storageErr *StorageError
			if !errors.As(err, &storageErr) {
				t.Errorf("MapMinIOError() should return StorageError, got %T", err)
				return
			}

			if storageErr.Type != tt.expectedType {
				t.Errorf("MapMinIOError() error type = %v, want %v", storageErr.Type, tt.expectedType)
			}

			if storageErr.Path != tt.path {
				t.Errorf("MapMinIOError() error path = %v, want %v", storageErr.Path, tt.path)
			}
		})
	}
}

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name: "NotFound error",
			err: &StorageError{
				Type: ErrorNotFound,
			},
			expected: true,
		},
		{
			name: "Forbidden error",
			err: &StorageError{
				Type: ErrorForbidden,
			},
			expected: false,
		},
		{
			name:     "Generic error",
			err:      errors.New("generic error"),
			expected: false,
		},
		{
			name:     "Nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNotFound(tt.err); got != tt.expected {
				t.Errorf("IsNotFound() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIsForbidden(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name: "Forbidden error",
			err: &StorageError{
				Type: ErrorForbidden,
			},
			expected: true,
		},
		{
			name: "NotFound error",
			err: &StorageError{
				Type: ErrorNotFound,
			},
			expected: false,
		},
		{
			name:     "Generic error",
			err:      errors.New("generic error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsForbidden(tt.err); got != tt.expected {
				t.Errorf("IsForbidden() = %v, want %v", got, tt.expected)
			}
		})
	}
}
