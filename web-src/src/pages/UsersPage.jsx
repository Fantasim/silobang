import { useState, useEffect, useCallback } from 'preact/hooks';
import { Button } from '@components/ui/Button';
import { Modal } from '@components/ui/Modal';
import { ErrorBanner } from '@components/ui/ErrorBanner';
import { Spinner } from '@components/ui/Spinner';
import { Icon } from '@components/ui/Icon';
import { api } from '@services/api';
import { canManageUsers } from '@store/auth';
import { showToast } from '@store/ui';
import {
  AUTH_ERROR_MESSAGES,
  AUTH_PASSWORD_MIN_LENGTH,
  AUTH_PASSWORD_MAX_LENGTH,
  AUTH_USERNAME_PATTERN,
  USERS_EMPTY_STATE_TEXT,
} from '@constants/auth';
import { UserList } from '@components/users/UserList';
import { UserDetail } from '@components/users/UserDetail';
import { CopyButton } from '@components/users/helpers';

// =============================================================================
// USERS PAGE — Master-detail layout orchestrator
// =============================================================================

export function UsersPage({ userId: initialUserId }) {
  const [users, setUsers] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [selectedUserId, setSelectedUserId] = useState(initialUserId || null);
  const [searchQuery, setSearchQuery] = useState('');
  const [createModalOpen, setCreateModalOpen] = useState(false);

  const fetchUsers = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const result = await api.getUsers();
      const data = result.data || result;
      setUsers(data.users || []);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchUsers();
  }, [fetchUsers]);

  // Auto-select from route param
  useEffect(() => {
    if (initialUserId && !selectedUserId) {
      setSelectedUserId(initialUserId);
    }
  }, [initialUserId]);

  if (!canManageUsers.value) {
    return (
      <div class="permission-denied">
        <Icon name="shieldOff" size={48} />
        <p class="permission-denied-message">
          You do not have permission to manage users.
        </p>
      </div>
    );
  }

  return (
    <div class="users-page">
      <div class="users-page-layout">
        {/* Left panel — Master */}
        <div class="users-page-master">
          <div class="users-page-header">
            <h1 class="users-page-title">Users</h1>
            <Button onClick={() => setCreateModalOpen(true)}>
              + Create
            </Button>
          </div>

          {error && <ErrorBanner message={error} onDismiss={() => setError(null)} />}

          {loading ? (
            <div class="users-page-loading">
              <Spinner />
            </div>
          ) : (
            <UserList
              users={users}
              selectedUserId={selectedUserId}
              onSelect={setSelectedUserId}
              searchQuery={searchQuery}
              onSearchChange={setSearchQuery}
            />
          )}
        </div>

        {/* Right panel — Detail */}
        <div class="users-page-detail">
          {selectedUserId ? (
            <UserDetail
              userId={selectedUserId}
              onUserUpdated={fetchUsers}
              onClose={() => setSelectedUserId(null)}
            />
          ) : (
            <div class="users-page-empty-state">
              <Icon name="userPlus" size={40} color="var(--text-dim)" />
              <p class="users-page-empty-text">{USERS_EMPTY_STATE_TEXT}</p>
            </div>
          )}
        </div>
      </div>

      {createModalOpen && (
        <CreateUserModal
          onClose={() => setCreateModalOpen(false)}
          onCreated={() => {
            fetchUsers();
            setCreateModalOpen(false);
          }}
        />
      )}
    </div>
  );
}

// =============================================================================
// CREATE USER MODAL (kept here — only used by UsersPage)
// =============================================================================

function CreateUserModal({ onClose, onCreated }) {
  const [username, setUsername] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const [createdApiKey, setCreatedApiKey] = useState(null);

  const isValid =
    AUTH_USERNAME_PATTERN.test(username) &&
    password.length >= AUTH_PASSWORD_MIN_LENGTH &&
    password.length <= AUTH_PASSWORD_MAX_LENGTH;

  const handleSubmit = async (e) => {
    e.preventDefault();
    if (!isValid) return;

    setLoading(true);
    setError(null);

    try {
      const result = await api.createUser(username, displayName || username, password);
      const data = result.data || result;
      setCreatedApiKey(data.api_key);
      showToast(`User "${username}" created`, 'success');
    } catch (err) {
      const message = AUTH_ERROR_MESSAGES[err.code] || err.message;
      setError(message);
    } finally {
      setLoading(false);
    }
  };

  // After creation, show API key then close
  if (createdApiKey) {
    return (
      <Modal isOpen title="User Created" onClose={onCreated}>
        <div class="api-key-display">
          <p class="api-key-display-warning">
            <Icon name="alertTriangle" size={14} /> Save this API key now. It will NOT be shown again.
          </p>
          <div class="credential-value" style={{ marginTop: 'var(--space-2)' }}>
            <span class="credential-value-text">{createdApiKey}</span>
            <CopyButton value={createdApiKey} />
          </div>
        </div>
        <Button
          onClick={onCreated}
          style={{ width: '100%', marginTop: 'var(--space-4)' }}
        >
          Done
        </Button>
      </Modal>
    );
  }

  return (
    <Modal isOpen title="Create User" onClose={onClose}>
      <form onSubmit={handleSubmit}>
        <label class="login-label">Username</label>
        <input
          type="text"
          class="user-field-input"
          placeholder="lowercase letters, numbers, hyphens, underscores"
          value={username}
          onInput={(e) => setUsername(e.target.value)}
          disabled={loading}
          autoFocus
          style={{ marginBottom: 'var(--space-3)' }}
        />

        <label class="login-label">Display Name</label>
        <input
          type="text"
          class="user-field-input"
          placeholder="Optional display name"
          value={displayName}
          onInput={(e) => setDisplayName(e.target.value)}
          disabled={loading}
          style={{ marginBottom: 'var(--space-3)' }}
        />

        <label class="login-label">Password</label>
        <input
          type="password"
          class="user-field-input"
          placeholder={`Minimum ${AUTH_PASSWORD_MIN_LENGTH} characters`}
          value={password}
          onInput={(e) => setPassword(e.target.value)}
          disabled={loading}
          style={{ marginBottom: 'var(--space-3)' }}
        />

        {error && <ErrorBanner message={error} onDismiss={() => setError(null)} />}

        <Button
          type="submit"
          disabled={loading || !isValid}
          style={{ width: '100%', marginTop: 'var(--space-2)' }}
        >
          {loading ? <Spinner /> : 'Create User'}
        </Button>
      </form>
    </Modal>
  );
}
