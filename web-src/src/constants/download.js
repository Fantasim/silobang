/**
 * Bulk Download Constants
 *
 * Options and defaults for the bulk download feature.
 */

/**
 * Filename format options for ToggleGroup selector.
 * Maps to backend constants: FilenameFormatHash, FilenameFormatOriginal, FilenameFormatHashOriginal
 */
export const FILENAME_FORMATS = [
  { id: 'original', label: 'Original', description: 'Use original filename' },
  { id: 'hash', label: 'Hash', description: 'Use asset hash as filename' },
  { id: 'hash_original', label: 'Hash + Original', description: 'Combine hash and original name' },
];

/** Default filename format for bulk downloads */
export const DEFAULT_FILENAME_FORMAT = 'original';

/** Default metadata inclusion for bulk downloads */
export const DEFAULT_INCLUDE_METADATA = false;
