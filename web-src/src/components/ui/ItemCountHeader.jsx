import { Icon } from './Icon';

/**
 * View mode constants
 */
export const VIEW_MODE = {
  LIST: 'list',
  GRID: 'grid',
};

/**
 * Reusable header component for displaying item count and view mode toggle.
 * Used in both Animation and Texture sections.
 */
export function ItemCountHeader({ count, label, viewMode, onViewModeChange }) {
  return (
    <div class="item-count-header">
      <span class="item-count">
        {count} {label}
      </span>
      <div class="view-toggle">
        <button
          class={`view-btn ${viewMode === VIEW_MODE.LIST ? 'active' : ''}`}
          onClick={() => onViewModeChange(VIEW_MODE.LIST)}
          title="List view"
        >
          <Icon name="viewList" size={16} />
        </button>
        <button
          class={`view-btn ${viewMode === VIEW_MODE.GRID ? 'active' : ''}`}
          onClick={() => onViewModeChange(VIEW_MODE.GRID)}
          title="Grid view"
        >
          <Icon name="viewGrid" size={16} />
        </button>
      </div>
    </div>
  );
}
