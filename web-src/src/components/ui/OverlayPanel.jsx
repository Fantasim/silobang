import { h } from 'preact';
import { useRef, useEffect } from 'preact/hooks';
import { Portal } from './Portal';
import { PanelHeader } from './PanelHeader';

/**
 * Reusable overlay panel component for dropdowns and menus.
 * Provides consistent styling, animations, header, and scrollbar behavior.
 *
 * Uses Portal to render at DOM root level, escaping parent stacking contexts
 * and ensuring proper z-index layering regardless of where component is used.
 *
 * @param {Object} props
 * @param {boolean} props.isOpen - Controls visibility of the panel
 * @param {Function} props.onClose - Callback when panel should close (Escape key or click outside)
 * @param {string} props.title - Title text for the header
 * @param {preact.ComponentChildren} props.children - Content to display in the panel body
 * @param {preact.ComponentChildren} props.headerActions - Optional actions/buttons for the header (e.g., Clear All, Reset)
 * @param {Object} props.style - Additional inline styles for positioning
 * @param {string} props.className - Additional CSS classes
 * @param {boolean} props.closeOnClickOutside - Whether to close when clicking outside (default: true)
 * @param {boolean} props.closeOnEscape - Whether to close on Escape key (default: true)
 * @param {Function} props.onEscapeOverride - Custom handler for Escape key (for multi-step flows)
 */
export function OverlayPanel({
  isOpen,
  onClose,
  title,
  children,
  headerActions,
  style = {},
  className = '',
  closeOnClickOutside = true,
  closeOnEscape = true,
  onEscapeOverride,
  onMouseEnter,
  onMouseLeave,
}) {
  const panelRef = useRef(null);

  // Handle click outside
  useEffect(() => {
    if (!isOpen || !closeOnClickOutside) return;

    const handleClickOutside = (e) => {
      if (panelRef.current && !panelRef.current.contains(e.target)) {
        onClose?.();
      }
    };

    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, [isOpen, closeOnClickOutside, onClose]);

  // Handle Escape key
  useEffect(() => {
    if (!isOpen) return;

    const handleKeyDown = (e) => {
      if (e.key === 'Escape') {
        if (onEscapeOverride) {
          onEscapeOverride();
        } else if (closeOnEscape) {
          onClose?.();
        }
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, closeOnEscape, onClose, onEscapeOverride]);

  if (!isOpen) return null;

  return (
    <Portal>
      <div
        ref={panelRef}
        class={`overlay-panel ${className}`}
        style={style}
        onMouseEnter={onMouseEnter}
        onMouseLeave={onMouseLeave}
      >
        {title && <PanelHeader title={title} actions={headerActions} />}
        <div class="overlay-panel-body">
          {children}
        </div>
      </div>
    </Portal>
  );
}

/**
 * Header action button component for consistent styling.
 */
export function OverlayPanelHeaderButton({ onClick, children, title, className = '' }) {
  return (
    <button
      class={`overlay-panel-header-btn ${className}`}
      onClick={onClick}
      title={title}
    >
      {children}
    </button>
  );
}
