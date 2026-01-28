package e2e

// UploadResponse represents the JSON response from asset upload
type UploadResponse struct {
	Hash          string `json:"hash"`
	Skipped       bool   `json:"skipped"`
	ExistingTopic string `json:"existing_topic,omitempty"`
	Blob          string `json:"blob,omitempty"`
	Size          int64  `json:"size,omitempty"`
}

// QueryResponse represents the JSON response from query execution
type QueryResponse struct {
	Preset   string          `json:"preset"`
	RowCount int             `json:"row_count"`
	Columns  []string        `json:"columns"`
	Rows     [][]interface{} `json:"rows"`
}

// TopicInfo represents a single topic in the topics list
type TopicInfo struct {
	Name    string                 `json:"name"`
	Healthy bool                   `json:"healthy"`
	Error   string                 `json:"error,omitempty"`
	Stats   map[string]interface{} `json:"stats,omitempty"`
}

// TopicsResponse represents the JSON response from GET /api/topics
type TopicsResponse struct {
	Topics  []TopicInfo  `json:"topics"`
	Service *ServiceInfo `json:"service,omitempty"`
}

// ServiceInfo holds service-level metrics
type ServiceInfo struct {
	OrchestratorDBSize int64          `json:"orchestrator_db_size"`
	TotalIndexedHashes int64          `json:"total_indexed_hashes"`
	TopicsSummary      TopicsSummary  `json:"topics_summary"`
	StorageSummary     StorageSummary `json:"storage_summary"`
	MaxDiskUsageBytes  int64          `json:"max_disk_usage_bytes"`
	VersionInfo        VersionInfo    `json:"version_info"`
}

// TopicsSummary provides counts of topics by health status
type TopicsSummary struct {
	Total     int `json:"total"`
	Healthy   int `json:"healthy"`
	Unhealthy int `json:"unhealthy"`
}

// StorageSummary provides aggregated storage metrics across all topics
type StorageSummary struct {
	TotalDatSize   int64   `json:"total_dat_size"`
	TotalDbSize    int64   `json:"total_db_size"`
	TotalAssetSize int64   `json:"total_asset_size"`
	TotalDatFiles  int     `json:"total_dat_files"`
	AvgAssetSize   float64 `json:"avg_asset_size"`
}

// VersionInfo provides version and format information
type VersionInfo struct {
	AppVersion  string `json:"app_version"`
	BlobVersion uint16 `json:"blob_version"`
	HeaderSize  int    `json:"header_size"`
}

// MetadataResponse represents the JSON response from metadata operations
type MetadataResponse struct {
	Success          bool                   `json:"success"`
	LogID            int64                  `json:"log_id"`
	ComputedMetadata map[string]interface{} `json:"computed_metadata"`
}

// ErrorResponse represents a JSON error response from the API
type ErrorResponse struct {
	Error   bool   `json:"error"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// BulkDownloadRequest represents the request body for bulk downloads
type BulkDownloadRequest struct {
	Mode            string                 `json:"mode"`
	Preset          string                 `json:"preset,omitempty"`
	Params          map[string]interface{} `json:"params,omitempty"`
	Topics          []string               `json:"topics,omitempty"`
	AssetIDs        []string               `json:"asset_ids,omitempty"`
	IncludeMetadata bool                   `json:"include_metadata"`
	FilenameFormat  string                 `json:"filename_format,omitempty"`
}

// BulkDownloadManifest represents the manifest.json content in ZIP
type BulkDownloadManifest struct {
	CreatedAt       int64                    `json:"created_at"`
	AssetCount      int                      `json:"asset_count"`
	TotalSize       int64                    `json:"total_size"`
	IncludeMetadata bool                     `json:"include_metadata"`
	Assets          []BulkDownloadAssetInfo  `json:"assets"`
	FailedAssets    []BulkDownloadFailedInfo `json:"failed_assets,omitempty"`
}

// BulkDownloadAssetInfo represents an asset in the manifest
type BulkDownloadAssetInfo struct {
	Hash       string `json:"hash"`
	Filename   string `json:"filename"`
	Size       int64  `json:"size"`
	Extension  string `json:"extension"`
	OriginName string `json:"origin_name"`
	Topic      string `json:"topic"`
}

// BulkDownloadFailedInfo represents a failed asset in the manifest
type BulkDownloadFailedInfo struct {
	Hash  string `json:"hash"`
	Error string `json:"error"`
	Topic string `json:"topic,omitempty"`
}

// AssetMetadataFile represents the per-asset metadata JSON file content
type AssetMetadataFile struct {
	Asset            AssetMetadataInfo      `json:"asset"`
	ComputedMetadata map[string]interface{} `json:"computed_metadata"`
}

// AssetMetadataInfo contains asset information in metadata files
type AssetMetadataInfo struct {
	Hash       string  `json:"hash"`
	Size       int64   `json:"size"`
	Extension  string  `json:"extension"`
	OriginName string  `json:"origin_name"`
	ParentID   *string `json:"parent_id,omitempty"`
	CreatedAt  int64   `json:"created_at"`
	Topic      string  `json:"topic"`
	BlobName   string  `json:"blob_name"`
}

// BulkDownloadSSE types

// BulkDownloadSSEEvent represents an SSE event from bulk download
type BulkDownloadSSEEvent struct {
	Type      string                 `json:"type"`
	Timestamp int64                  `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// DownloadStartEventData represents the data payload for download_start event
type DownloadStartEventData struct {
	DownloadID  string `json:"download_id"`
	TotalAssets int    `json:"total_assets"`
	TotalBytes  int64  `json:"total_bytes"`
	Mode        string `json:"mode"`
}

// AssetProgressEventData represents the data payload for asset_progress event
type AssetProgressEventData struct {
	DownloadID  string `json:"download_id"`
	AssetIndex  int    `json:"asset_index"`
	TotalAssets int    `json:"total_assets"`
	Hash        string `json:"hash"`
	Topic       string `json:"topic"`
	Size        int64  `json:"size"`
	Filename    string `json:"filename"`
}

// ZipProgressEventData represents the data payload for zip_progress event
type ZipProgressEventData struct {
	DownloadID      string `json:"download_id"`
	BytesWritten    int64  `json:"bytes_written"`
	TotalBytes      int64  `json:"total_bytes"`
	PercentComplete int    `json:"percent_complete"`
}

// DownloadCompleteEventData represents the data payload for complete event
type DownloadCompleteEventData struct {
	DownloadID   string `json:"download_id"`
	DownloadURL  string `json:"download_url"`
	TotalAssets  int    `json:"total_assets"`
	TotalSize    int64  `json:"total_size"`
	FailedAssets int    `json:"failed_assets"`
	DurationMs   int    `json:"duration_ms"`
	ExpiresAt    int64  `json:"expires_at"`
}

// DownloadErrorEventData represents the data payload for error event
type DownloadErrorEventData struct {
	DownloadID string `json:"download_id"`
	Message    string `json:"message"`
	Code       string `json:"code"`
}

// Audit Log types

// AuditEntry represents a single audit log entry
type AuditEntry struct {
	ID        int64       `json:"id"`
	Timestamp int64       `json:"timestamp"`
	Action    string      `json:"action"`
	IPAddress string      `json:"ip_address"`
	Username  string      `json:"username"`
	Details   interface{} `json:"details,omitempty"`
}

// AuditQueryResponse represents the response from GET /api/audit
type AuditQueryResponse struct {
	Entries []AuditEntry `json:"entries"`
	Total   int64        `json:"total"`
	Limit   int          `json:"limit"`
	Offset  int          `json:"offset"`
}

// AuditActionsResponse represents the response from GET /api/audit/actions
type AuditActionsResponse struct {
	Actions []string `json:"actions"`
}

// Batch Metadata types

// BatchMetadataOperation represents a single operation in batch request
type BatchMetadataOperation struct {
	Hash  string      `json:"hash"`
	Op    string      `json:"op"`
	Key   string      `json:"key"`
	Value interface{} `json:"value,omitempty"`
}

// BatchMetadataRequest represents the request body for POST /api/metadata/batch
type BatchMetadataRequest struct {
	Operations       []BatchMetadataOperation `json:"operations"`
	Processor        string                   `json:"processor"`
	ProcessorVersion string                   `json:"processor_version"`
}

// BatchMetadataResult represents a single operation result
type BatchMetadataResult struct {
	Hash    string `json:"hash"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	LogID   int64  `json:"log_id,omitempty"`
}

// BatchMetadataResponse represents the response from batch metadata operations
type BatchMetadataResponse struct {
	Success   bool                  `json:"success"`
	Total     int                   `json:"total"`
	Succeeded int                   `json:"succeeded"`
	Failed    int                   `json:"failed"`
	Results   []BatchMetadataResult `json:"results"`
}

// ApplyMetadataRequest represents the request body for POST /api/metadata/apply
type ApplyMetadataRequest struct {
	QueryPreset      string                 `json:"query_preset"`
	QueryParams      map[string]interface{} `json:"query_params,omitempty"`
	Topics           []string               `json:"topics,omitempty"`
	Op               string                 `json:"op"`
	Key              string                 `json:"key"`
	Value            interface{}            `json:"value,omitempty"`
	Processor        string                 `json:"processor"`
	ProcessorVersion string                 `json:"processor_version"`
}

// Monitoring types

// MonitoringResponse represents the JSON response from GET /api/monitoring
type MonitoringResponse struct {
	System      MonitoringSystem      `json:"system"`
	Application MonitoringApplication `json:"application"`
	Logs        MonitoringLogs        `json:"logs"`
	Service     *ServiceInfo          `json:"service,omitempty"`
}

// MonitoringSystem holds OS-level resource metrics
type MonitoringSystem struct {
	RAMUsedBytes        uint64 `json:"ram_used_bytes"`
	RAMTotalBytes       uint64 `json:"ram_total_bytes"`
	ProjectDirSizeBytes uint64 `json:"project_dir_size_bytes"`
}

// MonitoringApplication holds application-level configuration and state metrics
type MonitoringApplication struct {
	UptimeSeconds         int64  `json:"uptime_seconds"`
	StartedAt             int64  `json:"started_at"`
	WorkingDirectory      string `json:"working_directory"`
	Port                  int    `json:"port"`
	MaxDatSizeBytes       int64  `json:"max_dat_size_bytes"`
	MaxMetadataValueBytes int    `json:"max_metadata_value_bytes"`
	MaxDiskUsageBytes     int64  `json:"max_disk_usage_bytes"`
	TopicsTotal           int    `json:"topics_total"`
	TopicsHealthy         int    `json:"topics_healthy"`
	TopicsUnhealthy       int    `json:"topics_unhealthy"`
	TotalIndexedHashes    int64  `json:"total_indexed_hashes"`
}

// MonitoringLogs holds log file summaries per level
type MonitoringLogs struct {
	Levels []MonitoringLogLevel `json:"levels"`
}

// MonitoringLogLevel holds the summary for a single log level directory
type MonitoringLogLevel struct {
	Level     string              `json:"level"`
	FileCount int                 `json:"file_count"`
	TotalSize int64               `json:"total_size"`
	Files     []MonitoringLogFile `json:"files,omitempty"`
}

// MonitoringLogFile holds metadata about a single log file
type MonitoringLogFile struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"mod_time"`
}
