import { Icon } from './Icon';
import { SimpleTooltip } from './SmartTooltip';

/**
 * ChipButton - Reusable chip button with icon, text, and optional remove button.
 * Used for displaying active filters, assigned tags, etc.
 *
 * @param {Object} props
 * @param {Object} props.item - The item data (preset, tag, etc.)
 * @param {Array} props.item.icon - Icon path or name
 * @param {string} props.item.name - Display name
 * @param {string} props.item.description - Tooltip description
 * @param {Function} props.onRemove - Remove handler (optional)
 * @param {string} props.color - Category color (CSS variable or hex)
 * @param {string} props.className - Additional CSS classes
 */
export function ChipButton({ item, onRemove, color, className = '' }) {
  const hasRemove = typeof onRemove === 'function';

  return (
    <SimpleTooltip text={item.description || item.name} preferredPosition="top">
      <div
        class={`chip-button ${className}`}
        style={color ? { '--chip-color': color } : undefined}
      >
        <span class="chip-button-icon">
          <Icon name={item.icon} path={item.icon} size={12} />
        </span>
        <span class="chip-button-name">{item.name}</span>

        {hasRemove && (
          <button
            class="chip-button-remove"
            onClick={(e) => {
              e.stopPropagation();
              onRemove(item.id);
            }}
            title={`Remove ${item.name}`}
            type="button"
          >
            <Icon name="close" size={10} />
          </button>
        )}
      </div>
    </SimpleTooltip>
  );
}

/**
 * Clear All chip for removing all active items
 */
export function ClearAllChip({ onClick }) {
  return (
    <button
      class="chip-button chip-button-clear-all"
      onClick={onClick}
      title="Clear all"
      type="button"
    >
      Clear All
    </button>
  );
}
