package constants

// API Error Codes
const (
	ErrCodeTopicNotFound      = "TOPIC_NOT_FOUND"
	ErrCodeTopicAlreadyExists = "TOPIC_ALREADY_EXISTS"
	ErrCodeTopicUnhealthy     = "TOPIC_UNHEALTHY"
	ErrCodeInvalidTopicName   = "INVALID_TOPIC_NAME"
	ErrCodeAssetNotFound      = "ASSET_NOT_FOUND"
	ErrCodeAssetTooLarge      = "ASSET_TOO_LARGE"
	ErrCodeAssetDuplicate     = "ASSET_DUPLICATE"
	ErrCodeParentNotFound     = "PARENT_NOT_FOUND"
	ErrCodeInvalidRequest     = "INVALID_REQUEST"
	ErrCodeInternalError      = "INTERNAL_ERROR"
	ErrCodeNotConfigured      = "NOT_CONFIGURED"
	ErrCodeInvalidHash        = "INVALID_HASH"
	ErrCodeMetadataError      = "METADATA_ERROR"
	ErrCodePresetNotFound     = "PRESET_NOT_FOUND"
	ErrCodeQueryError         = "QUERY_ERROR"
	ErrCodeMissingParam       = "MISSING_PARAM"
	ErrCodeVerificationFailed = "VERIFICATION_FAILED"
	ErrCodeStreamingError     = "STREAMING_ERROR"

	// Bulk Download
	ErrCodeBulkDownloadEmpty     = "BULK_DOWNLOAD_EMPTY"
	ErrCodeBulkDownloadTooLarge  = "BULK_DOWNLOAD_TOO_LARGE"
	ErrCodeInvalidFilenameFormat = "INVALID_FILENAME_FORMAT"
	ErrCodeInvalidDownloadMode   = "INVALID_DOWNLOAD_MODE"

	// Bulk Download SSE Sessions
	ErrCodeDownloadSessionNotFound = "DOWNLOAD_SESSION_NOT_FOUND"
	ErrCodeDownloadSessionExpired  = "DOWNLOAD_SESSION_EXPIRED"
	ErrCodeDownloadInProgress      = "DOWNLOAD_IN_PROGRESS"

	// Audit Log
	ErrCodeAuditLogError       = "AUDIT_LOG_ERROR"
	ErrCodeAuditInvalidAction  = "AUDIT_INVALID_ACTION"
	ErrCodeAuditInvalidFilter  = "AUDIT_INVALID_FILTER"

	// Batch Metadata
	ErrCodeBatchTooManyOperations = "BATCH_TOO_MANY_OPERATIONS"
	ErrCodeBatchInvalidOperation  = "BATCH_INVALID_OPERATION"
	ErrCodeBatchPartialFailure    = "BATCH_PARTIAL_FAILURE"

	// Metadata Validation
	ErrCodeMetadataKeyTooLong   = "METADATA_KEY_TOO_LONG"
	ErrCodeMetadataValueTooLong = "METADATA_VALUE_TOO_LONG"

	// Prompts
	ErrCodePromptNotFound = "PROMPT_NOT_FOUND"

	// Monitoring
	ErrCodeLogFileNotFound    = "LOG_FILE_NOT_FOUND"
	ErrCodeLogLevelNotAllowed = "LOG_LEVEL_NOT_ALLOWED"

	// Filename Sanitization
	ErrCodeInvalidFilename = "INVALID_FILENAME"

	// Disk Usage
	ErrCodeDiskLimitExceeded = "DISK_LIMIT_EXCEEDED"
)
