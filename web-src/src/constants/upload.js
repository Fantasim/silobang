// =============================================================================
// UPLOAD CONSTANTS
// =============================================================================

// File status enum
export const FileStatus = {
  PENDING: 'pending',
  UPLOADING: 'uploading',
  SUCCESS: 'success',
  SKIPPED: 'skipped',
  ERROR: 'error',
};

// Maximum concurrent upload workers
export const MAX_CONCURRENT_UPLOADS = 3;

// Maximum items to keep in display queue (browser memory limit)
export const MAX_DISPLAY_ITEMS = 100;

// UI update throttle interval (ms) - batches signal updates
export const UPLOAD_UI_UPDATE_INTERVAL_MS = 150;
