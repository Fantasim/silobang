import { Icon } from './Icon';
import { SimpleTooltip } from './SmartTooltip';

/**
 * GridItem - Individual clickable item in a grid (preset/tag chip without remove button).
 * Displays item icon, name, and hover/active states with category color.
 *
 * @param {Object} props
 * @param {Object} props.item - The item data (preset, tag, etc.)
 * @param {Array} props.item.icon - Icon path or name
 * @param {string} props.item.name - Display name
 * @param {string} props.item.description - Tooltip description
 * @param {boolean} props.isActive - Whether item is active/selected
 * @param {Function} props.onToggle - Toggle handler (id) => void
 * @param {string} props.color - Category color (CSS variable or hex)
 * @param {string} props.className - Additional CSS classes
 */
export function GridItem({ item, isActive = false, onToggle, color, className = '' }) {
  // Determine if icon is a name (string) or an SVG path (typically starts with 'M')
  // Presets use icon names (e.g., 'filterError'), tags use SVG paths
  const isPath = item.icon && typeof item.icon === 'string' && item.icon.startsWith('M');
  const iconProps = isPath ? { path: item.icon } : { name: item.icon };

  return (
    <SimpleTooltip text={item.description || item.name} preferredPosition="top">
      <button
        class={`grid-item ${isActive ? 'active' : ''} ${className}`}
        style={color ? { '--cat-color': color } : undefined}
        onClick={() => onToggle(item.id)}
        type="button"
      >
        <span class="grid-item-icon">
          <Icon {...iconProps} size={14} />
        </span>
        <span class="grid-item-name">{item.name}</span>
      </button>
    </SimpleTooltip>
  );
}
