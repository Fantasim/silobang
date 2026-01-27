import { useEffect } from 'preact/hooks';
import { Spinner } from '@components/ui/Spinner';
import { ErrorBanner } from '@components/ui/ErrorBanner';
import { Button } from '@components/ui/Button';
import {
  MONITORING_LOG_LEVEL_LABELS,
  MONITORING_LOG_LEVELS,
} from '@constants/monitoring';
import {
  monitoringData,
  monitoringLoading,
  monitoringError,
  fetchMonitoring,
  startAutoRefresh,
  cleanupMonitoring,
} from '@store/monitoring';
import { api } from '@services/api';
import { formatBytes, formatUptime, formatDateTime } from '../utils/format';
import { canManageConfig } from '@store/auth';
import { Icon } from '@components/ui/Icon';

// =============================================================================
// Sub-components
// =============================================================================

function InfoCard({ title, value, variant, icon }) {
  const valueClass = variant
    ? `service-info-card-value service-info-card-value--${variant}`
    : 'service-info-card-value';
  return (
    <div class="service-info-card">
      <div class="service-info-card-header">
        {icon && <Icon name={icon} size={14} color="var(--text-dim)" />}
        <span class="service-info-card-title">{title}</span>
      </div>
      <div class={valueClass}>{value}</div>
    </div>
  );
}

function LogFileRow({ file, level, onClick }) {
  return (
    <div class="monitoring-log-row" onClick={onClick}>
      <span class="monitoring-log-row-name">{file.name}</span>
      <span class="monitoring-log-row-size">{formatBytes(file.size)}</span>
      <span class="monitoring-log-row-date">{formatDateTime(file.mod_time)}</span>
    </div>
  );
}

function LogLevelSection({ levelInfo }) {
  const { level, file_count, total_size, files } = levelInfo;
  const label = MONITORING_LOG_LEVEL_LABELS[level] || level;
  const levelClass = `monitoring-log-level monitoring-log-level--${level}`;
  const nameClass = `monitoring-log-level-name monitoring-log-level-name--${level}`;

  return (
    <div class={levelClass}>
      <div class="monitoring-log-level-header">
        <span class={nameClass}>{label}</span>
        <span class="monitoring-log-level-meta">
          {file_count} file{file_count !== 1 ? 's' : ''} — {formatBytes(total_size)}
        </span>
      </div>
      {files && files.length > 0 && (
        <div class="monitoring-log-rows">
          {files.map((file) => (
            <LogFileRow
              key={file.name}
              file={file}
              level={level}
              onClick={() => api.viewLogFile(level, file.name)}
            />
          ))}
        </div>
      )}
      {(!files || files.length === 0) && file_count === 0 && (
        <div class="monitoring-log-empty">No log files</div>
      )}
    </div>
  );
}

// =============================================================================
// Main Page
// =============================================================================

export function MonitoringPage() {
  if (!canManageConfig.value) {
    return (
      <div class="permission-denied">
        <Icon name="shieldOff" size={48} />
        <p class="permission-denied-message">You do not have permission to view monitoring.</p>
      </div>
    );
  }

  useEffect(() => {
    fetchMonitoring();
    startAutoRefresh();
    return () => cleanupMonitoring();
  }, []);

  const data = monitoringData.value;
  const loading = monitoringLoading.value;
  const error = monitoringError.value;

  if (loading && !data) {
    return (
      <div class="empty-state">
        <Spinner />
      </div>
    );
  }

  if (error && !data) {
    return <ErrorBanner message={error} />;
  }

  if (!data) return null;

  const { system, application, logs } = data;

  // Only show warn/error log levels
  const visibleLevels = logs.levels.filter((l) =>
    MONITORING_LOG_LEVELS.includes(l.level)
  );

  return (
    <div>
      <div class="page-header">
        <h1 class="page-title">Monitoring</h1>
        <Button variant="ghost" onClick={fetchMonitoring} disabled={loading}>
          {loading ? 'Refreshing...' : 'Refresh'}
        </Button>
      </div>

      {error && <ErrorBanner message={error} />}

      {/* System & Application — single compact row */}
      <section class="monitoring-section">
        <h2 class="monitoring-section-title">
          <Icon name="server" size={16} color="var(--terminal-green)" />
          System
        </h2>
        <div class="monitoring-metrics-grid">
          <InfoCard icon="cpu" title="RAM" value={`${formatBytes(system.ram_used_bytes)} / ${formatBytes(system.ram_total_bytes)}`} variant="size" />
          <InfoCard icon="hardDrive" title="Storage" value={formatBytes(system.project_dir_size_bytes)} variant="size" />
          <InfoCard icon="clock" title="Uptime" value={formatUptime(application.uptime_seconds)} />
          <InfoCard icon="hardDrive" title="Max DAT" value={formatBytes(application.max_dat_size_bytes)} variant="size" />
          <InfoCard icon="fileText" title="Max Meta" value={formatBytes(application.max_metadata_value_bytes)} variant="size" />
        </div>
      </section>

      {/* Log Files */}
      <section class="monitoring-section">
        <h2 class="monitoring-section-title">
          <Icon name="fileText" size={16} color="var(--terminal-green)" />
          Logs
        </h2>
        <div class="monitoring-log-grid">
          {visibleLevels.map((levelInfo) => (
            <LogLevelSection key={levelInfo.level} levelInfo={levelInfo} />
          ))}
        </div>
      </section>
    </div>
  );
}
