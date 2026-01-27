package auth

// UploadConstraints defines limits and rules for the upload action.
// All fields are optional — zero/nil values mean no restriction on that dimension.
type UploadConstraints struct {
	AllowedExtensions []string `json:"allowed_extensions,omitempty"` // empty = all allowed
	MaxFileSizeBytes  int64    `json:"max_file_size_bytes,omitempty"`
	DailyCountLimit   int64    `json:"daily_count_limit,omitempty"`
	DailyVolumeBytes  int64    `json:"daily_volume_bytes,omitempty"`
	AllowedTopics     []string `json:"allowed_topics,omitempty"` // empty = all allowed
}

// DownloadConstraints defines limits for the download action.
type DownloadConstraints struct {
	DailyCountLimit  int64    `json:"daily_count_limit,omitempty"`
	DailyVolumeBytes int64    `json:"daily_volume_bytes,omitempty"`
	AllowedTopics    []string `json:"allowed_topics,omitempty"`
}

// QueryConstraints defines limits for the query action.
type QueryConstraints struct {
	AllowedPresets  []string `json:"allowed_presets,omitempty"` // empty = all presets
	DailyCountLimit int64    `json:"daily_count_limit,omitempty"`
	AllowedTopics   []string `json:"allowed_topics,omitempty"`
}

// ManageUsersConstraints defines what user management operations are allowed.
type ManageUsersConstraints struct {
	CanCreate         bool     `json:"can_create"`
	CanEdit           bool     `json:"can_edit"`
	CanDisable        bool     `json:"can_disable"`
	CanGrantActions   []string `json:"can_grant_actions,omitempty"` // empty = all they have
	EscalationAllowed bool     `json:"escalation_allowed"`
}

// ManageTopicsConstraints defines what topic management operations are allowed.
type ManageTopicsConstraints struct {
	CanCreate     bool     `json:"can_create"`
	CanDelete     bool     `json:"can_delete"`
	AllowedTopics []string `json:"allowed_topics,omitempty"`
}

// MetadataConstraints defines limits for metadata operations.
type MetadataConstraints struct {
	DailyCountLimit int64    `json:"daily_count_limit,omitempty"`
	AllowedTopics   []string `json:"allowed_topics,omitempty"`
}

// BulkDownloadConstraints defines limits for bulk download operations.
type BulkDownloadConstraints struct {
	DailyCountLimit    int64    `json:"daily_count_limit,omitempty"`
	DailyVolumeBytes   int64    `json:"daily_volume_bytes,omitempty"`
	MaxAssetsPerRequest int     `json:"max_assets_per_request,omitempty"`
}

// ViewAuditConstraints defines what audit access is allowed.
type ViewAuditConstraints struct {
	CanViewAll bool `json:"can_view_all"` // false = only own actions
	CanStream  bool `json:"can_stream"`   // false = no SSE streaming
}

// VerifyConstraints defines limits for verification operations.
type VerifyConstraints struct {
	DailyCountLimit int64 `json:"daily_count_limit,omitempty"`
}
