import { useState, useRef } from 'preact/hooks';
import { Button } from '@components/ui/Button';
import { Modal } from '@components/ui/Modal';
import { Spinner } from '@components/ui/Spinner';
import { api } from '@services/api';
import { showToast } from '@store/ui';
import {
  getSelectedHashes,
  clearSelection,
  selectedPreset,
  queryParams,
  selectedTopics,
} from '@store/query';
import { TOAST_TYPES } from '@constants/ui';
import { DEFAULT_METADATA_PROCESSOR, DEFAULT_METADATA_PROCESSOR_VERSION } from '@constants/metadata';

/**
 * BulkMetadataModal - Set metadata on multiple assets
 *
 * @param {Object} props
 * @param {boolean} props.isOpen - Whether the modal is open
 * @param {Function} props.onClose - Called when modal closes
 * @param {string} props.mode - "selected" (default) or "apply-all"
 * @param {number} props.totalCount - Total count for apply-all mode
 */
export function BulkMetadataModal({ isOpen, onClose, mode = 'selected', totalCount = 0 }) {
  const [key, setKey] = useState('');
  const valueRef = useRef(null);
  const [processor, setProcessor] = useState(DEFAULT_METADATA_PROCESSOR);
  const [processorVersion, setProcessorVersion] = useState(DEFAULT_METADATA_PROCESSOR_VERSION);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [result, setResult] = useState(null);

  const isApplyAll = mode === 'apply-all';
  const selectedHashes = isApplyAll ? [] : getSelectedHashes();
  const count = isApplyAll ? totalCount : selectedHashes.length;

  const handleSubmit = async (e) => {
    e.preventDefault();

    if (!key.trim()) {
      showToast('Key is required', TOAST_TYPES.warning);
      return;
    }

    setSubmitting(true);
    setResult(null);

    try {
      let response;

      if (isApplyAll) {
        // Apply to all query results using applyMetadata
        if (!selectedPreset.value) {
          showToast('No query preset selected', TOAST_TYPES.error);
          setSubmitting(false);
          return;
        }

        response = await api.applyMetadata(
          selectedPreset.value.name,
          queryParams.value,
          selectedTopics.value,
          'set',
          key.trim(),
          valueRef.current?.value || '',
          processor,
          processorVersion
        );
      } else {
        // Apply to selected assets using batchMetadata
        const operations = selectedHashes.map(hash => ({
          hash,
          op: 'set',
          key: key.trim(),
          value: valueRef.current?.value || '',
        }));

        response = await api.batchMetadata(
          operations,
          processor,
          processorVersion
        );
      }

      setResult(response);

      if (response.failed === 0) {
        showToast(`Metadata set on ${response.succeeded} assets`, TOAST_TYPES.success);
        handleClose();
        if (!isApplyAll) {
          clearSelection();
        }
      } else {
        showToast(`Partial success: ${response.succeeded} succeeded, ${response.failed} failed`, TOAST_TYPES.warning);
      }
    } catch (err) {
      showToast(`Failed to set metadata: ${err.message}`, TOAST_TYPES.error);
    } finally {
      setSubmitting(false);
    }
  };

  const handleClose = () => {
    setKey('');
    if (valueRef.current) {
      valueRef.current.value = '';
    }
    setProcessor(DEFAULT_METADATA_PROCESSOR);
    setProcessorVersion(DEFAULT_METADATA_PROCESSOR_VERSION);
    setShowAdvanced(false);
    setResult(null);
    onClose();
  };

  const title = isApplyAll ? 'Apply Metadata to All Results' : 'Set Metadata';
  const infoText = isApplyAll
    ? `Applying metadata to all ${count} result${count !== 1 ? 's' : ''} from current query`
    : `Setting metadata on ${count} selected asset${count !== 1 ? 's' : ''}`;
  const buttonText = isApplyAll
    ? `Apply to All ${count} Result${count !== 1 ? 's' : ''}`
    : `Apply to ${count} Asset${count !== 1 ? 's' : ''}`;

  return (
    <Modal
      isOpen={isOpen}
      onClose={submitting ? undefined : handleClose}
      title={title}
    >
      <div class="bulk-metadata-form">
        <div class="bulk-metadata-info">
          <strong>{infoText}</strong>
        </div>

        {result && result.failed > 0 && (
          <div class="bulk-metadata-results">
            <div class="bulk-metadata-results-summary">
              <span class="bulk-metadata-results-stat success">
                {result.succeeded} succeeded
              </span>
              <span class="bulk-metadata-results-stat failed">
                {result.failed} failed
              </span>
            </div>
          </div>
        )}

        <form onSubmit={handleSubmit}>
          <div class="bulk-metadata-field">
            <label htmlFor="bulk-key">Key</label>
            <input
              id="bulk-key"
              type="text"
              placeholder="Enter metadata key"
              value={key}
              onInput={(e) => setKey(e.target.value)}
              disabled={submitting}
              autoFocus
            />
          </div>

          <div class="bulk-metadata-field">
            <label htmlFor="bulk-value">Value</label>
            <textarea
              id="bulk-value"
              ref={valueRef}
              placeholder="Enter metadata value (supports large values)"
              disabled={submitting}
              rows={3}
            />
          </div>

          <div
            class="bulk-metadata-advanced-toggle"
            onClick={() => setShowAdvanced(!showAdvanced)}
          >
            {showAdvanced ? '▼' : '▶'} Advanced
          </div>

          {showAdvanced && (
            <div class="bulk-metadata-advanced">
              <div class="bulk-metadata-field">
                <label htmlFor="bulk-processor">Processor</label>
                <input
                  id="bulk-processor"
                  type="text"
                  placeholder="Processor name"
                  value={processor}
                  onInput={(e) => setProcessor(e.target.value)}
                  disabled={submitting}
                />
              </div>
              <div class="bulk-metadata-field">
                <label htmlFor="bulk-processor-version">Version</label>
                <input
                  id="bulk-processor-version"
                  type="text"
                  placeholder="Version"
                  value={processorVersion}
                  onInput={(e) => setProcessorVersion(e.target.value)}
                  disabled={submitting}
                />
              </div>
            </div>
          )}

          <div style={{ display: 'flex', gap: 'var(--space-2)' }}>
            <Button type="submit" disabled={submitting || !key.trim()}>
              {submitting ? (
                <>
                  <Spinner /> Applying...
                </>
              ) : (
                buttonText
              )}
            </Button>
            <Button type="button" variant="ghost" onClick={handleClose} disabled={submitting}>
              Cancel
            </Button>
          </div>
        </form>
      </div>
    </Modal>
  );
}
