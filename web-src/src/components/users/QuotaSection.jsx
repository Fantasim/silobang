import { AUTH_ACTION_LABELS } from '@constants/auth';
import { formatBytes } from './helpers';

/**
 * QuotaSection - Displays daily quota usage for a user.
 *
 * @param {Object} props
 * @param {Array} props.quotas - Array of quota usage objects
 */
export function QuotaSection({ quotas }) {
  if (!quotas || quotas.length === 0) {
    return (
      <div class="quota-section-empty">
        <p style={{ color: 'var(--text-dim)', fontSize: 'var(--font-sm)' }}>
          No usage recorded today.
        </p>
      </div>
    );
  }

  return (
    <div class="quota-section">
      <table class="quota-table">
        <thead>
          <tr>
            <th>Action</th>
            <th>Date</th>
            <th>Requests</th>
            <th>Bytes</th>
          </tr>
        </thead>
        <tbody>
          {quotas.map((q, i) => (
            <tr key={i}>
              <td>
                <span class="grant-action-badge">
                  {AUTH_ACTION_LABELS[q.action] || q.action}
                </span>
              </td>
              <td style={{ fontSize: 'var(--font-sm)', color: 'var(--text-secondary)' }}>
                {q.usage_date}
              </td>
              <td>{q.request_count}</td>
              <td>{formatBytes(q.total_bytes)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
