import { Icon } from './Icon';

/**
 * ErrorBanner component - Displays error messages with optional dismiss button.
 * Reusable component for showing validation errors, import failures, etc.
 *
 * @param {Object} props
 * @param {string} props.message - Error message to display
 * @param {function} [props.onDismiss] - Optional callback to dismiss the error
 */
export function ErrorBanner({ message, onDismiss }) {
  if (!message) return null;

  return (
    <div class="error-banner">
      <Icon name="alertCircle" size={14} />
      <span>{message}</span>
      {onDismiss && (
        <button
          class="error-banner-close"
          onClick={onDismiss}
          title="Dismiss"
        >
          <Icon name="close" size={12} />
        </button>
      )}
    </div>
  );
}
