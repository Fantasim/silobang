import { useState } from 'preact/hooks';
import { MetadataModal } from './MetadataModal';
import {
  formatMetadataValue,
  truncateValue,
  isValueTruncated,
} from '../../utils/metadata';
import { METADATA_TRUNCATE_LENGTH } from '@constants/metadata.js';

/**
 * Component for displaying metadata values with truncation and expand-to-modal.
 *
 * Short values are displayed inline. Long values are truncated with "..."
 * and clicking opens a modal with the full value.
 *
 * @param {Object} props
 * @param {*} props.value - The metadata value to display
 * @param {string} [props.label] - Optional label/key name (used in modal title)
 * @param {number} [props.maxLength] - Maximum length before truncation
 * @param {string} [props.className] - Additional CSS class
 */
export function MetadataValue({
  value,
  label,
  maxLength = METADATA_TRUNCATE_LENGTH,
  className = '',
}) {
  const [modalOpen, setModalOpen] = useState(false);

  const isTruncated = isValueTruncated(value, maxLength);
  const displayValue = isTruncated
    ? truncateValue(value, maxLength)
    : formatMetadataValue(value, false);

  const handleClick = () => {
    if (isTruncated) {
      setModalOpen(true);
    }
  };

  const baseClass = 'metadata-value';
  const classes = [
    baseClass,
    isTruncated && `${baseClass}-clickable`,
    className,
  ]
    .filter(Boolean)
    .join(' ');

  return (
    <>
      <span
        class={classes}
        onClick={handleClick}
        title={isTruncated ? 'Click to view full value' : undefined}
      >
        {displayValue}
      </span>

      {isTruncated && (
        <MetadataModal
          isOpen={modalOpen}
          onClose={() => setModalOpen(false)}
          value={value}
          label={label}
        />
      )}
    </>
  );
}
