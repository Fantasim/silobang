package services

import (
	"errors"
	"fmt"

	"meshbank/internal/constants"
)

// ServiceError represents a service-level error with an error code
type ServiceError struct {
	Code    string
	Message string
	Err     error
}

func (e *ServiceError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *ServiceError) Unwrap() error {
	return e.Err
}

// NewServiceError creates a new service error
func NewServiceError(code, message string) *ServiceError {
	return &ServiceError{Code: code, Message: message}
}

// WrapServiceError wraps an existing error with a service error
func WrapServiceError(code, message string, err error) *ServiceError {
	return &ServiceError{Code: code, Message: message, Err: err}
}

// IsServiceError checks if an error is a ServiceError and returns its code
func IsServiceError(err error) (string, bool) {
	var svcErr *ServiceError
	if errors.As(err, &svcErr) {
		return svcErr.Code, true
	}
	return "", false
}

// Pre-defined service errors for common cases
var (
	// Topic errors
	ErrTopicNotFound = NewServiceError(constants.ErrCodeTopicNotFound, "topic not found")
	ErrTopicUnhealthy = NewServiceError(constants.ErrCodeTopicUnhealthy, "topic is unhealthy")
	ErrTopicAlreadyExists = NewServiceError(constants.ErrCodeTopicAlreadyExists, "topic already exists")
	ErrInvalidTopicName = NewServiceError(constants.ErrCodeInvalidTopicName, "invalid topic name")

	// Asset errors
	ErrAssetNotFound = NewServiceError(constants.ErrCodeAssetNotFound, "asset not found")
	ErrAssetDuplicate = NewServiceError(constants.ErrCodeAssetDuplicate, "asset already exists")
	ErrAssetTooLarge = NewServiceError(constants.ErrCodeAssetTooLarge, "asset exceeds maximum size")
	ErrInvalidHash = NewServiceError(constants.ErrCodeInvalidHash, "invalid hash format")

	// Config errors
	ErrNotConfigured = NewServiceError(constants.ErrCodeNotConfigured, "working directory not configured")

	// Metadata errors
	ErrMetadataKeyTooLong = NewServiceError(constants.ErrCodeMetadataKeyTooLong, "metadata key exceeds maximum length")
	ErrMetadataValueTooLong = NewServiceError(constants.ErrCodeMetadataValueTooLong, "metadata value exceeds maximum size")

	// Query errors
	ErrPresetNotFound = NewServiceError(constants.ErrCodePresetNotFound, "query preset not found")
	ErrMissingParam = NewServiceError(constants.ErrCodeMissingParam, "required parameter missing")
	ErrQueryError = NewServiceError(constants.ErrCodeQueryError, "query execution failed")

	// Batch errors
	ErrBatchTooManyOperations = NewServiceError(constants.ErrCodeBatchTooManyOperations, "batch exceeds maximum operations")
	ErrBatchInvalidOperation = NewServiceError(constants.ErrCodeBatchInvalidOperation, "invalid batch operation")

	// Bulk download errors
	ErrBulkDownloadEmpty = NewServiceError(constants.ErrCodeBulkDownloadEmpty, "no assets to download")
	ErrBulkDownloadTooLarge = NewServiceError(constants.ErrCodeBulkDownloadTooLarge, "download exceeds maximum size")
	ErrDownloadSessionNotFound = NewServiceError(constants.ErrCodeDownloadSessionNotFound, "download session not found")
	ErrDownloadSessionExpired = NewServiceError(constants.ErrCodeDownloadSessionExpired, "download session expired")

	// Verification errors
	ErrVerificationFailed = NewServiceError(constants.ErrCodeVerificationFailed, "verification failed")

	// Monitoring errors
	ErrLogFileNotFound    = NewServiceError(constants.ErrCodeLogFileNotFound, "log file not found")
	ErrLogLevelNotAllowed = NewServiceError(constants.ErrCodeLogLevelNotAllowed, "log level not accessible")

	// Auth errors
	ErrAuthRequired           = NewServiceError(constants.ErrCodeAuthRequired, "authentication required")
	ErrAuthInvalidCredentials = NewServiceError(constants.ErrCodeAuthInvalidCredentials, "invalid credentials")
	ErrAuthForbidden          = NewServiceError(constants.ErrCodeAuthForbidden, "access denied")
	ErrAuthQuotaExceeded      = NewServiceError(constants.ErrCodeAuthQuotaExceeded, "quota exceeded")
	ErrAuthConstraintViolation = NewServiceError(constants.ErrCodeAuthConstraintViolation, "constraint violation")
	ErrAuthUserNotFound       = NewServiceError(constants.ErrCodeAuthUserNotFound, "user not found")
	ErrAuthUserExists         = NewServiceError(constants.ErrCodeAuthUserExists, "user already exists")
	ErrAuthUserDisabled       = NewServiceError(constants.ErrCodeAuthUserDisabled, "user account is disabled")
	ErrAuthSessionExpired     = NewServiceError(constants.ErrCodeAuthSessionExpired, "session expired")
	ErrAuthEscalationDenied   = NewServiceError(constants.ErrCodeAuthEscalationDenied, "escalation denied")
	ErrAuthBootstrapProtected = NewServiceError(constants.ErrCodeAuthBootstrapProtected, "bootstrap user is protected")
	ErrAuthAccountLocked      = NewServiceError(constants.ErrCodeAuthAccountLocked, "account is temporarily locked")
	ErrAuthInvalidGrant       = NewServiceError(constants.ErrCodeAuthInvalidGrant, "invalid grant")
	ErrAuthInvalidAPIKey      = NewServiceError(constants.ErrCodeAuthInvalidAPIKey, "invalid API key")
	ErrAuthPasswordTooWeak    = NewServiceError(constants.ErrCodeAuthPasswordTooWeak, "password does not meet requirements")
	ErrAuthUsernameInvalid      = NewServiceError(constants.ErrCodeAuthUsernameInvalid, "invalid username format")
	ErrAuthInvalidConstraints   = NewServiceError(constants.ErrCodeAuthInvalidConstraints, "invalid grant constraints")
	ErrAuthGrantActionDenied    = NewServiceError(constants.ErrCodeAuthGrantActionDenied, "not permitted to grant this action")

	// Internal errors
	ErrInternal = NewServiceError(constants.ErrCodeInternalError, "internal server error")
)

// Topic errors with context
func ErrTopicNotFoundWithName(name string) *ServiceError {
	return &ServiceError{
		Code:    constants.ErrCodeTopicNotFound,
		Message: fmt.Sprintf("topic not found: %s", name),
	}
}

func ErrTopicUnhealthyWithReason(name, reason string) *ServiceError {
	return &ServiceError{
		Code:    constants.ErrCodeTopicUnhealthy,
		Message: fmt.Sprintf("topic %s is unhealthy: %s", name, reason),
	}
}

// Asset errors with context
func ErrAssetNotFoundWithHash(hash string) *ServiceError {
	return &ServiceError{
		Code:    constants.ErrCodeAssetNotFound,
		Message: fmt.Sprintf("asset not found: %s", hash),
	}
}

// Query errors with context
func ErrPresetNotFoundWithName(name string) *ServiceError {
	return &ServiceError{
		Code:    constants.ErrCodePresetNotFound,
		Message: fmt.Sprintf("query preset not found: %s", name),
	}
}

func ErrMissingParamWithName(name string) *ServiceError {
	return &ServiceError{
		Code:    constants.ErrCodeMissingParam,
		Message: fmt.Sprintf("required parameter missing: %s", name),
	}
}

// Wrap internal errors
func WrapInternalError(err error) *ServiceError {
	return WrapServiceError(constants.ErrCodeInternalError, "internal error", err)
}

func WrapQueryError(err error) *ServiceError {
	return WrapServiceError(constants.ErrCodeQueryError, "query execution failed", err)
}

func WrapMetadataError(err error) *ServiceError {
	return WrapServiceError(constants.ErrCodeMetadataError, "metadata operation failed", err)
}
