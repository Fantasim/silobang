import { CategoryHeader } from './CategoryHeader';

/**
 * AccordionCategoryList - Expandable category display with CategoryHeader and collapsible items.
 * Supports hiding empty categories and uses render props pattern for flexible item rendering.
 *
 * @param {Object} props
 * @param {Object} props.categories - Category definitions object (TAG_CATEGORIES or PRESET_CATEGORIES)
 * @param {Array} props.categoryOrder - Array of category IDs in display order
 * @param {Object} props.counts - Object mapping category ID → item count
 * @param {string} props.expandedCategoryId - Currently expanded category ID
 * @param {Function} props.onCategoryClick - Function(categoryId) to handle expand/collapse
 * @param {Function} props.renderItems - Function(categoryId) => JSX for rendering category items
 * @param {boolean} props.hideIfEmpty - If true, hide categories with count === 0 (default: false)
 * @param {boolean} props.hideIcon - Hide category icons (default: false)
 * @param {string} props.categoryIdPrefix - Prefix for expandedCategoryId comparison (default: '')
 * @param {string} props.className - Additional CSS classes
 */
export function AccordionCategoryList({
  categories,
  categoryOrder,
  counts,
  expandedCategoryId,
  onCategoryClick,
  renderItems,
  hideIfEmpty = false,
  hideIcon = false,
  categoryIdPrefix = '',
  className = '',
}) {
  return (
    <div class={`tag-category-list ${className}`}>
      {categoryOrder.map((categoryId) => {
        const category = categories[categoryId];
        const itemCount = counts[category.id] || 0;

        // Hide category if empty and hideIfEmpty is true
        if (hideIfEmpty && itemCount === 0) {
          return null;
        }

        const prefixedId = `${categoryIdPrefix}${category.id}`;
        const isExpanded = expandedCategoryId === prefixedId;

        return (
          <div key={category.id} class="tag-category-section">
            <CategoryHeader
              category={category}
              itemCount={itemCount}
              isExpanded={isExpanded}
              onClick={() => onCategoryClick(prefixedId)}
              color={category.color}
              hideIcon={hideIcon}
            />
            {isExpanded && itemCount > 0 && renderItems(category.id)}
          </div>
        );
      })}
    </div>
  );
}
