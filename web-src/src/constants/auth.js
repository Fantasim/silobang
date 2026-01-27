// =============================================================================
// AUTH ACTIONS — mirrors backend constants.AuthAction*
// =============================================================================

export const AUTH_ACTIONS = {
  UPLOAD: 'upload',
  DOWNLOAD: 'download',
  QUERY: 'query',
  MANAGE_USERS: 'manage_users',
  MANAGE_TOPICS: 'manage_topics',
  METADATA: 'metadata',
  BULK_DOWNLOAD: 'bulk_download',
  VIEW_AUDIT: 'view_audit',
  VERIFY: 'verify',
  MANAGE_CONFIG: 'manage_config',
};

export const ALL_AUTH_ACTIONS = Object.values(AUTH_ACTIONS);

export const AUTH_ACTION_LABELS = {
  [AUTH_ACTIONS.UPLOAD]: 'Upload',
  [AUTH_ACTIONS.DOWNLOAD]: 'Download',
  [AUTH_ACTIONS.QUERY]: 'Query',
  [AUTH_ACTIONS.MANAGE_USERS]: 'Manage Users',
  [AUTH_ACTIONS.MANAGE_TOPICS]: 'Manage Topics',
  [AUTH_ACTIONS.METADATA]: 'Metadata',
  [AUTH_ACTIONS.BULK_DOWNLOAD]: 'Bulk Download',
  [AUTH_ACTIONS.VIEW_AUDIT]: 'View Audit',
  [AUTH_ACTIONS.VERIFY]: 'Verify',
  [AUTH_ACTIONS.MANAGE_CONFIG]: 'Manage Config',
};

export const AUTH_ACTION_DESCRIPTIONS = {
  [AUTH_ACTIONS.UPLOAD]: 'Upload files to topics',
  [AUTH_ACTIONS.DOWNLOAD]: 'Download individual assets',
  [AUTH_ACTIONS.QUERY]: 'Run query presets',
  [AUTH_ACTIONS.MANAGE_USERS]: 'Create, edit, and disable users',
  [AUTH_ACTIONS.MANAGE_TOPICS]: 'Create and delete topics',
  [AUTH_ACTIONS.METADATA]: 'Read and write asset metadata',
  [AUTH_ACTIONS.BULK_DOWNLOAD]: 'Download multiple assets as ZIP',
  [AUTH_ACTIONS.VIEW_AUDIT]: 'View audit logs and stream',
  [AUTH_ACTIONS.VERIFY]: 'Run integrity verification',
  [AUTH_ACTIONS.MANAGE_CONFIG]: 'View and change system configuration',
};

// =============================================================================
// AUTH ERROR CODES — mirrors backend constants.ErrCodeAuth*
// =============================================================================

export const AUTH_ERROR_CODES = {
  AUTH_REQUIRED: 'AUTH_REQUIRED',
  AUTH_INVALID_CREDENTIALS: 'AUTH_INVALID_CREDENTIALS',
  AUTH_FORBIDDEN: 'AUTH_FORBIDDEN',
  AUTH_QUOTA_EXCEEDED: 'AUTH_QUOTA_EXCEEDED',
  AUTH_CONSTRAINT_VIOLATION: 'AUTH_CONSTRAINT_VIOLATION',
  AUTH_USER_NOT_FOUND: 'AUTH_USER_NOT_FOUND',
  AUTH_USER_ALREADY_EXISTS: 'AUTH_USER_ALREADY_EXISTS',
  AUTH_USER_DISABLED: 'AUTH_USER_DISABLED',
  AUTH_SESSION_EXPIRED: 'AUTH_SESSION_EXPIRED',
  AUTH_ESCALATION_DENIED: 'AUTH_ESCALATION_DENIED',
  AUTH_BOOTSTRAP_PROTECTED: 'AUTH_BOOTSTRAP_PROTECTED',
  AUTH_ACCOUNT_LOCKED: 'AUTH_ACCOUNT_LOCKED',
  AUTH_INVALID_GRANT: 'AUTH_INVALID_GRANT',
  AUTH_INVALID_API_KEY: 'AUTH_INVALID_API_KEY',
  AUTH_PASSWORD_TOO_WEAK: 'AUTH_PASSWORD_TOO_WEAK',
  AUTH_USERNAME_INVALID: 'AUTH_USERNAME_INVALID',
};

export const AUTH_ERROR_MESSAGES = {
  [AUTH_ERROR_CODES.AUTH_INVALID_CREDENTIALS]: 'Invalid username or password.',
  [AUTH_ERROR_CODES.AUTH_USER_DISABLED]: 'This account has been disabled. Contact your administrator.',
  [AUTH_ERROR_CODES.AUTH_ACCOUNT_LOCKED]: 'Account temporarily locked due to too many failed attempts. Try again later.',
  [AUTH_ERROR_CODES.AUTH_REQUIRED]: 'Authentication required. Please log in.',
  [AUTH_ERROR_CODES.AUTH_SESSION_EXPIRED]: 'Your session has expired. Please log in again.',
  [AUTH_ERROR_CODES.AUTH_FORBIDDEN]: 'You do not have permission to perform this action.',
  [AUTH_ERROR_CODES.AUTH_QUOTA_EXCEEDED]: 'Daily quota exceeded for this action.',
  [AUTH_ERROR_CODES.AUTH_ESCALATION_DENIED]: 'Cannot grant permissions you do not have.',
  [AUTH_ERROR_CODES.AUTH_BOOTSTRAP_PROTECTED]: 'Bootstrap admin account is protected.',
  [AUTH_ERROR_CODES.AUTH_USER_ALREADY_EXISTS]: 'A user with this username already exists.',
  [AUTH_ERROR_CODES.AUTH_PASSWORD_TOO_WEAK]: 'Password does not meet minimum requirements.',
  [AUTH_ERROR_CODES.AUTH_USERNAME_INVALID]: 'Username must be 3-64 characters: lowercase letters, numbers, hyphens, underscores.',
};

// =============================================================================
// AUTH STATUS — frontend auth state machine
// =============================================================================

export const AUTH_STATUS = {
  LOADING: 'loading',
  UNCONFIGURED: 'unconfigured',
  UNAUTHENTICATED: 'unauthenticated',
  AUTHENTICATED: 'authenticated',
};

// =============================================================================
// AUTH METHODS
// =============================================================================

export const AUTH_METHODS = {
  SESSION: 'session',
  API_KEY: 'api_key',
};

// =============================================================================
// TOKEN / STORAGE
// =============================================================================

export const AUTH_TOKEN_STORAGE_KEY = 'meshbank_session_token';
export const AUTH_TOKEN_QUERY_PARAM = 'token';

// =============================================================================
// VALIDATION — mirrors backend constants.Auth*
// =============================================================================

export const AUTH_PASSWORD_MIN_LENGTH = 12;
export const AUTH_PASSWORD_MAX_LENGTH = 128;
export const AUTH_USERNAME_PATTERN = /^[a-z0-9_-]{3,64}$/;

// =============================================================================
// CONSTRAINT FIELD DEFINITIONS — for grant management UI
// =============================================================================

/** Suggestion data sources for constraint fields */
export const CONSTRAINT_SUGGEST = {
  TOPICS: 'topics',
  PRESETS: 'presets',
  ACTIONS: 'actions',
};

/** Byte unit definitions for ByteInput decomposition/recomposition */
export const BYTE_UNITS = [
  { label: 'B', value: 1 },
  { label: 'KB', value: 1024 },
  { label: 'MB', value: 1024 * 1024 },
  { label: 'GB', value: 1024 * 1024 * 1024 },
  { label: 'TB', value: 1024 * 1024 * 1024 * 1024 },
];

/** Default byte unit when creating new values */
export const BYTE_UNIT_DEFAULT = 'MB';

export const AUTH_CONSTRAINT_FIELDS = {
  [AUTH_ACTIONS.UPLOAD]: [
    { key: 'allowed_extensions', type: 'string_array', label: 'Allowed Extensions', placeholder: 'png, jpg, obj' },
    { key: 'max_file_size_bytes', type: 'bytes', label: 'Max File Size' },
    { key: 'daily_count_limit', type: 'number', label: 'Daily Upload Limit' },
    { key: 'daily_volume_bytes', type: 'bytes', label: 'Daily Volume Limit' },
    { key: 'allowed_topics', type: 'string_array', label: 'Allowed Topics', placeholder: 'Or type a custom topic...', suggest: CONSTRAINT_SUGGEST.TOPICS },
  ],
  [AUTH_ACTIONS.DOWNLOAD]: [
    { key: 'daily_count_limit', type: 'number', label: 'Daily Download Limit' },
    { key: 'daily_volume_bytes', type: 'bytes', label: 'Daily Volume Limit' },
    { key: 'allowed_topics', type: 'string_array', label: 'Allowed Topics', placeholder: 'Or type a custom topic...', suggest: CONSTRAINT_SUGGEST.TOPICS },
  ],
  [AUTH_ACTIONS.QUERY]: [
    { key: 'allowed_presets', type: 'string_array', label: 'Allowed Presets', placeholder: 'Or type a custom preset...', suggest: CONSTRAINT_SUGGEST.PRESETS },
    { key: 'daily_count_limit', type: 'number', label: 'Daily Query Limit' },
    { key: 'allowed_topics', type: 'string_array', label: 'Allowed Topics', placeholder: 'Or type a custom topic...', suggest: CONSTRAINT_SUGGEST.TOPICS },
  ],
  [AUTH_ACTIONS.MANAGE_USERS]: [
    { key: 'can_create', type: 'boolean', label: 'Can Create Users' },
    { key: 'can_edit', type: 'boolean', label: 'Can Edit Users' },
    { key: 'can_disable', type: 'boolean', label: 'Can Disable Users' },
    { key: 'can_grant_actions', type: 'string_array', label: 'Can Grant Actions', placeholder: 'Or type an action...', suggest: CONSTRAINT_SUGGEST.ACTIONS },
    { key: 'escalation_allowed', type: 'boolean', label: 'Escalation Allowed' },
  ],
  [AUTH_ACTIONS.MANAGE_TOPICS]: [
    { key: 'can_create', type: 'boolean', label: 'Can Create Topics' },
    { key: 'can_delete', type: 'boolean', label: 'Can Delete Topics' },
    { key: 'allowed_topics', type: 'string_array', label: 'Allowed Topics', placeholder: 'Or type a custom topic...', suggest: CONSTRAINT_SUGGEST.TOPICS },
  ],
  [AUTH_ACTIONS.METADATA]: [
    { key: 'daily_count_limit', type: 'number', label: 'Daily Operation Limit' },
    { key: 'allowed_topics', type: 'string_array', label: 'Allowed Topics', placeholder: 'Or type a custom topic...', suggest: CONSTRAINT_SUGGEST.TOPICS },
  ],
  [AUTH_ACTIONS.BULK_DOWNLOAD]: [
    { key: 'daily_count_limit', type: 'number', label: 'Daily Download Limit' },
    { key: 'daily_volume_bytes', type: 'bytes', label: 'Daily Volume Limit' },
    { key: 'max_assets_per_request', type: 'number', label: 'Max Assets Per Request' },
  ],
  [AUTH_ACTIONS.VIEW_AUDIT]: [
    { key: 'can_view_all', type: 'boolean', label: 'Can View All Users' },
    { key: 'can_stream', type: 'boolean', label: 'Can Stream Live' },
  ],
  [AUTH_ACTIONS.VERIFY]: [
    { key: 'daily_count_limit', type: 'number', label: 'Daily Verification Limit' },
  ],
  [AUTH_ACTIONS.MANAGE_CONFIG]: [],
};

// =============================================================================
// USERS PAGE UI CONSTANTS
// =============================================================================

export const USERS_DETAIL_TABS = [
  { id: 'info', label: 'Info', description: 'User account details' },
  { id: 'grants', label: 'Grants', description: 'Action permissions and constraints' },
  { id: 'quota', label: 'Quota', description: 'Daily usage statistics' },
];

export const USERS_SEARCH_PLACEHOLDER = 'Search users...';
export const USERS_EMPTY_STATE_TEXT = 'Select a user to view details';
export const USERS_NO_CONSTRAINTS_TEXT = 'This action has no configurable constraints.';
export const USERS_NO_SEARCH_RESULTS_TEXT = 'No users match your search.';
