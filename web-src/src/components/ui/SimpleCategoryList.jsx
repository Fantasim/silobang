import { SimpleCategoryTitle } from './CategoryHeader';

/**
 * SimpleCategoryList - Flat category display with category titles and items.
 * Filters to only show categories with count > 0.
 * Uses render props pattern for flexible item rendering.
 *
 * @param {Object} props
 * @param {Object} props.categories - Category definitions object (TAG_CATEGORIES or PRESET_CATEGORIES)
 * @param {Array} props.categoryOrder - Array of category IDs in display order
 * @param {Object} props.counts - Object mapping category ID → item count
 * @param {Function} props.renderItems - Function(categoryId) => JSX for rendering category items
 * @param {boolean} props.hideIcon - Hide category icons (default: false)
 * @param {string} props.className - Additional CSS classes
 */
export function SimpleCategoryList({
  categories,
  categoryOrder,
  counts,
  renderItems,
  hideIcon = false,
  className = '',
}) {
  const visibleCategories = categoryOrder
    .map((id) => categories[id])
    .filter((category) => (counts[category.id] || 0) > 0);

  return (
    <div class={`tag-simple-layout ${className}`}>
      {visibleCategories.map((category, index) => (
        <div
          key={category.id}
          class={[
            'tag-simple-category',
            index === 0 && 'category-first',
            index === visibleCategories.length - 1 && 'category-last',
          ]
            .filter(Boolean)
            .join(' ')}
        >
          <SimpleCategoryTitle category={category} color={category.color} hideIcon={hideIcon} />
          {renderItems(category.id)}
        </div>
      ))}
    </div>
  );
}
