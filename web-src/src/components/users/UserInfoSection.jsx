import { useState } from 'preact/hooks';
import { Button } from '@components/ui/Button';
import { Spinner } from '@components/ui/Spinner';
import { Icon } from '@components/ui/Icon';
import { api } from '@services/api';
import { showToast } from '@store/ui';
import {
  AUTH_ERROR_MESSAGES,
  AUTH_PASSWORD_MIN_LENGTH,
} from '@constants/auth';
import { CopyButton, UserStatusBadge } from './helpers';

/**
 * UserInfoSection - Displays and manages user account information.
 * Includes display name editing, status toggle, password change, and API key regeneration.
 *
 * @param {Object} props
 * @param {Object} props.user - User object
 * @param {() => void} props.onUserUpdated - Called after any user update
 */
export function UserInfoSection({ user, onUserUpdated }) {
  const [editingName, setEditingName] = useState(false);
  const [newDisplayName, setNewDisplayName] = useState(user.display_name);
  const [changingPassword, setChangingPassword] = useState(false);
  const [newPassword, setNewPassword] = useState('');
  const [actionLoading, setActionLoading] = useState(null);
  const [regeneratedKey, setRegeneratedKey] = useState(null);

  const handleUpdateDisplayName = async () => {
    setActionLoading('name');
    try {
      await api.updateUser(user.id, { display_name: newDisplayName });
      showToast('Display name updated', 'success');
      setEditingName(false);
      onUserUpdated();
    } catch (err) {
      showToast(err.message, 'error');
    } finally {
      setActionLoading(null);
    }
  };

  const handleToggleActive = async () => {
    const action = user.is_active ? 'disable' : 'enable';
    if (!confirm(`Are you sure you want to ${action} user "${user.username}"?`)) return;

    setActionLoading('toggle');
    try {
      await api.updateUser(user.id, { is_active: !user.is_active });
      showToast(`User ${action}d`, 'success');
      onUserUpdated();
    } catch (err) {
      const message = AUTH_ERROR_MESSAGES[err.code] || err.message;
      showToast(message, 'error');
    } finally {
      setActionLoading(null);
    }
  };

  const handleChangePassword = async () => {
    if (newPassword.length < AUTH_PASSWORD_MIN_LENGTH) return;
    setActionLoading('password');
    try {
      await api.updateUser(user.id, { new_password: newPassword });
      showToast('Password changed (all sessions invalidated)', 'success');
      setChangingPassword(false);
      setNewPassword('');
    } catch (err) {
      showToast(err.message, 'error');
    } finally {
      setActionLoading(null);
    }
  };

  const handleRegenerateKey = async () => {
    if (!confirm('This will invalidate the current API key immediately. Continue?')) return;
    setActionLoading('apikey');
    try {
      const result = await api.regenerateAPIKey(user.id);
      const data = result.data || result;
      setRegeneratedKey(data.api_key);
      showToast('API key regenerated', 'success');
    } catch (err) {
      showToast(err.message, 'error');
    } finally {
      setActionLoading(null);
    }
  };

  return (
    <div class="user-info-section">
      <div class="user-field-row">
        <span class="user-field-label">Username</span>
        <span class="user-field-value" style={{ color: 'var(--terminal-cyan)' }}>
          {user.username}
        </span>
      </div>

      <div class="user-field-row">
        <span class="user-field-label">Display Name</span>
        {editingName ? (
          <span style={{ display: 'flex', gap: 'var(--space-2)', flex: 1 }}>
            <input
              class="user-field-input"
              value={newDisplayName}
              onInput={(e) => setNewDisplayName(e.target.value)}
              autoFocus
            />
            <Button onClick={handleUpdateDisplayName} disabled={actionLoading === 'name'}>
              {actionLoading === 'name' ? <Spinner /> : 'Save'}
            </Button>
            <Button onClick={() => setEditingName(false)}>Cancel</Button>
          </span>
        ) : (
          <span style={{ display: 'flex', gap: 'var(--space-2)', flex: 1, alignItems: 'center' }}>
            <span class="user-field-value">{user.display_name}</span>
            <button class="credential-copy-btn" onClick={() => setEditingName(true)} title="Edit">
              <Icon name="pencil" size={12} />
            </button>
          </span>
        )}
      </div>

      <div class="user-field-row">
        <span class="user-field-label">Status</span>
        <span class="user-field-value">
          <UserStatusBadge user={user} />
        </span>
      </div>

      <div class="user-field-row">
        <span class="user-field-label">Created</span>
        <span class="user-field-value" style={{ color: 'var(--text-secondary)', fontSize: 'var(--font-sm)' }}>
          {new Date(user.created_at * 1000).toLocaleString()}
        </span>
      </div>

      <div class="user-actions" style={{ marginTop: 'var(--space-3)' }}>
        {!user.is_bootstrap && (
          <Button
            variant={user.is_active ? 'danger' : 'default'}
            onClick={handleToggleActive}
            disabled={actionLoading === 'toggle'}
          >
            {actionLoading === 'toggle' ? <Spinner /> : (user.is_active ? 'Disable User' : 'Enable User')}
          </Button>
        )}

        <Button onClick={() => setChangingPassword(!changingPassword)}>
          Change Password
        </Button>

        <Button onClick={handleRegenerateKey} disabled={actionLoading === 'apikey'}>
          {actionLoading === 'apikey' ? <Spinner /> : 'Regenerate API Key'}
        </Button>
      </div>

      {changingPassword && (
        <div style={{ marginTop: 'var(--space-3)', display: 'flex', gap: 'var(--space-2)' }}>
          <input
            class="user-field-input"
            type="password"
            placeholder={`New password (min ${AUTH_PASSWORD_MIN_LENGTH} chars)`}
            value={newPassword}
            onInput={(e) => setNewPassword(e.target.value)}
          />
          <Button
            onClick={handleChangePassword}
            disabled={newPassword.length < AUTH_PASSWORD_MIN_LENGTH || actionLoading === 'password'}
          >
            {actionLoading === 'password' ? <Spinner /> : 'Set'}
          </Button>
        </div>
      )}

      {regeneratedKey && (
        <div class="api-key-display">
          <p class="api-key-display-warning">
            <Icon name="alertTriangle" size={14} /> Save this API key now. It will NOT be shown again.
          </p>
          <div class="credential-value" style={{ marginTop: 'var(--space-2)' }}>
            <span class="credential-value-text">{regeneratedKey}</span>
            <CopyButton value={regeneratedKey} />
          </div>
          <Button
            onClick={() => setRegeneratedKey(null)}
            style={{ marginTop: 'var(--space-2)' }}
          >
            Dismiss
          </Button>
        </div>
      )}
    </div>
  );
}
