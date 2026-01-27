/**
 * Reusable panel header component.
 *
 * @param {string} title - The panel title
 * @param {object} extraContent - Optional extra content (e.g., counter, size, chips) placed before actions
 * @param {object} actions - Optional action buttons displayed on the right side
 */
export function PanelHeader({ title, extraContent, actions }) {
  return (
    <div class="panel-header">
      <span class="panel-header-title">{title}</span>
      <div class="panel-header-right">
        {extraContent && <span class="panel-header-extra">{extraContent}</span>}
        {actions && <div class="panel-header-actions">{actions}</div>}
      </div>
    </div>
  );
}
