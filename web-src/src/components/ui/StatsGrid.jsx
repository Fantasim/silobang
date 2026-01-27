/**
 * StatsGrid - CSS Grid layout for 2-3 column stat displays.
 * Replaces 3 identical stat grid patterns (info-stats-grid, logs-stats-grid, texture-stats-grid).
 *
 * @param {Object} props
 * @param {2|3} props.columns - Number of columns (default: 2)
 * @param {number} props.gap - Gap size referencing --space-X tokens (default: 2 for --space-2)
 * @param {string} props.className - Additional CSS classes
 * @param {*} props.children - Stat components or other grid items
 */
export function StatsGrid({ columns = 2, gap = 3, className = '', children }) {
  const styles = {
    display: 'grid',
    gridTemplateColumns: `repeat(${columns}, 1fr)`,
    gap: `var(--space-${gap})`,
  };

  return (
    <div class={`stats-grid ${className}`} style={styles}>
      {children}
    </div>
  );
}
