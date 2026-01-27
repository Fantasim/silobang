import { useState } from 'preact/hooks';
import { Button } from '@components/ui/Button';
import { Icon } from '@components/ui/Icon';
import { api } from '@services/api';
import { showToast } from '@store/ui';
import {
  AUTH_ACTION_LABELS,
  AUTH_ERROR_MESSAGES,
  AUTH_CONSTRAINT_FIELDS,
} from '@constants/auth';
import { ConstraintEditor } from './ConstraintEditor';
import { ConstraintSummary } from './ConstraintSummary';

/**
 * GrantsSection - Displays and manages user grants with visual constraint editing.
 *
 * @param {Object} props
 * @param {number} props.userId - User ID
 * @param {Array} props.grants - Array of grant objects
 * @param {() => void} props.onGrantsChanged - Called after any grant change
 * @param {() => void} props.onOpenAddGrant - Called when user wants to add a grant
 * @param {boolean} props.hasAvailableActions - Whether there are ungrated actions available
 */
export function GrantsSection({ userId, grants, onGrantsChanged, onOpenAddGrant, hasAvailableActions }) {
  const [editingGrantId, setEditingGrantId] = useState(null);
  const [editConstraints, setEditConstraints] = useState({});
  const [saveLoading, setSaveLoading] = useState(false);

  const activeGrants = grants.filter(g => g.is_active);
  const revokedGrants = grants.filter(g => !g.is_active);

  const handleRevokeGrant = async (grant) => {
    if (!confirm(`Revoke "${AUTH_ACTION_LABELS[grant.action]}" grant?`)) return;
    try {
      await api.revokeGrant(grant.id);
      showToast(`Grant "${AUTH_ACTION_LABELS[grant.action]}" revoked`, 'success');
      onGrantsChanged();
    } catch (err) {
      const message = AUTH_ERROR_MESSAGES[err.code] || err.message;
      showToast(message, 'error');
    }
  };

  const handleSaveConstraints = async (grantId) => {
    setSaveLoading(true);
    try {
      const constraintsJson = Object.keys(editConstraints).length > 0
        ? JSON.stringify(editConstraints)
        : null;
      await api.updateGrant(grantId, constraintsJson);
      showToast('Constraints updated', 'success');
      setEditingGrantId(null);
      setEditConstraints({});
      onGrantsChanged();
    } catch (err) {
      if (err instanceof SyntaxError) {
        showToast('Invalid constraints', 'error');
      } else {
        const message = AUTH_ERROR_MESSAGES[err.code] || err.message;
        showToast(message, 'error');
      }
    } finally {
      setSaveLoading(false);
    }
  };

  const startEditConstraints = (grant) => {
    setEditingGrantId(grant.id);
    try {
      setEditConstraints(
        grant.constraints_json ? JSON.parse(grant.constraints_json) : {}
      );
    } catch {
      setEditConstraints({});
    }
  };

  const cancelEdit = () => {
    setEditingGrantId(null);
    setEditConstraints({});
  };

  return (
    <div class="grants-section">
      <div class="grants-header">
        <span class="grants-count">
          {activeGrants.length} active grant{activeGrants.length !== 1 ? 's' : ''}
        </span>
        {hasAvailableActions && (
          <Button onClick={onOpenAddGrant}>
            <Icon name="plus" size={14} />
            Add Grant
          </Button>
        )}
      </div>

      {activeGrants.length > 0 ? (
        <div class="grants-list">
          {activeGrants.map((grant) => (
            <div key={grant.id} class={`grant-card ${editingGrantId === grant.id ? 'grant-card--editing' : ''}`}>
              <div class="grant-card-header">
                <span class="grant-action-badge">
                  {AUTH_ACTION_LABELS[grant.action] || grant.action}
                </span>
                <div class="grant-card-actions">
                  {editingGrantId !== grant.id && (AUTH_CONSTRAINT_FIELDS[grant.action]?.length > 0) && (
                    <button
                      class="credential-copy-btn"
                      onClick={() => startEditConstraints(grant)}
                      title="Edit constraints"
                    >
                      <Icon name="pencil" size={12} />
                    </button>
                  )}
                  <button
                    class="credential-copy-btn"
                    onClick={() => handleRevokeGrant(grant)}
                    title="Revoke grant"
                    style={{ color: 'var(--terminal-red)' }}
                  >
                    <Icon name="trash2" size={12} />
                  </button>
                </div>
              </div>

              {editingGrantId === grant.id ? (
                <div class="grant-card-edit">
                  <ConstraintEditor
                    action={grant.action}
                    constraints={editConstraints}
                    onChange={setEditConstraints}
                  />
                  <div class="grant-card-edit-actions">
                    <Button
                      onClick={() => handleSaveConstraints(grant.id)}
                      disabled={saveLoading}
                    >
                      {saveLoading ? 'Saving...' : 'Save'}
                    </Button>
                    <Button onClick={cancelEdit}>Cancel</Button>
                  </div>
                </div>
              ) : (
                <div class="grant-card-constraints">
                  <ConstraintSummary
                    action={grant.action}
                    constraintsJson={grant.constraints_json}
                  />
                </div>
              )}
            </div>
          ))}
        </div>
      ) : (
        <p class="grants-empty">No grants assigned. Click "Add Grant" to get started.</p>
      )}

      {revokedGrants.length > 0 && (
        <details class="grants-revoked-details">
          <summary class="grants-revoked-summary">
            {revokedGrants.length} revoked grant{revokedGrants.length !== 1 ? 's' : ''}
          </summary>
          <div class="grants-list grants-list--revoked">
            {revokedGrants.map((grant) => (
              <div key={grant.id} class="grant-card grant-card--revoked">
                <div class="grant-card-header">
                  <span class="grant-action-badge grant-action-badge--revoked">
                    {AUTH_ACTION_LABELS[grant.action] || grant.action}
                  </span>
                  <span class="grant-revoked-label">Revoked</span>
                </div>
                <div class="grant-card-constraints">
                  <ConstraintSummary
                    action={grant.action}
                    constraintsJson={grant.constraints_json}
                  />
                </div>
              </div>
            ))}
          </div>
        </details>
      )}
    </div>
  );
}
