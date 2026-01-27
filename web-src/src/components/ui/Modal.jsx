import { useEffect } from 'preact/hooks';
import { Portal } from './Portal';
import { Icon } from './Icon';

/**
 * Reusable modal component.
 *
 * @param {Object} props
 * @param {boolean} props.isOpen - Whether the modal is open
 * @param {Function} [props.onClose] - Callback when modal should close (optional for blocking modals)
 * @param {string} props.title - Modal title
 * @param {string} [props.subtitle] - Optional subtitle below title
 * @param {preact.ComponentChildren} [props.headerActions] - Optional actions in header (buttons, etc.)
 * @param {preact.ComponentChildren} props.children - Modal body content
 * @param {string} [props.className] - Additional CSS class for modal content
 * @param {boolean} [props.showCloseButton=true] - Whether to show the close button
 * @param {boolean} [props.closeOnOverlayClick=true] - Whether clicking overlay closes modal
 */
export function Modal({
  isOpen,
  onClose,
  title,
  subtitle,
  headerActions,
  children,
  className = '',
  showCloseButton = true,
  closeOnOverlayClick = true,
}) {
  // Handle ESC key to close
  useEffect(() => {
    if (!isOpen || !onClose) return;

    const handleKeyDown = (e) => {
      if (e.key === 'Escape') {
        onClose();
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, onClose]);

  if (!isOpen) return null;

  const handleOverlayClick = () => {
    if (closeOnOverlayClick && onClose) {
      onClose();
    }
  };

  return (
    <Portal>
      <div class={`cp-modal active`} onClick={handleOverlayClick}>
        <div
          class={`cp-modal-content ${className}`}
          onClick={(e) => e.stopPropagation()}
        >
          <div class="cp-modal-header">
            <div>
              <span class="cp-modal-title">{title}</span>
              {subtitle && <div class="cp-modal-subtitle">{subtitle}</div>}
            </div>
            <div class="cp-modal-header-actions">
              {headerActions}
              {showCloseButton && onClose && (
                <button class="cp-modal-close" onClick={onClose} title="Close (Esc)">
                  <Icon name="close" size={16} />
                </button>
              )}
            </div>
          </div>
          <div class="cp-modal-body">{children}</div>
        </div>
      </div>
    </Portal>
  );
}
