import { SmartTooltip } from './SmartTooltip';

/**
 * Shared stat component for displaying statistics in ModelInfo and LogsPanel.
 *
 * @param {string|number} value - The stat value to display
 * @param {string} label - The stat label
 * @param {string} type - Type of stat: 'error' | 'warning' | 'info' | null (for model info)
 * @param {boolean} isModelInfo - Whether this is used in ModelInfo (affects color logic)
 * @param {Function} onClick - Optional click handler
 * @param {string} tooltipTitle - Optional tooltip title
 * @param {string} tooltipDesc - Optional tooltip description
 * @param {Array} tooltipRows - Optional tooltip stat rows
 */
export function Stat({
  value,
  label,
  type = null,
  isModelInfo = false,
  onClick = null,
  tooltipTitle,
  tooltipDesc,
  tooltipRows = [],
}) {
  // Determine color class based on context
  let colorClass = '';

  // Parse numeric value to check if it's zero
  const numValue = typeof value === 'string' ? parseFloat(value.replace(/[^\d.]/g, '')) : value;
  const isZero = numValue === 0 || isNaN(numValue);

  // If value is 0, always use grey
  if (isZero) {
    colorClass = 'stat-value-zero';
  } else if (isModelInfo) {
    // For ModelInfo: green if not zero
    colorClass = 'stat-value-green';
  } else {
    // For Validation: use type-based colors
    if (type === 'error') {
      colorClass = 'stat-value-error';
    } else if (type === 'warning') {
      colorClass = 'stat-value-warning';
    } else {
      colorClass = 'stat-value-info';
    }
  }

  const handleClick = (e) => {
    if (onClick && !isZero) {
      onClick(e);
    }
  };

  const statContent = (
    <div
      class={`stat ${onClick && !isZero ? 'stat-clickable' : ''}`}
      onClick={handleClick}
    >
      <span class={`stat-value ${colorClass}`}>{value}</span>
      <span class="stat-label">{label}</span>
    </div>
  );

  // Wrap with tooltip if tooltip props are provided
  if (tooltipTitle || tooltipDesc || tooltipRows.length > 0) {
    return (
      <SmartTooltip
        title={tooltipTitle}
        description={tooltipDesc}
        rows={tooltipRows}
        preferredPosition="right"
      >
        {statContent}
      </SmartTooltip>
    );
  }

  return statContent;
}

