import { StatsGrid } from '@components/ui/StatsGrid';
import { formatBytes, formatDateTime } from '../../utils/format';

export function TopicCard({ topic, onClick }) {
  const stats = topic.stats || {};

  const primaryStats = [
    { label: 'Files', value: stats.file_count?.toLocaleString() || '0', type: 'count' },
    { label: 'Total Size', value: formatBytes(stats.total_size), type: 'size' },
    { label: 'DB Size', value: formatBytes(stats.db_size), type: 'size' },
    { label: 'DAT Size', value: formatBytes(stats.dat_size), type: 'size' },
  ];

  const hasLastAdded = stats.last_added != null;

  const getValueClass = (type) => {
    if (type === 'size') return 'topic-stat-value topic-stat-value--size';
    if (type === 'date') return 'topic-stat-value topic-stat-value--date';
    return 'topic-stat-value';
  };

  return (
    <div
      class={`topic-card ${topic.healthy ? '' : 'unhealthy'}`}
      onClick={onClick}
    >
      <div class="topic-card-header">
        <span class="topic-card-name">{topic.name}</span>
        <span class={`topic-card-status ${topic.healthy ? 'healthy' : 'unhealthy'}`} />
      </div>

      {topic.healthy ? (
        <>
          <StatsGrid columns={2} gap={2}>
            {primaryStats.map(stat => (
              <div class="topic-stat" key={stat.label}>
                <span class={getValueClass(stat.type)}>{stat.value}</span>
                <span class="topic-stat-label">{stat.label}</span>
              </div>
            ))}
          </StatsGrid>
          {hasLastAdded && (
            <div class="topic-card-footer">
              <span class="topic-card-last-added">
                Last added: {formatDateTime(stats.last_added)}
              </span>
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
