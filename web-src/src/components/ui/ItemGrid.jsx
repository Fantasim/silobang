/**
 * ItemGrid - Shared grid layout component for displaying items in a 2-column grid.
 * Used in both filter and tag panels for preset/tag items.
 *
 * @param {Object} props
 * @param {ReactNode} props.children - Grid item children
 * @param {string} props.color - Category color (CSS variable or hex)
 * @param {string} props.className - Additional CSS classes
 */
export function ItemGrid({ children, color, className = '' }) {
  return (
    <div
      class={`item-grid ${className}`}
      style={color ? { '--cat-color': color } : undefined}
    >
      {children}
    </div>
  );
}
