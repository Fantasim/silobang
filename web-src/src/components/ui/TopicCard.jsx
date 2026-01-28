import { formatBytes, formatRelativeTime, formatPercent, formatDateTime } from '../../utils/format';
import { DetailTooltip, DetailRow, DetailSection } from './DetailTooltip';
import { SimpleTooltip } from './SmartTooltip';

/**
 * Parse extension breakdown JSON string into array of {ext, count, total_size}.
 * Returns empty array on parse failure.
 */
function parseExtensionBreakdown(raw) {
  if (!raw || typeof raw !== 'string') return [];
  try {
    return JSON.parse(raw);
  } catch {
    return [];
  }
}

/**
 * Parse recent DAT files value into array of {name, size}.
 * The backend returns this as a JSON string or array.
 */
function parseDatFiles(raw) {
  if (!raw) return [];
  if (Array.isArray(raw)) return raw;
  if (typeof raw === 'string') {
    try {
      return JSON.parse(raw);
    } catch {
      return [];
    }
  }
  return [];
}

export function TopicCard({ topic, onClick, totalIndexedHashes, totalStorageBytes }) {
  const stats = topic.stats || {};
  const fileCount = stats.file_count || 0;
  const totalSize = (stats.dat_size || 0) + (stats.db_size || 0);
  const hasLastAdded = stats.last_added != null;

  // Share percentages
  const fileShare = totalIndexedHashes > 0 ? formatPercent(fileCount, totalIndexedHashes) : null;
  const sizeShare = totalStorageBytes > 0 ? formatPercent(totalSize, totalStorageBytes) : null;

  // Extension breakdown for file count tooltip
  const extensions = parseExtensionBreakdown(stats.extension_breakdown);
  const avgSize = fileCount > 0 ? (stats.total_size || 0) / fileCount : 0;
  const avgMetaKeys = stats.avg_metadata_keys;

  // DAT files for size tooltip
  const recentDats = parseDatFiles(stats.recent_dat_files);
  const datFileCount = stats.dat_file_count || 0;

  return (
    <div
      class={`topic-card ${topic.healthy ? '' : 'unhealthy'}`}
      onClick={onClick}
    >
      {/* Header: topic name + health indicator */}
      <div class="topic-card-header">
        <h3 class="topic-card-name">{topic.name}</h3>
        <span class={`topic-card-status ${topic.healthy ? 'healthy' : 'unhealthy'}`} />
      </div>

      {topic.healthy ? (
        <>
          {/* Metrics row */}
          <div class="topic-card-metrics">
            {/* File count with detail tooltip */}
            <div class="topic-card-metric">
              <DetailTooltip
                title="File Breakdown"
                preferredPosition="bottom"
                trigger={
                  <span class="topic-card-metric-value">{fileCount.toLocaleString()} files</span>
                }
              >
                {extensions.length > 0 && (
                  <DetailSection label="Extensions" />
                )}
                {extensions.map((ext) => (
                  <DetailRow
                    key={ext.ext}
                    label={ext.ext || '(none)'}
                    value={`${ext.count.toLocaleString()} — ${formatBytes(ext.total_size)}`}
                  />
                ))}
                {extensions.length > 0 && <DetailSection />}
                <DetailRow label="Avg Size" value={formatBytes(avgSize)} />
                {avgMetaKeys != null && (
                  <DetailRow label="Avg Meta Keys" value={Number(avgMetaKeys).toFixed(1)} />
                )}
                <DetailRow label="Versioned" value={(stats.versioned_count || 0).toLocaleString()} />
              </DetailTooltip>
              {fileShare && (
                <span class="topic-card-metric-share">{fileShare} of files</span>
              )}
            </div>

            {/* Total size with detail tooltip */}
            <div class="topic-card-metric">
              <DetailTooltip
                title="Size Breakdown"
                preferredPosition="bottom"
                trigger={
                  <span class="topic-card-metric-value topic-card-metric-value--size">{formatBytes(totalSize)}</span>
                }
              >
                <DetailRow label="Asset Data" value={formatBytes(stats.total_size)} />
                <DetailRow label="DB Size" value={formatBytes(stats.db_size)} />
                <DetailRow label="DAT Containers" value={datFileCount.toString()} />
                {recentDats.length > 0 && <DetailSection label="Recent DATs" />}
                {recentDats.map((dat) => (
                  <DetailRow
                    key={dat.name}
                    label={dat.name}
                    value={formatBytes(dat.size)}
                  />
                ))}
              </DetailTooltip>
              {sizeShare && (
                <span class="topic-card-metric-share">{sizeShare} of storage</span>
              )}
            </div>
          </div>

          {/* Footer: last added timestamp */}
          {hasLastAdded && (
            <div class="topic-card-footer">
              <SimpleTooltip text={formatDateTime(stats.last_added)} preferredPosition="top">
                <span class="topic-card-last-added">
                  Last added: {formatRelativeTime(stats.last_added)}
                </span>
              </SimpleTooltip>
            </div>
          )}
        </>
      ) : (
        <div class="topic-card-error">
          {topic.error}
        </div>
      )}
    </div>
  );
}
