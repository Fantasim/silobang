import { signal, computed, batch } from '@preact/signals';
import { api } from '@services/api';
import { currentUser } from '@store/auth';
import {
  AUDIT_BATCH_SIZE,
  AUDIT_MAX_ENTRIES,
  AUDIT_UI_UPDATE_INTERVAL_MS,
  AUDIT_USER_FILTER,
  AUDIT_CONNECTION_STATUS,
  AUDIT_ENTRY_SOURCE,
} from '@constants/audit';

// Re-export constants for backward compatibility
export {
  AUDIT_BATCH_SIZE,
  AUDIT_MAX_ENTRIES,
  AUDIT_UI_UPDATE_INTERVAL_MS,
  AUDIT_USER_FILTER,
  AUDIT_CONNECTION_STATUS,
  AUDIT_ENTRY_SOURCE,
  AUDIT_ACTION_COLORS,
  TIME_THRESHOLDS,
} from '@constants/audit';

// =============================================================================
// STATE SIGNALS
// =============================================================================

// Connection state
export const connectionStatus = signal(AUDIT_CONNECTION_STATUS.DISCONNECTED);
export const connectionError = signal(null);

// Live stream control
export const isLiveEnabled = signal(true);
export const isLivePaused = signal(false);

// Available action types (from API)
export const availableActions = signal([]);
export const actionsLoading = signal(false);

// Filters
export const userFilter = signal(AUDIT_USER_FILTER.ALL);
export const selectedActions = signal([]);

// Entries storage
let _liveEntries = [];
let _historicalEntries = [];
let _updateScheduled = false;

export const liveEntries = signal([]);
export const historicalEntries = signal([]);

// Pagination for historical fetch
export const historicalOffset = signal(0);
export const historicalHasMore = signal(true);
export const historicalLoading = signal(false);

// Current client IP (from connected event)
export const currentClientIP = signal(null);

// =============================================================================
// COMPUTED
// =============================================================================

// Combined and filtered entries for display
export const displayEntries = computed(() => {
  const live = liveEntries.value;
  const historical = historicalEntries.value;
  const filterUser = userFilter.value;
  const filterActions = selectedActions.value;
  const myUsername = currentUser.value?.username;
  const paused = isLivePaused.value;

  // Filter function
  const passesFilter = (entry) => {
    // User filter using username
    if (myUsername && filterUser !== AUDIT_USER_FILTER.ALL) {
      if (filterUser === AUDIT_USER_FILTER.ME && entry.username !== myUsername) {
        return false;
      }
      if (filterUser === AUDIT_USER_FILTER.OTHERS && entry.username === myUsername) {
        return false;
      }
    }

    // Action filter
    if (filterActions.length > 0 && !filterActions.includes(entry.action)) {
      return false;
    }

    return true;
  };

  // If paused, don't show new live entries (but keep receiving them)
  const visibleLive = paused ? [] : live.filter(passesFilter);
  const visibleHistorical = historical.filter(passesFilter);

  return [...visibleLive, ...visibleHistorical];
});

// Count of pending live entries when paused
export const pendingLiveCount = computed(() => {
  if (!isLivePaused.value) return 0;
  return liveEntries.value.length;
});

// Total visible count
export const displayCount = computed(() => displayEntries.value.length);

// =============================================================================
// INTERNAL HELPERS
// =============================================================================

// Throttled UI update
function scheduleUIUpdate() {
  if (_updateScheduled) return;
  _updateScheduled = true;

  setTimeout(() => {
    _updateScheduled = false;
    batch(() => {
      liveEntries.value = _liveEntries.slice();
      historicalEntries.value = _historicalEntries.slice();
    });
  }, AUDIT_UI_UPDATE_INTERVAL_MS);
}

// Force immediate UI update
function flushUIUpdate() {
  _updateScheduled = false;
  batch(() => {
    liveEntries.value = _liveEntries.slice();
    historicalEntries.value = _historicalEntries.slice();
  });
}

// Memory management: trim oldest entries if exceeding limit
function trimEntriesIfNeeded() {
  const totalEntries = _liveEntries.length + _historicalEntries.length;
  if (totalEntries > AUDIT_MAX_ENTRIES) {
    const excess = totalEntries - AUDIT_MAX_ENTRIES;
    // Remove from historical first (oldest)
    if (_historicalEntries.length >= excess) {
      _historicalEntries = _historicalEntries.slice(0, -excess);
    } else {
      const remainingExcess = excess - _historicalEntries.length;
      _historicalEntries = [];
      _liveEntries = _liveEntries.slice(0, -remainingExcess);
    }
  }
}

// =============================================================================
// SSE CONNECTION
// =============================================================================

let _eventSource = null;

export function connectLiveStream() {
  if (_eventSource) {
    _eventSource.close();
  }

  connectionStatus.value = AUDIT_CONNECTION_STATUS.CONNECTING;
  connectionError.value = null;

  _eventSource = api.createAuditStream();

  _eventSource.onopen = () => {
    connectionStatus.value = AUDIT_CONNECTION_STATUS.CONNECTED;
  };

  _eventSource.onmessage = (event) => {
    try {
      const envelope = JSON.parse(event.data);
      const eventType = envelope.type;
      const data = envelope.data || {};

      if (eventType === 'connected') {
        // Initial connection acknowledgment
        if (data.client_ip) {
          currentClientIP.value = data.client_ip;
        }
      } else if (eventType === 'audit_entry') {
        // New audit entry - add to live entries
        const entry = {
          ...data,
          _source: AUDIT_ENTRY_SOURCE.LIVE,
          _receivedAt: Date.now(),
        };

        _liveEntries.unshift(entry);
        trimEntriesIfNeeded();
        scheduleUIUpdate();
      } else if (eventType === 'error') {
        connectionError.value = data.message || 'Unknown error';
      }
    } catch (err) {
      console.error('Failed to parse audit SSE:', err);
    }
  };

  _eventSource.onerror = () => {
    connectionStatus.value = AUDIT_CONNECTION_STATUS.ERROR;
    connectionError.value = 'Connection lost. Reconnecting...';

    // Auto-reconnect after 3 seconds
    setTimeout(() => {
      if (isLiveEnabled.value) {
        connectLiveStream();
      }
    }, 3000);
  };
}

export function disconnectLiveStream() {
  if (_eventSource) {
    _eventSource.close();
    _eventSource = null;
  }
  connectionStatus.value = AUDIT_CONNECTION_STATUS.DISCONNECTED;
}

export function toggleLiveStream() {
  isLiveEnabled.value = !isLiveEnabled.value;

  if (isLiveEnabled.value) {
    connectLiveStream();
  } else {
    disconnectLiveStream();
  }
}

export function toggleLivePause() {
  isLivePaused.value = !isLivePaused.value;

  // When unpausing, flush pending live entries
  if (!isLivePaused.value) {
    flushUIUpdate();
  }
}

// =============================================================================
// HISTORICAL FETCH
// =============================================================================

export async function fetchHistoricalLogs(reset = false) {
  if (historicalLoading.value) return;
  if (!reset && !historicalHasMore.value) return;

  historicalLoading.value = true;

  try {
    const offset = reset ? 0 : historicalOffset.value;
    const params = {
      offset,
      limit: AUDIT_BATCH_SIZE,
    };

    // Apply server-side filters
    if (userFilter.value !== AUDIT_USER_FILTER.ALL) {
      params.filter = userFilter.value;
    }
    if (selectedActions.value.length === 1) {
      // Single action can use server filter
      params.action = selectedActions.value[0];
    }

    const result = await api.getAuditLogs(params);
    const entries = (result.entries || []).map((entry) => ({
      ...entry,
      _source: AUDIT_ENTRY_SOURCE.HISTORICAL,
    }));

    if (reset) {
      _historicalEntries = entries;
      historicalOffset.value = entries.length;
    } else {
      _historicalEntries = [..._historicalEntries, ...entries];
      historicalOffset.value = historicalOffset.value + entries.length;
    }

    historicalHasMore.value = entries.length === AUDIT_BATCH_SIZE;
    flushUIUpdate();
  } catch (err) {
    console.error('Failed to fetch audit logs:', err);
  } finally {
    historicalLoading.value = false;
  }
}

// =============================================================================
// ACTIONS API
// =============================================================================

export async function fetchAvailableActions() {
  actionsLoading.value = true;
  try {
    const result = await api.getAuditActions();
    availableActions.value = result.actions || [];
  } catch (err) {
    console.error('Failed to fetch audit actions:', err);
  } finally {
    actionsLoading.value = false;
  }
}

// =============================================================================
// FILTER ACTIONS
// =============================================================================

export function setUserFilter(filter) {
  userFilter.value = filter;
  // Refetch historical with new filter
  fetchHistoricalLogs(true);
}

export function toggleActionFilter(action) {
  const current = selectedActions.value;
  if (current.includes(action)) {
    selectedActions.value = current.filter((a) => a !== action);
  } else {
    selectedActions.value = [...current, action];
  }

  // Refetch historical with new filter (if single action selected)
  fetchHistoricalLogs(true);
}

export function clearActionFilters() {
  selectedActions.value = [];
  fetchHistoricalLogs(true);
}

// =============================================================================
// CLEANUP
// =============================================================================

export function clearAllEntries() {
  _liveEntries = [];
  _historicalEntries = [];
  historicalOffset.value = 0;
  historicalHasMore.value = true;
  flushUIUpdate();
}

export function cleanup() {
  disconnectLiveStream();
  clearAllEntries();
}
