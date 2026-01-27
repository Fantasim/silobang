import { Icon } from './Icon';

/**
 * CategoryHeader - Shared accordion header component for categories.
 * Used in both filter and tag panels for expandable categories.
 *
 * @param {Object} props
 * @param {Object} props.category - Category data
 * @param {string[]|Object} props.category.icon - Icon path or name
 * @param {string} props.category.label - Category label
 * @param {number} props.itemCount - Number of items in category
 * @param {boolean} props.isExpanded - Whether category is expanded
 * @param {Function} props.onClick - Click handler
 * @param {string} props.color - Category color (CSS variable or hex)
 * @param {string} props.className - Additional CSS classes
 * @param {boolean} props.hideIcon - Hide the category icon
 */
export function CategoryHeader({
  category,
  itemCount,
  isExpanded = false,
  onClick,
  color,
  className = '',
  hideIcon = false
}) {
  return (
    <button
      class={`category-header ${isExpanded ? 'expanded' : ''} ${className}`}
      style={color ? { '--cat-color': color } : undefined}
      onClick={onClick}
      type="button"
    >
      {!hideIcon && (
        <span class="category-icon">
          <Icon name={category.icon} size={16} />
        </span>
      )}
      <span class="category-label">{category.label}</span>
      <span class="category-count">{itemCount}</span>
      <span class="category-chevron">
        <Icon name="chevronDown" size={14} />
      </span>
    </button>
  );
}

/**
 * SimpleCategoryTitle - Non-clickable category title for simple mode layouts.
 * Shows category icon and label without accordion functionality.
 *
 * @param {Object} props
 * @param {Object} props.category - Category data
 * @param {string[]|Object} props.category.icon - Icon path or name
 * @param {string} props.category.label - Category label
 * @param {string} props.color - Category color (CSS variable or hex)
 * @param {string} props.className - Additional CSS classes
 * @param {boolean} props.hideIcon - Hide the category icon
 */
export function SimpleCategoryTitle({ category, color, className = '', hideIcon = false }) {
  return (
    <div
      class={`simple-category-title ${className}`}
      style={color ? { '--cat-color': color } : undefined}
    >
      {!hideIcon && (
        <span class="category-icon">
          <Icon name={category.icon} size={14} />
        </span>
      )}
      <span class="category-label">{category.label}</span>
    </div>
  );
}
