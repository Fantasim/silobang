import { useEffect, useRef } from 'preact/hooks';
import { Button, Spinner, AuditEntry, AuditActionFilter, Icon } from '@components/ui';
import { canViewAudit } from '@store/auth';
import {
  connectionStatus,
  isLiveEnabled,
  isLivePaused,
  availableActions,
  userFilter,
  selectedActions,
  displayEntries,
  displayCount,
  pendingLiveCount,
  historicalLoading,
  historicalHasMore,
  connectLiveStream,
  toggleLiveStream,
  toggleLivePause,
  fetchAvailableActions,
  fetchHistoricalLogs,
  setUserFilter,
  toggleActionFilter,
  clearActionFilters,
  selectAllActionFilters,
  cleanup,
  AUDIT_USER_FILTER,
  AUDIT_CONNECTION_STATUS,
} from '../store/audit';

export function AuditLogPage() {
  const scrollContainerRef = useRef(null);
  const lastScrollTopRef = useRef(0);

  if (!canViewAudit.value) {
    return (
      <div class="permission-denied">
        <Icon name="shieldOff" size={48} />
        <p class="permission-denied-message">You do not have permission to view audit logs.</p>
      </div>
    );
  }

  // Initialize on mount
  useEffect(() => {
    fetchAvailableActions();

    if (isLiveEnabled.value) {
      connectLiveStream();
    }

    // Also fetch initial historical logs
    fetchHistoricalLogs(true);

    return () => {
      cleanup();
    };
  }, []);

  // Infinite scroll handler
  const handleScroll = () => {
    const el = scrollContainerRef.current;
    if (!el) return;

    // Only load more when scrolling down and near bottom
    const scrollingDown = el.scrollTop > lastScrollTopRef.current;
    lastScrollTopRef.current = el.scrollTop;

    if (!scrollingDown) return;

    const nearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 200;

    if (nearBottom && historicalHasMore.value && !historicalLoading.value) {
      fetchHistoricalLogs();
    }
  };

  // Connection status indicator
  const renderConnectionStatus = () => {
    const status = connectionStatus.value;
    const statusClass = {
      [AUDIT_CONNECTION_STATUS.CONNECTED]: 'connected',
      [AUDIT_CONNECTION_STATUS.CONNECTING]: 'connecting',
      [AUDIT_CONNECTION_STATUS.DISCONNECTED]: 'disconnected',
      [AUDIT_CONNECTION_STATUS.ERROR]: 'error',
    }[status] || 'disconnected';

    return (
      <span class={`audit-connection-status ${statusClass}`}>
        <span class="audit-connection-dot" />
        <span class="audit-connection-text">
          {status === AUDIT_CONNECTION_STATUS.CONNECTED && 'Live'}
          {status === AUDIT_CONNECTION_STATUS.CONNECTING && 'Connecting...'}
          {status === AUDIT_CONNECTION_STATUS.DISCONNECTED && 'Disconnected'}
          {status === AUDIT_CONNECTION_STATUS.ERROR && 'Reconnecting...'}
        </span>
      </span>
    );
  };

  const entries = displayEntries.value;
  const count = displayCount.value;
  const pending = pendingLiveCount.value;

  return (
    <div class="audit-log-page">
      {/* Header */}
      <div class="page-header">
        <div class="page-header-left">
          <h1 class="page-title">Audit Log</h1>
          {isLiveEnabled.value && renderConnectionStatus()}
        </div>
        <div class="page-header-right">
          {/* Live/Pause Toggle */}
          <Button
            variant={isLiveEnabled.value ? 'default' : 'ghost'}
            onClick={toggleLiveStream}
            title={isLiveEnabled.value ? 'Disconnect live stream' : 'Connect live stream'}
          >
            {isLiveEnabled.value ? 'Live' : 'Connect Live'}
          </Button>

          {isLiveEnabled.value && (
            <Button
              variant={isLivePaused.value ? 'default' : 'ghost'}
              onClick={toggleLivePause}
              title={isLivePaused.value ? 'Resume updates' : 'Pause updates'}
            >
              {isLivePaused.value ? `Resume (${pending} new)` : 'Pause'}
            </Button>
          )}
        </div>
      </div>

      {/* Filters Toolbar */}
      <div class="audit-toolbar">
        {/* User Filter */}
        <div class="audit-filter-section">
          <span class="audit-filter-label">Show:</span>
          <div class="audit-user-filter">
            <button
              type="button"
              class={`audit-filter-btn ${userFilter.value === AUDIT_USER_FILTER.ALL ? 'active' : ''}`}
              onClick={() => setUserFilter(AUDIT_USER_FILTER.ALL)}
            >
              All
            </button>
            <button
              type="button"
              class={`audit-filter-btn ${userFilter.value === AUDIT_USER_FILTER.ME ? 'active' : ''}`}
              onClick={() => setUserFilter(AUDIT_USER_FILTER.ME)}
            >
              Me
            </button>
            <button
              type="button"
              class={`audit-filter-btn ${userFilter.value === AUDIT_USER_FILTER.OTHERS ? 'active' : ''}`}
              onClick={() => setUserFilter(AUDIT_USER_FILTER.OTHERS)}
            >
              Others
            </button>
          </div>
        </div>

        {/* Action Type Filter */}
        <div class="audit-filter-section">
          <span class="audit-filter-label">Actions:</span>
          <AuditActionFilter
            actions={availableActions.value}
            selected={selectedActions.value}
            onToggle={toggleActionFilter}
            onClear={clearActionFilters}
            onSelectAll={selectAllActionFilters}
          />
        </div>

        {/* Count */}
        <div class="audit-count">
          {count.toLocaleString()} event{count !== 1 ? 's' : ''}
        </div>
      </div>

      {/* Entries List */}
      <div
        class="audit-entries-container"
        ref={scrollContainerRef}
        onScroll={handleScroll}
      >
        {entries.length === 0 ? (
          <div class="empty-state">
            <div class="empty-state-icon">📋</div>
            <div class="empty-state-text">No audit events</div>
            <div class="empty-state-hint">
              {isLiveEnabled.value
                ? 'Events will appear here in real-time'
                : 'Try adjusting your filters or connecting to live stream'}
            </div>
          </div>
        ) : (
          <div class="audit-entries-list">
            {entries.map((entry, index) => (
              <AuditEntry
                key={entry.id || `${entry.timestamp}-${index}`}
                entry={entry}
                isNew={entry._source === 'live' && index < 3}
              />
            ))}
          </div>
        )}

        {/* Loading indicator */}
        {historicalLoading.value && (
          <div class="audit-loading">
            <Spinner />
          </div>
        )}

        {/* End of list indicator */}
        {!historicalHasMore.value && entries.length > 0 && (
          <div class="audit-end-marker">End of audit log</div>
        )}
      </div>
    </div>
  );
}
