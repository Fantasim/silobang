import { SimpleTooltip } from './SmartTooltip';

/**
 * ToggleGroup - Generic multi-choice/toggle menu component.
 * Displays a row of toggle buttons where one can be active.
 *
 * @param {Object} props
 * @param {Array<Object>} props.options - Array of options
 * @param {string} props.options[].id - Unique identifier
 * @param {string} props.options[].label - Button label
 * @param {string} props.options[].description - Tooltip description
 * @param {string} props.activeId - Currently active option id
 * @param {Function} props.onToggle - Handler for toggling (id) => void
 * @param {string} props.tooltipPosition - Tooltip position (default: 'top')
 * @param {boolean} props.compact - Compact mode for headers (no background, smaller)
 */
export function ToggleGroup({
  options,
  activeId,
  onToggle,
  tooltipPosition = 'top',
  compact = false
}) {
  return (
    <div class={`toggle-group ${compact ? 'toggle-group-compact' : ''}`}>
      {options.map((option) => (
        <SimpleTooltip
          key={option.id}
          text={option.description}
          preferredPosition={tooltipPosition}
        >
          <button
            class={`toggle-button ${activeId === option.id ? 'active' : ''}`}
            onClick={() => onToggle(option.id)}
            type="button"
          >
            {option.label}
          </button>
        </SimpleTooltip>
      ))}
    </div>
  );
}
