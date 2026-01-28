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
import { formatBytes, formatUptime, formatDateTime, formatPercent, getStorageThreshold } from '../utils/format';
import { canManageConfig } from '@store/auth';
import { Icon } from '@components/ui/Icon';
import { DetailTooltip, DetailRow } from '@components/ui/DetailTooltip';

// =============================================================================
// Sub-components
// =============================================================================

function InfoCard({ title, value, variant, icon, subtitle, subtitleColor }) {
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
      {subtitle && (
        <div class="service-info-card-subtitle" style={subtitleColor ? { color: subtitleColor } : undefined}>
          {subtitle}
        </div>
      )}
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

  const { system, application, logs, service } = data;

  // Only show warn/error log levels
  const visibleLevels = logs.levels.filter((l) =>
    MONITORING_LOG_LEVELS.includes(l.level)
  );

  // Storage threshold for color coding
  const hasMaxDisk = application.max_disk_usage_bytes > 0;
  const storageThreshold = hasMaxDisk
    ? getStorageThreshold(system.project_dir_size_bytes, application.max_disk_usage_bytes)
    : { cssClass: 'storage-ok', percent: 0 };
  const storageValue = hasMaxDisk
    ? `${formatBytes(system.project_dir_size_bytes)} / ${formatBytes(application.max_disk_usage_bytes)} (${formatPercent(system.project_dir_size_bytes, application.max_disk_usage_bytes)})`
    : formatBytes(system.project_dir_size_bytes);

  // Service-level totals from cached stats
  const topicsSummary = service?.topics_summary;
  const storageSummary = service?.storage_summary;

  return (
    <div>
      <div class="page-header">
        <h1 class="page-title">Monitoring</h1>
        <Button variant="ghost" onClick={fetchMonitoring} disabled={loading}>
          {loading ? 'Refreshing...' : 'Refresh'}
        </Button>
      </div>

      {error && <ErrorBanner message={error} />}

      {/* Service Overview — stats from cache */}
      {service && (
        <section class="monitoring-section">
          <h2 class="monitoring-section-title">
            <Icon name="layers" size={16} color="var(--terminal-green)" />
            Service Overview
          </h2>
          <div class="monitoring-metrics-grid">
            <InfoCard
              icon="box"
              title="Topics"
              value={topicsSummary?.total || 0}
              variant="count"
              subtitle={topicsSummary?.unhealthy > 0 ? `${topicsSummary.unhealthy} unhealthy` : 'All healthy'}
              subtitleColor={topicsSummary?.unhealthy > 0 ? 'var(--terminal-red)' : 'var(--terminal-green)'}
            />
            <InfoCard
              icon="fileText"
              title="Total DAT"
              value={storageSummary?.total_dat_files || 0}
              variant="count"
            />
            <InfoCard
              icon="layers"
              title="Indexed Assets"
              value={(service.total_indexed_hashes || 0).toLocaleString()}
              variant="count"
            />
            <DetailTooltip
              title="Storage Breakdown"
              preferredPosition="bottom"
              trigger={
                <InfoCard
                  icon="hardDrive"
                  title="Total Storage"
                  value={storageValue}
                  variant={storageThreshold.cssClass}
                />
              }
            >
              <DetailRow label="DAT Files" value={formatBytes(storageSummary?.total_dat_size)} />
              <DetailRow label="Databases" value={formatBytes(storageSummary?.total_db_size)} />
              <DetailRow label="Orchestrator DB" value={formatBytes(service.orchestrator_db_size)} />
            </DetailTooltip>
          </div>
        </section>
      )}

      {/* System Resources */}
      <section class="monitoring-section">
        <h2 class="monitoring-section-title">
          <Icon name="server" size={16} color="var(--terminal-green)" />
          System
        </h2>
        <div class="monitoring-metrics-grid">
          <InfoCard icon="cpu" title="RAM" value={`${formatBytes(system.ram_used_bytes)} / ${formatBytes(system.ram_total_bytes)}`} variant="size" />
          <InfoCard icon="hardDrive" title="Working Dir" value={storageValue} variant={storageThreshold.cssClass}
            subtitle={hasMaxDisk ? undefined : 'No limit set'}
            subtitleColor="var(--text-dim)"
          />
          <InfoCard icon="clock" title="Uptime" value={formatUptime(application.uptime_seconds)}
            subtitle={`Started ${formatDateTime(application.started_at)}`}
          />
        </div>
      </section>

      {/* Application Configuration */}
      <section class="monitoring-section">
        <h2 class="monitoring-section-title">
          <Icon name="fileText" size={16} color="var(--terminal-green)" />
          Configuration
        </h2>
        <div class="monitoring-config-grid">
          <div class="monitoring-config-row">
            <span class="monitoring-config-label">Working Directory</span>
            <span class="monitoring-config-value">{application.working_directory || '-'}</span>
          </div>
          <div class="monitoring-config-row">
            <span class="monitoring-config-label">Port</span>
            <span class="monitoring-config-value">{application.port}</span>
          </div>
          <div class="monitoring-config-row">
            <span class="monitoring-config-label">Max DAT Size</span>
            <span class="monitoring-config-value">{formatBytes(application.max_dat_size_bytes)}</span>
          </div>
          <div class="monitoring-config-row">
            <span class="monitoring-config-label">Max Meta Value</span>
            <span class="monitoring-config-value">{formatBytes(application.max_metadata_value_bytes)}</span>
          </div>
          <div class="monitoring-config-row">
            <span class="monitoring-config-label">Max Disk Usage</span>
            <span class="monitoring-config-value">{hasMaxDisk ? formatBytes(application.max_disk_usage_bytes) : 'Unlimited'}</span>
          </div>
          {service?.version_info && (
            <>
              <div class="monitoring-config-row">
                <span class="monitoring-config-label">App Version</span>
                <span class="monitoring-config-value">{service.version_info.app_version}</span>
              </div>
              <div class="monitoring-config-row">
                <span class="monitoring-config-label">Blob Version</span>
                <span class="monitoring-config-value">{service.version_info.blob_version}</span>
              </div>
              <div class="monitoring-config-row">
                <span class="monitoring-config-label">Header Size</span>
                <span class="monitoring-config-value">{service.version_info.header_size} bytes</span>
              </div>
            </>
          )}
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
