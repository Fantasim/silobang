package services

import (
	"errors"
	"fmt"
	"testing"

	"meshbank/internal/constants"
)

func TestNewServiceError(t *testing.T) {
	tests := []struct {
		name        string
		code        string
		message     string
		wantCode    string
		wantMessage string
	}{
		{
			name:        "creates error with code and message",
			code:        constants.ErrCodeTopicNotFound,
			message:     "topic not found",
			wantCode:    constants.ErrCodeTopicNotFound,
			wantMessage: "topic not found",
		},
		{
			name:        "creates error with different code",
			code:        constants.ErrCodeAssetNotFound,
			message:     "asset does not exist",
			wantCode:    constants.ErrCodeAssetNotFound,
			wantMessage: "asset does not exist",
		},
		{
			name:        "handles empty message",
			code:        constants.ErrCodeInternalError,
			message:     "",
			wantCode:    constants.ErrCodeInternalError,
			wantMessage: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewServiceError(tt.code, tt.message)

			if err.Code != tt.wantCode {
				t.Errorf("Code = %q, want %q", err.Code, tt.wantCode)
			}
			if err.Message != tt.wantMessage {
				t.Errorf("Message = %q, want %q", err.Message, tt.wantMessage)
			}
			if err.Err != nil {
				t.Error("Err should be nil for NewServiceError")
			}
		})
	}
}

func TestServiceError_Error(t *testing.T) {
	tests := []struct {
		name    string
		err     *ServiceError
		wantStr string
	}{
		{
			name:    "formats error without wrapped error",
			err:     NewServiceError(constants.ErrCodeTopicNotFound, "topic not found"),
			wantStr: "TOPIC_NOT_FOUND: topic not found",
		},
		{
			name:    "formats error with wrapped error",
			err:     WrapServiceError(constants.ErrCodeInternalError, "operation failed", errors.New("disk full")),
			wantStr: "INTERNAL_ERROR: operation failed: disk full",
		},
		{
			name:    "handles empty message without wrapped error",
			err:     NewServiceError(constants.ErrCodeInternalError, ""),
			wantStr: "INTERNAL_ERROR: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.wantStr {
				t.Errorf("Error() = %q, want %q", got, tt.wantStr)
			}
		})
	}
}

func TestServiceError_Unwrap(t *testing.T) {
	t.Run("returns wrapped error", func(t *testing.T) {
		innerErr := errors.New("inner error")
		svcErr := WrapServiceError(constants.ErrCodeInternalError, "outer", innerErr)

		unwrapped := svcErr.Unwrap()
		if unwrapped != innerErr {
			t.Errorf("Unwrap() = %v, want %v", unwrapped, innerErr)
		}
	})

	t.Run("returns nil when no wrapped error", func(t *testing.T) {
		svcErr := NewServiceError(constants.ErrCodeTopicNotFound, "not found")

		unwrapped := svcErr.Unwrap()
		if unwrapped != nil {
			t.Errorf("Unwrap() = %v, want nil", unwrapped)
		}
	})
}

func TestWrapServiceError(t *testing.T) {
	innerErr := errors.New("database connection lost")

	err := WrapServiceError(constants.ErrCodeQueryError, "query failed", innerErr)

	if err.Code != constants.ErrCodeQueryError {
		t.Errorf("Code = %q, want %q", err.Code, constants.ErrCodeQueryError)
	}
	if err.Message != "query failed" {
		t.Errorf("Message = %q, want %q", err.Message, "query failed")
	}
	if err.Err != innerErr {
		t.Error("Err should be the wrapped error")
	}
}

func TestIsServiceError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode string
		wantOk   bool
	}{
		{
			name:     "detects ServiceError",
			err:      NewServiceError(constants.ErrCodeTopicNotFound, "not found"),
			wantCode: constants.ErrCodeTopicNotFound,
			wantOk:   true,
		},
		{
			name:     "detects wrapped ServiceError",
			err:      fmt.Errorf("wrapped: %w", NewServiceError(constants.ErrCodeAssetNotFound, "asset missing")),
			wantCode: constants.ErrCodeAssetNotFound,
			wantOk:   true,
		},
		{
			name:     "returns false for non-ServiceError",
			err:      errors.New("plain error"),
			wantCode: "",
			wantOk:   false,
		},
		{
			name:     "returns false for nil error",
			err:      nil,
			wantCode: "",
			wantOk:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, ok := IsServiceError(tt.err)
			if ok != tt.wantOk {
				t.Errorf("ok = %v, want %v", ok, tt.wantOk)
			}
			if code != tt.wantCode {
				t.Errorf("code = %q, want %q", code, tt.wantCode)
			}
		})
	}
}

func TestErrorsAs(t *testing.T) {
	t.Run("errors.As works with ServiceError", func(t *testing.T) {
		svcErr := NewServiceError(constants.ErrCodeTopicNotFound, "not found")
		wrappedErr := fmt.Errorf("context: %w", svcErr)

		var target *ServiceError
		if !errors.As(wrappedErr, &target) {
			t.Error("errors.As should find ServiceError in wrapped error")
		}
		if target.Code != constants.ErrCodeTopicNotFound {
			t.Errorf("Code = %q, want %q", target.Code, constants.ErrCodeTopicNotFound)
		}
	})

	t.Run("errors.As returns false for non-ServiceError", func(t *testing.T) {
		plainErr := errors.New("plain error")

		var target *ServiceError
		if errors.As(plainErr, &target) {
			t.Error("errors.As should return false for non-ServiceError")
		}
	})
}

func TestErrorsIs(t *testing.T) {
	t.Run("errors.Is does not match different ServiceError instances", func(t *testing.T) {
		err1 := NewServiceError(constants.ErrCodeTopicNotFound, "not found")
		err2 := NewServiceError(constants.ErrCodeTopicNotFound, "not found")

		// ServiceError does not implement Is, so two different instances should not match
		if errors.Is(err1, err2) {
			t.Error("errors.Is should return false for different ServiceError instances")
		}
	})

	t.Run("errors.Is matches same instance", func(t *testing.T) {
		wrappedErr := fmt.Errorf("context: %w", ErrTopicNotFound)

		if !errors.Is(wrappedErr, ErrTopicNotFound) {
			t.Error("errors.Is should match the same ServiceError instance")
		}
	})
}

func TestErrorChaining(t *testing.T) {
	t.Run("maintains error chain through multiple wraps", func(t *testing.T) {
		rootErr := errors.New("root cause")
		svcErr := WrapServiceError(constants.ErrCodeQueryError, "query failed", rootErr)
		outerErr := fmt.Errorf("handler: %w", svcErr)

		// Should be able to extract ServiceError
		var target *ServiceError
		if !errors.As(outerErr, &target) {
			t.Fatal("should find ServiceError in chain")
		}

		// Should be able to find root error
		if !errors.Is(outerErr, rootErr) {
			t.Error("should find root error in chain")
		}
	})
}

func TestPredefinedErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      *ServiceError
		wantCode string
	}{
		{"ErrTopicNotFound", ErrTopicNotFound, constants.ErrCodeTopicNotFound},
		{"ErrTopicUnhealthy", ErrTopicUnhealthy, constants.ErrCodeTopicUnhealthy},
		{"ErrTopicAlreadyExists", ErrTopicAlreadyExists, constants.ErrCodeTopicAlreadyExists},
		{"ErrInvalidTopicName", ErrInvalidTopicName, constants.ErrCodeInvalidTopicName},
		{"ErrAssetNotFound", ErrAssetNotFound, constants.ErrCodeAssetNotFound},
		{"ErrAssetDuplicate", ErrAssetDuplicate, constants.ErrCodeAssetDuplicate},
		{"ErrAssetTooLarge", ErrAssetTooLarge, constants.ErrCodeAssetTooLarge},
		{"ErrInvalidHash", ErrInvalidHash, constants.ErrCodeInvalidHash},
		{"ErrNotConfigured", ErrNotConfigured, constants.ErrCodeNotConfigured},
		{"ErrMetadataKeyTooLong", ErrMetadataKeyTooLong, constants.ErrCodeMetadataKeyTooLong},
		{"ErrMetadataValueTooLong", ErrMetadataValueTooLong, constants.ErrCodeMetadataValueTooLong},
		{"ErrPresetNotFound", ErrPresetNotFound, constants.ErrCodePresetNotFound},
		{"ErrMissingParam", ErrMissingParam, constants.ErrCodeMissingParam},
		{"ErrQueryError", ErrQueryError, constants.ErrCodeQueryError},
		{"ErrBatchTooManyOperations", ErrBatchTooManyOperations, constants.ErrCodeBatchTooManyOperations},
		{"ErrBatchInvalidOperation", ErrBatchInvalidOperation, constants.ErrCodeBatchInvalidOperation},
		{"ErrBulkDownloadEmpty", ErrBulkDownloadEmpty, constants.ErrCodeBulkDownloadEmpty},
		{"ErrBulkDownloadTooLarge", ErrBulkDownloadTooLarge, constants.ErrCodeBulkDownloadTooLarge},
		{"ErrDownloadSessionNotFound", ErrDownloadSessionNotFound, constants.ErrCodeDownloadSessionNotFound},
		{"ErrDownloadSessionExpired", ErrDownloadSessionExpired, constants.ErrCodeDownloadSessionExpired},
		{"ErrVerificationFailed", ErrVerificationFailed, constants.ErrCodeVerificationFailed},
		{"ErrInternal", ErrInternal, constants.ErrCodeInternalError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code != tt.wantCode {
				t.Errorf("%s.Code = %q, want %q", tt.name, tt.err.Code, tt.wantCode)
			}
			if tt.err.Message == "" {
				t.Errorf("%s.Message should not be empty", tt.name)
			}
		})
	}
}

func TestContextualErrorConstructors(t *testing.T) {
	t.Run("ErrTopicNotFoundWithName includes topic name", func(t *testing.T) {
		err := ErrTopicNotFoundWithName("my-topic")

		if err.Code != constants.ErrCodeTopicNotFound {
			t.Errorf("Code = %q, want %q", err.Code, constants.ErrCodeTopicNotFound)
		}
		if err.Message != "topic not found: my-topic" {
			t.Errorf("Message = %q, want %q", err.Message, "topic not found: my-topic")
		}
	})

	t.Run("ErrTopicUnhealthyWithReason includes topic and reason", func(t *testing.T) {
		err := ErrTopicUnhealthyWithReason("test-topic", "missing index file")

		if err.Code != constants.ErrCodeTopicUnhealthy {
			t.Errorf("Code = %q, want %q", err.Code, constants.ErrCodeTopicUnhealthy)
		}
		if err.Message != "topic test-topic is unhealthy: missing index file" {
			t.Errorf("Message = %q, want expected message", err.Message)
		}
	})

	t.Run("ErrAssetNotFoundWithHash includes hash", func(t *testing.T) {
		hash := "abc123def456"
		err := ErrAssetNotFoundWithHash(hash)

		if err.Code != constants.ErrCodeAssetNotFound {
			t.Errorf("Code = %q, want %q", err.Code, constants.ErrCodeAssetNotFound)
		}
		if err.Message != "asset not found: abc123def456" {
			t.Errorf("Message = %q, want expected message", err.Message)
		}
	})

	t.Run("ErrPresetNotFoundWithName includes preset name", func(t *testing.T) {
		err := ErrPresetNotFoundWithName("recent-imports")

		if err.Code != constants.ErrCodePresetNotFound {
			t.Errorf("Code = %q, want %q", err.Code, constants.ErrCodePresetNotFound)
		}
		if err.Message != "query preset not found: recent-imports" {
			t.Errorf("Message = %q, want expected message", err.Message)
		}
	})

	t.Run("ErrMissingParamWithName includes param name", func(t *testing.T) {
		err := ErrMissingParamWithName("limit")

		if err.Code != constants.ErrCodeMissingParam {
			t.Errorf("Code = %q, want %q", err.Code, constants.ErrCodeMissingParam)
		}
		if err.Message != "required parameter missing: limit" {
			t.Errorf("Message = %q, want expected message", err.Message)
		}
	})
}

func TestWrapHelpers(t *testing.T) {
	innerErr := errors.New("underlying issue")

	t.Run("WrapInternalError wraps with internal code", func(t *testing.T) {
		err := WrapInternalError(innerErr)

		if err.Code != constants.ErrCodeInternalError {
			t.Errorf("Code = %q, want %q", err.Code, constants.ErrCodeInternalError)
		}
		if err.Err != innerErr {
			t.Error("Err should be the wrapped error")
		}
	})

	t.Run("WrapQueryError wraps with query code", func(t *testing.T) {
		err := WrapQueryError(innerErr)

		if err.Code != constants.ErrCodeQueryError {
			t.Errorf("Code = %q, want %q", err.Code, constants.ErrCodeQueryError)
		}
		if err.Err != innerErr {
			t.Error("Err should be the wrapped error")
		}
	})

	t.Run("WrapMetadataError wraps with metadata code", func(t *testing.T) {
		err := WrapMetadataError(innerErr)

		if err.Code != constants.ErrCodeMetadataError {
			t.Errorf("Code = %q, want %q", err.Code, constants.ErrCodeMetadataError)
		}
		if err.Err != innerErr {
			t.Error("Err should be the wrapped error")
		}
	})
}

func TestServiceErrorImplementsError(t *testing.T) {
	// Compile-time check that ServiceError implements error interface
	var _ error = &ServiceError{}
	var _ error = NewServiceError("CODE", "msg")
	var _ error = WrapServiceError("CODE", "msg", nil)
}
