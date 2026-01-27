// =============================================================================
// AUDIT LOG CONSTANTS
// =============================================================================

// Batch size for infinite scroll
export const AUDIT_BATCH_SIZE = 50;

// Maximum entries to keep in memory (browser RAM limit)
export const AUDIT_MAX_ENTRIES = 50000;

// UI update throttle interval for live stream (ms)
export const AUDIT_UI_UPDATE_INTERVAL_MS = 150;

// User filter options
export const AUDIT_USER_FILTER = {
  ALL: 'all',
  ME: 'me',
  OTHERS: 'others',
};

// Connection status
export const AUDIT_CONNECTION_STATUS = {
  DISCONNECTED: 'disconnected',
  CONNECTING: 'connecting',
  CONNECTED: 'connected',
  ERROR: 'error',
};

// Entry source (for visual distinction)
export const AUDIT_ENTRY_SOURCE = {
  LIVE: 'live',
  HISTORICAL: 'historical',
};

// Action type display colors (CSS variable references)
export const AUDIT_ACTION_COLORS = {
  connected: 'var(--terminal-cyan)',
  adding_topic: 'var(--terminal-green)',
  adding_file: 'var(--terminal-green)',
  downloaded: 'var(--terminal-cyan)',
  downloaded_bulk: 'var(--terminal-cyan)',
  downloading: 'var(--terminal-amber)',
  querying: 'var(--terminal-amber)',
  verified: 'var(--terminal-green)',
  default: 'var(--text-secondary)',
};

// Time formatting thresholds (seconds)
export const TIME_THRESHOLDS = {
  JUST_NOW: 5,
  SECONDS: 60,
  MINUTES: 3600,
  HOURS: 86400,
  DAYS: 604800,
};
