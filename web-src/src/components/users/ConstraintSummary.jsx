import { AUTH_CONSTRAINT_FIELDS } from '@constants/auth';
import { Icon } from '@components/ui/Icon';

/**
 * ConstraintSummary - Read-only display of grant constraints in human-readable form.
 * Replaces raw JSON display in the grants table.
 *
 * @param {Object} props
 * @param {string} props.action - The auth action
 * @param {string|null} props.constraintsJson - Raw JSON string from the API
 */
export function ConstraintSummary({ action, constraintsJson }) {
  const hasConfigurableFields = (AUTH_CONSTRAINT_FIELDS[action]?.length || 0) > 0;

  if (!constraintsJson) {
    // Don't show "No constraints" for actions that have no configurable fields
    if (!hasConfigurableFields) return null;
    return <span class="grant-constraints-none">No constraints</span>;
  }

  let parsed;
  try {
    parsed = JSON.parse(constraintsJson);
  } catch {
    // Invalid JSON — show raw with warning
    return (
      <span class="constraint-summary-error">
        <Icon name="alertTriangle" size={12} />
        <span class="constraint-summary-error-text" title={constraintsJson}>
          Invalid JSON
        </span>
      </span>
    );
  }

  const fields = AUTH_CONSTRAINT_FIELDS[action] || [];
  const knownKeys = new Set(fields.map(f => f.key));
  const parsedKeys = Object.keys(parsed);

  // Check for unknown keys not in the schema
  const unknownKeys = parsedKeys.filter(k => !knownKeys.has(k));

  // Build human-readable entries from known fields
  const entries = fields
    .filter(f => parsed[f.key] !== undefined && parsed[f.key] !== null)
    .map(f => {
      const val = parsed[f.key];
      if (f.type === 'boolean') {
        return { label: f.label, value: val ? 'Yes' : 'No', type: 'boolean', boolVal: val };
      }
      if (f.type === 'bytes') {
        return { label: f.label, value: formatBytes(val), type: 'number' };
      }
      if (f.type === 'number') {
        return { label: f.label, value: formatConstraintNumber(f.key, val), type: 'number' };
      }
      if (f.type === 'string_array' && Array.isArray(val)) {
        return { label: f.label, value: val.join(', '), type: 'array' };
      }
      return { label: f.label, value: String(val), type: 'unknown' };
    });

  if (entries.length === 0 && unknownKeys.length === 0) {
    return <span class="grant-constraints-none">No constraints</span>;
  }

  return (
    <div class="constraint-summary">
      {entries.map((entry, i) => (
        <span key={i} class="constraint-summary-item">
          <span class="constraint-summary-label">{entry.label}:</span>
          <span class={`constraint-summary-value ${entry.type === 'boolean' ? (entry.boolVal ? 'constraint-summary-value--yes' : 'constraint-summary-value--no') : ''}`}>
            {entry.value}
          </span>
        </span>
      ))}
      {unknownKeys.length > 0 && (
        <span class="constraint-summary-item constraint-summary-item--custom">
          <Icon name="info" size={11} />
          <span class="constraint-summary-label">
            +{unknownKeys.length} custom
          </span>
        </span>
      )}
    </div>
  );
}

/**
 * Format constraint numbers with appropriate units.
 */
function formatConstraintNumber(key, value) {
  if (key.endsWith('_bytes') && value > 0) {
    return formatBytes(value);
  }
  return value.toLocaleString();
}

function formatBytes(bytes) {
  if (!bytes || bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const k = 1024;
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / Math.pow(k, i)).toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
}
