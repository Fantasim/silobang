import { Modal } from './Modal';
import { Button } from './Button';
import { showToast } from '@store/ui';
import { formatMetadataValue } from '../../utils/metadata';
import { TOAST_TYPES } from '@constants/ui.js';

/**
 * Modal for displaying full metadata values.
 * Used when metadata values are too long to display inline.
 *
 * @param {Object} props
 * @param {boolean} props.isOpen - Whether the modal is open
 * @param {Function} props.onClose - Callback when modal closes
 * @param {*} props.value - The metadata value to display
 * @param {string} [props.label] - Optional label/key name for the title
 */
export function MetadataModal({ isOpen, onClose, value, label }) {
  const formattedValue = formatMetadataValue(value, true);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(formattedValue);
      showToast('Copied to clipboard', TOAST_TYPES.info);
    } catch (err) {
      showToast('Failed to copy', TOAST_TYPES.error);
    }
  };

  const title = label ? `Metadata: ${label}` : 'Metadata Value';

  const headerActions = (
    <Button variant="ghost" onClick={handleCopy} title="Copy to clipboard">
      Copy
    </Button>
  );

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={title}
      headerActions={headerActions}
      className="metadata-modal"
    >
      <div class="metadata-modal-content">
        <pre class="metadata-modal-value">{formattedValue}</pre>
      </div>
    </Modal>
  );
}
