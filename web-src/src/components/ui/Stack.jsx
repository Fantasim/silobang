/**
 * Stack - Flexbox layout component for vertical/horizontal stacking with consistent spacing.
 * Replaces raw div flex patterns across the codebase.
 *
 * @param {Object} props
 * @param {'vertical'|'horizontal'} props.direction - Flex direction (default: 'vertical')
 * @param {number} props.gap - Gap size referencing --space-X tokens (e.g., 2 = --space-2, 3 = --space-3)
 * @param {'start'|'center'|'end'|'stretch'} props.align - align-items value
 * @param {'start'|'center'|'end'|'space-between'} props.justify - justify-content value
 * @param {string} props.className - Additional CSS classes
 * @param {*} props.children - Content to stack
 */
export function Stack({
  direction = 'vertical',
  gap = 2,
  align,
  justify,
  className = '',
  children,
}) {
  const styles = {
    display: 'flex',
    flexDirection: direction === 'horizontal' ? 'row' : 'column',
    gap: `var(--space-${gap})`,
  };

  if (align) {
    styles.alignItems = align === 'start' || align === 'end' ? `flex-${align}` : align;
  }

  if (justify) {
    styles.justifyContent = justify === 'start' || justify === 'end' ? `flex-${justify}` : justify;
  }

  return (
    <div class={`stack ${className}`} style={styles}>
      {children}
    </div>
  );
}
