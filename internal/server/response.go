package server

import (
	"encoding/json"
	"net/http"

	"silobang/internal/constants"
	"silobang/internal/services"
)

// APIError represents a standard error response
type APIError struct {
	Error   bool   `json:"error"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// WriteJSON writes a JSON response with the given status code
func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// WriteError writes a standard error response
func WriteError(w http.ResponseWriter, status int, message string, code string) {
	WriteJSON(w, status, APIError{
		Error:   true,
		Message: message,
		Code:    code,
	})
}

// WriteSuccess writes a simple success response
func WriteSuccess(w http.ResponseWriter, data interface{}) {
	WriteJSON(w, http.StatusOK, data)
}

// handleServiceError maps service errors to HTTP responses.
// It extracts the error code from ServiceError and maps it to the appropriate HTTP status.
func (s *Server) handleServiceError(w http.ResponseWriter, err error) {
	code, isServiceErr := services.IsServiceError(err)
	if !isServiceErr {
		WriteError(w, http.StatusInternalServerError, err.Error(), constants.ErrCodeInternalError)
		return
	}

	// Map error codes to HTTP status codes
	status := http.StatusInternalServerError
	switch code {
	case constants.ErrCodeAssetNotFound, constants.ErrCodeTopicNotFound, constants.ErrCodePresetNotFound, constants.ErrCodePromptNotFound,
		constants.ErrCodeLogFileNotFound:
		status = http.StatusNotFound
	case constants.ErrCodeAuthRequired, constants.ErrCodeAuthInvalidCredentials,
		constants.ErrCodeAuthSessionExpired:
		status = http.StatusUnauthorized
	case constants.ErrCodeAuthForbidden, constants.ErrCodeAuthConstraintViolation,
		constants.ErrCodeAuthEscalationDenied, constants.ErrCodeAuthBootstrapProtected,
		constants.ErrCodeAuthUserDisabled, constants.ErrCodeLogLevelNotAllowed,
		constants.ErrCodeAuthGrantActionDenied:
		status = http.StatusForbidden
	case constants.ErrCodeAuthQuotaExceeded, constants.ErrCodeAuthAccountLocked:
		status = http.StatusTooManyRequests
	case constants.ErrCodeAuthUserNotFound:
		status = http.StatusNotFound
	case constants.ErrCodeAuthInvalidGrant, constants.ErrCodeAuthInvalidAPIKey,
		constants.ErrCodeAuthPasswordTooWeak, constants.ErrCodeAuthUsernameInvalid,
		constants.ErrCodeAuthInvalidConstraints:
		status = http.StatusBadRequest
	case constants.ErrCodeAssetDuplicate, constants.ErrCodeTopicAlreadyExists,
		constants.ErrCodeAuthUserExists:
		status = http.StatusConflict
	case constants.ErrCodeAssetTooLarge:
		status = http.StatusRequestEntityTooLarge
	case constants.ErrCodeInvalidRequest, constants.ErrCodeInvalidHash, constants.ErrCodeInvalidTopicName,
		constants.ErrCodeParentNotFound, constants.ErrCodeMissingParam, constants.ErrCodeMetadataKeyTooLong,
		constants.ErrCodeMetadataValueTooLong, constants.ErrCodeBatchInvalidOperation, constants.ErrCodeBatchTooManyOperations,
		constants.ErrCodeTopicUnhealthy,
		constants.ErrCodeBulkDownloadEmpty, constants.ErrCodeBulkDownloadTooLarge,
		constants.ErrCodeInvalidFilenameFormat, constants.ErrCodeInvalidDownloadMode:
		status = http.StatusBadRequest
	case constants.ErrCodeNotConfigured:
		status = http.StatusBadRequest
	case constants.ErrCodeQueryError, constants.ErrCodeMetadataError:
		status = http.StatusInternalServerError
	case constants.ErrCodeDiskLimitExceeded:
		status = http.StatusInsufficientStorage
	}

	WriteError(w, status, err.Error(), code)
}
