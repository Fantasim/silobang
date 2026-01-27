import { useState } from 'preact/hooks';
import { Icon } from '@components/ui/Icon';

/**
 * CopyButton - Copy a value to clipboard with visual feedback.
 *
 * @param {Object} props
 * @param {string} props.value - The text to copy
 */
export function CopyButton({ value }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Fallback — silently fail
    }
  };

  return (
    <button
      class={`credential-copy-btn ${copied ? 'credential-copy-btn--copied' : ''}`}
      onClick={handleCopy}
      title={copied ? 'Copied' : 'Copy to clipboard'}
      type="button"
    >
      <Icon name={copied ? 'check' : 'copy'} size={14} />
    </button>
  );
}

/**
 * UserStatusBadge - Display user active/disabled/bootstrap status.
 *
 * @param {Object} props
 * @param {Object} props.user - User object with is_active and is_bootstrap
 */
export function UserStatusBadge({ user }) {
  return (
    <span>
      <span class={`user-status-badge ${user.is_active ? 'user-status-badge--active' : 'user-status-badge--disabled'}`}>
        {user.is_active ? 'Active' : 'Disabled'}
      </span>
      {user.is_bootstrap && (
        <span class="user-status-badge user-status-badge--bootstrap">Admin</span>
      )}
    </span>
  );
}

/**
 * Format byte values into human-readable strings.
 *
 * @param {number} bytes - Byte count
 * @returns {string} Formatted string (e.g., "1.5 MB")
 */
export function formatBytes(bytes) {
  if (!bytes || bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const k = 1024;
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / Math.pow(k, i)).toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
}
