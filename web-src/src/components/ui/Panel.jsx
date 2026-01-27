/**
 * Panel - Standard panel wrapper with backdrop blur + border styling.
 * Replaces 6 identical panel wrapper patterns (model-info, logs, nav, filter, tag, control).
 *
 * @param {string} className - Additional CSS classes for panel-specific styling
 * @param {*} children - Panel content (typically PanelHeader + content divs)
 */
export function Panel({ className = '', children }) {
  return <div class={`panel ${className}`}>{children}</div>;
}
