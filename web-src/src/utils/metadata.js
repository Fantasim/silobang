import {
  METADATA_TRUNCATE_LENGTH,
  METADATA_JSON_INDENT,
} from '@constants/metadata.js';

/**
 * Convert a metadata value to a display string.
 * Objects are JSON-stringified with pretty formatting.
 *
 * @param {*} value - The metadata value to format
 * @param {boolean} prettyPrint - Whether to pretty-print JSON (default: true)
 * @returns {string} The formatted string representation
 */
export function formatMetadataValue(value, prettyPrint = true) {
  if (value === null || value === undefined) {
    return '-';
  }

  if (typeof value === 'object') {
    return prettyPrint
      ? JSON.stringify(value, null, METADATA_JSON_INDENT)
      : JSON.stringify(value);
  }

  return String(value);
}

/**
 * Truncate a string to a maximum length with ellipsis.
 *
 * @param {string} str - The string to truncate
 * @param {number} maxLength - Maximum length (default: METADATA_TRUNCATE_LENGTH)
 * @returns {string} The truncated string with "..." if it was shortened
 */
export function truncateValue(str, maxLength = METADATA_TRUNCATE_LENGTH) {
  if (typeof str !== 'string') {
    str = formatMetadataValue(str, false);
  }

  if (str.length <= maxLength) {
    return str;
  }

  return str.substring(0, maxLength) + '...';
}

/**
 * Check if a value would be truncated at the given length.
 *
 * @param {*} value - The value to check
 * @param {number} maxLength - Maximum length threshold (default: METADATA_TRUNCATE_LENGTH)
 * @returns {boolean} True if the value would be truncated
 */
export function isValueTruncated(value, maxLength = METADATA_TRUNCATE_LENGTH) {
  const str = formatMetadataValue(value, false);
  return str.length > maxLength;
}
