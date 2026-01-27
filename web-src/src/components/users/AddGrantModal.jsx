import { useState } from 'preact/hooks';
import { Button } from '@components/ui/Button';
import { Modal } from '@components/ui/Modal';
import { Icon } from '@components/ui/Icon';
import { Spinner } from '@components/ui/Spinner';
import { ErrorBanner } from '@components/ui/ErrorBanner';
import { api } from '@services/api';
import { showToast } from '@store/ui';
import {
  AUTH_ACTION_LABELS,
  AUTH_ACTION_DESCRIPTIONS,
  AUTH_CONSTRAINT_FIELDS,
  AUTH_ERROR_MESSAGES,
  USERS_NO_CONSTRAINTS_TEXT,
} from '@constants/auth';
import { ConstraintEditor } from './ConstraintEditor';

/**
 * AddGrantModal - Two-step modal for granting a new permission.
 * Step 1: Select an action from a visual card grid.
 * Step 2: Configure constraints via the visual editor.
 *
 * @param {Object} props
 * @param {number} props.userId - User ID to grant permission to
 * @param {string[]} props.availableActions - Actions not yet granted
 * @param {() => void} props.onClose - Close the modal
 * @param {() => void} props.onGrantAdded - Called after successful grant creation
 */
export function AddGrantModal({ userId, availableActions, onClose, onGrantAdded }) {
  const [selectedAction, setSelectedAction] = useState(null);
  const [constraints, setConstraints] = useState({});
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);

  const handleCreate = async () => {
    setLoading(true);
    setError(null);
    try {
      const constraintsJson = Object.keys(constraints).length > 0
        ? JSON.stringify(constraints)
        : null;
      await api.createGrant(userId, selectedAction, constraintsJson);
      showToast(`Grant "${AUTH_ACTION_LABELS[selectedAction]}" added`, 'success');
      onGrantAdded();
      onClose();
    } catch (err) {
      const message = AUTH_ERROR_MESSAGES[err.code] || err.message;
      setError(message);
    } finally {
      setLoading(false);
    }
  };

  // Step 1: Select action
  if (!selectedAction) {
    return (
      <Modal isOpen title="Add Grant" onClose={onClose}>
        <p class="add-grant-subtitle">
          Select a permission to grant:
        </p>
        <div class="add-grant-actions-grid">
          {availableActions.map((action) => (
            <button
              key={action}
              class="add-grant-action-card"
              onClick={() => setSelectedAction(action)}
              type="button"
            >
              <span class="add-grant-action-card-label">
                {AUTH_ACTION_LABELS[action]}
              </span>
              <span class="add-grant-action-card-desc">
                {AUTH_ACTION_DESCRIPTIONS[action]}
              </span>
            </button>
          ))}
        </div>
      </Modal>
    );
  }

  // Step 2: Configure constraints
  const fields = AUTH_CONSTRAINT_FIELDS[selectedAction] || [];

  return (
    <Modal
      isOpen
      title={`Grant: ${AUTH_ACTION_LABELS[selectedAction]}`}
      onClose={onClose}
    >
      <button
        class="add-grant-back"
        onClick={() => { setSelectedAction(null); setConstraints({}); setError(null); }}
        type="button"
      >
        <Icon name="arrowLeft" size={14} />
        Change action
      </button>

      {fields.length > 0 ? (
        <div class="add-grant-constraints">
          <h4 class="add-grant-constraints-title">Constraints (optional)</h4>
          <p class="add-grant-constraints-desc">
            Leave fields empty for unlimited access, or set limits below:
          </p>
          <ConstraintEditor
            action={selectedAction}
            constraints={constraints}
            onChange={setConstraints}
          />
        </div>
      ) : (
        <p class="constraint-editor-empty">
          {USERS_NO_CONSTRAINTS_TEXT}
        </p>
      )}

      {error && <ErrorBanner message={error} onDismiss={() => setError(null)} />}

      <Button
        onClick={handleCreate}
        disabled={loading}
        style={{ width: '100%', marginTop: 'var(--space-4)' }}
      >
        {loading ? <Spinner /> : 'Add Grant'}
      </Button>
    </Modal>
  );
}
