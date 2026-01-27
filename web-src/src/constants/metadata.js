/**
 * Metadata Display Configuration Constants
 *
 * Centralized configuration for metadata value display behavior.
 */

// =============================================================================
// TRUNCATION SETTINGS
// =============================================================================

/**
 * Maximum length before truncating metadata value display.
 * Values longer than this will show truncated with "..." and be clickable.
 */
export const METADATA_TRUNCATE_LENGTH = 100;

/**
 * Number of spaces for JSON indentation when pretty-printing.
 */
export const METADATA_JSON_INDENT = 2;

// =============================================================================
// METADATA PROCESSOR DEFAULTS
// =============================================================================

/**
 * Default processor name for metadata operations.
 */
export const DEFAULT_METADATA_PROCESSOR = 'silobang-ui';

/**
 * Default processor version for metadata operations.
 */
export const DEFAULT_METADATA_PROCESSOR_VERSION = '1.0';
