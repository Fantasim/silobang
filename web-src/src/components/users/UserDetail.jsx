import { useState, useEffect, useCallback } from 'preact/hooks';
import { Icon } from '@components/ui/Icon';
import { Spinner } from '@components/ui/Spinner';
import { ErrorBanner } from '@components/ui/ErrorBanner';
import { ToggleGroup } from '@components/ui/ToggleGroup';
import { api } from '@services/api';
import { showToast } from '@store/ui';
import {
  ALL_AUTH_ACTIONS,
  USERS_DETAIL_TABS,
} from '@constants/auth';
import { UserStatusBadge } from './helpers';
import { UserInfoSection } from './UserInfoSection';
import { GrantsSection } from './GrantsSection';
import { QuotaSection } from './QuotaSection';
import { AddGrantModal } from './AddGrantModal';

/**
 * UserDetail - Detail panel showing user info, grants, and quota in tabs.
 *
 * @param {Object} props
 * @param {number} props.userId - User ID to display
 * @param {() => void} props.onUserUpdated - Called after any user update (refreshes user list)
 * @param {() => void} props.onClose - Close the detail panel
 */
export function UserDetail({ userId, onUserUpdated, onClose }) {
  const [user, setUser] = useState(null);
  const [grants, setGrants] = useState([]);
  const [quotas, setQuotas] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [activeTab, setActiveTab] = useState('info');
  const [addGrantOpen, setAddGrantOpen] = useState(false);

  const fetchUser = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const result = await api.getUser(userId);
      const data = result.data || result;
      setUser(data.user);
      setGrants(data.grants || []);

      // Fetch quota separately (non-critical)
      try {
        const quotaResult = await api.getUserQuota(userId);
        const quotaData = quotaResult.data || quotaResult;
        setQuotas(quotaData.quotas || []);
      } catch {
        setQuotas([]);
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, [userId]);

  useEffect(() => {
    fetchUser();
  }, [fetchUser]);

  // Compute available actions for add grant
  const activeGrants = grants.filter(g => g.is_active);
  const grantedActionSet = new Set(activeGrants.map(g => g.action));
  const availableActions = ALL_AUTH_ACTIONS.filter(a => !grantedActionSet.has(a));

  if (loading) {
    return (
      <div class="user-detail">
        <div class="user-detail-loading">
          <Spinner />
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div class="user-detail">
        <div class="user-detail-header">
          <h2>Error</h2>
          <button class="credential-copy-btn" onClick={onClose} title="Close">
            <Icon name="x" size={16} />
          </button>
        </div>
        <div class="user-detail-body">
          <ErrorBanner message={error} onDismiss={() => setError(null)} />
        </div>
      </div>
    );
  }

  if (!user) return null;

  const handleUserUpdated = () => {
    fetchUser();
    onUserUpdated();
  };

  return (
    <div class="user-detail">
      <div class="user-detail-header">
        <div class="user-detail-header-info">
          <h2>{user.display_name || user.username}</h2>
          <UserStatusBadge user={user} />
        </div>
        <button class="credential-copy-btn" onClick={onClose} title="Close">
          <Icon name="x" size={16} />
        </button>
      </div>

      <div class="user-detail-tabs">
        <ToggleGroup
          options={USERS_DETAIL_TABS}
          activeId={activeTab}
          onToggle={setActiveTab}
          compact
        />
      </div>

      <div class="user-detail-body">
        {activeTab === 'info' && (
          <UserInfoSection user={user} onUserUpdated={handleUserUpdated} />
        )}

        {activeTab === 'grants' && (
          <GrantsSection
            userId={userId}
            grants={grants}
            onGrantsChanged={fetchUser}
            onOpenAddGrant={() => setAddGrantOpen(true)}
            hasAvailableActions={availableActions.length > 0}
          />
        )}

        {activeTab === 'quota' && (
          <QuotaSection quotas={quotas} />
        )}
      </div>

      {addGrantOpen && (
        <AddGrantModal
          userId={userId}
          availableActions={availableActions}
          onClose={() => setAddGrantOpen(false)}
          onGrantAdded={() => {
            setAddGrantOpen(false);
            fetchUser();
          }}
        />
      )}
    </div>
  );
}
