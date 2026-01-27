import { useState, useRef, useEffect } from 'preact/hooks';
import { ChevronDown, X } from 'lucide-preact';
import { AUDIT_ACTION_COLORS } from '../../store/audit';
import { AUDIT_FILTER_LABELS } from '@constants/audit';

export function AuditActionFilter({ actions, selected, onToggle, onClear, onSelectAll }) {
  const [isOpen, setIsOpen] = useState(false);
  const dropdownRef = useRef(null);

  // Close dropdown when clicking outside
  useEffect(() => {
    const handleClickOutside = (e) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target)) {
        setIsOpen(false);
      }
    };

    if (isOpen) {
      document.addEventListener('mousedown', handleClickOutside);
      return () => document.removeEventListener('mousedown', handleClickOutside);
    }
  }, [isOpen]);

  const selectedCount = selected.length;

  // Format action name for display
  const formatActionName = (action) => {
    return action
      .split('_')
      .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
      .join(' ');
  };

  return (
    <div class="audit-action-filter" ref={dropdownRef}>
      {/* Trigger button */}
      <button
        class={`audit-action-filter-trigger ${isOpen ? 'open' : ''}`}
        onClick={() => setIsOpen(!isOpen)}
        type="button"
      >
        <span>{selectedCount === 0 ? 'All actions' : `${selectedCount} selected`}</span>
        <ChevronDown size={14} />
      </button>

      {/* Selected chips */}
      {selectedCount > 0 && (
        <div class="audit-action-filter-chips">
          {selected.slice(0, 3).map((action) => (
            <span
              key={action}
              class="audit-action-chip"
              style={{ borderColor: AUDIT_ACTION_COLORS[action] || AUDIT_ACTION_COLORS.default }}
            >
              {formatActionName(action)}
              <button
                type="button"
                onClick={(e) => {
                  e.stopPropagation();
                  onToggle(action);
                }}
              >
                <X size={10} />
              </button>
            </span>
          ))}
          {selectedCount > 3 && (
            <span class="audit-action-chip audit-action-chip-more">+{selectedCount - 3}</span>
          )}
          <button class="audit-action-clear" onClick={onClear} type="button" title="Clear all filters">
            Clear
          </button>
        </div>
      )}

      {/* Dropdown menu */}
      {isOpen && (
        <div class="audit-action-filter-dropdown">
          <div class="audit-action-filter-toggle-all">
            <button
              type="button"
              class="audit-action-filter-toggle-btn"
              onClick={() => {
                if (selected.length === actions.length) {
                  onClear();
                } else {
                  onSelectAll();
                }
              }}
            >
              {selected.length === actions.length
                ? AUDIT_FILTER_LABELS.DESELECT_ALL
                : AUDIT_FILTER_LABELS.SELECT_ALL}
            </button>
          </div>
          {actions.map((action) => (
            <label key={action} class="audit-action-filter-option">
              <input
                type="checkbox"
                checked={selected.includes(action)}
                onChange={() => onToggle(action)}
              />
              <span
                class="audit-action-filter-option-dot"
                style={{
                  background: AUDIT_ACTION_COLORS[action] || AUDIT_ACTION_COLORS.default,
                }}
              />
              <span class="audit-action-filter-option-name">{formatActionName(action)}</span>
            </label>
          ))}
        </div>
      )}
    </div>
  );
}
