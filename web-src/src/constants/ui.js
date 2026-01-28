/**
 * UI Configuration Constants
 *
 * Centralized configuration for UI components styling and behavior.
 */

// =============================================================================
// DESIGN SYSTEM TOKENS
// =============================================================================

/**
 * Spacing scale (2px fine + 4px coarse increments)
 * Matches CSS variables in tokens.css (--space-*)
 * VERIFIED SYNC: 2026-01-13
 */
export const SPACING = {
  '0': '0',
  '0.5': '2px',
  '1': '4px',
  '1.5': '6px',
  '2': '8px',
  '2.5': '10px',
  '3': '12px',
  '3.5': '14px',
  '4': '16px',
  '5': '20px',
  '6': '24px',
  '8': '32px',
};

/**
 * Typography scale (2px increments, 8-20px range)
 * Matches CSS variables in tokens.css (--font-*)
 * VERIFIED SYNC: 2026-01-13
 */
export const FONT_SIZE = {
  '3xs': '0.5rem',      // 8px
  '2xs': '0.5625rem',   // 9px
  'xs': '0.625rem',     // 10px
  'sm': '0.6875rem',    // 11px
  'md': '0.75rem',      // 12px
  'base': '0.8125rem',  // 13px
  'lg': '0.875rem',     // 14px
  'xl': '1rem',         // 16px
  '2xl': '1.125rem',    // 18px
  '3xl': '1.25rem',     // 20px
};

/**
 * Border radius scale
 * Matches CSS variables in tokens.css (--radius-*)
 */
export const BORDER_RADIUS = {
  'xs': '2px',       // --radius-xs: slider tracks, thin borders
  'sm': '3px',       // --radius-sm: scrollbars, small elements
  'md': '4px',       // --radius-md: default (buttons, inputs)
  'lg': '6px',       // --radius-lg: cards, larger panels
  'xl': '8px',       // --radius-xl: prominent panels
  'pill': '12px',    // --radius-pill: chips, tags
  'badge': '10px',   // --radius-badge: badge counts
  'full': '50%',     // --radius-full: circular elements
};

/**
 * Transition timing and easing functions
 * Matches CSS variables in tokens.css (--transition-*, --ease-*)
 */
export const TRANSITIONS = {
  // Timing durations
  timing: {
    'instant': '0s',      // --transition-instant: immediate changes
    'fast': '0.1s',       // --transition-fast: quick interactions
    'normal': '0.15s',    // --transition-normal: default transitions
    'medium': '0.2s',     // --transition-medium: moderate animations
    'slow': '0.3s',       // --transition-slow: deliberate animations
  },

  // Easing functions
  easing: {
    'default': 'ease',                              // --ease-default: browser default
    'inOut': 'cubic-bezier(0.4, 0, 0.2, 1)',       // --ease-in-out: smooth accel/decel
    'out': 'cubic-bezier(0, 0, 0.2, 1)',           // --ease-out: quick start, slow end
  },
};

// =============================================================================
// ICON BUTTON CONFIGURATION
// =============================================================================

/**
 * IconButton size configurations.
 * Each size defines button dimensions and icon size.
 */
export const ICON_BUTTON_SIZES = {
  large: {
    button: '3rem',      // 48px - default toolbar size
    icon: 18,
  },
  medium: {
    button: '2.25rem',   // 36px - compact toolbar
    icon: 16,
  },
  small: {
    button: '1.75rem',   // 28px - inline/navigation hints
    icon: 12,
  },
};

/**
 * Default colors for IconButton component.
 * Uses CSS custom properties for theme consistency.
 */
export const ICON_BUTTON_COLORS = {
  // Default theme (green terminal style)
  default: {
    border: 'var(--border-dim)',
    color: 'var(--text-secondary)',
    active: 'var(--terminal-green)',
    hover: 'var(--terminal-green)',
  },

  // Cyan accent (for special actions)
  cyan: {
    border: 'var(--terminal-cyan)',
    color: 'var(--terminal-cyan)',
    active: 'var(--terminal-cyan)',
    hover: 'var(--terminal-cyan)',
  },

  // Amber/Orange accent (for warnings, sun-related)
  amber: {
    border: 'var(--terminal-amber)',
    color: 'var(--terminal-amber)',
    active: 'var(--terminal-amber)',
    hover: 'var(--terminal-amber)',
  },

  // Red accent (for destructive actions)
  red: {
    border: 'var(--terminal-red)',
    color: 'var(--terminal-red)',
    active: 'var(--terminal-red)',
    hover: 'var(--terminal-red)',
  },

  // Pink accent (spring theme)
  pink: {
    border: '#FFB7C5',
    color: '#FFB7C5',
    active: '#FFB7C5',
    hover: '#FFB7C5',
  },

  // Golden accent (summer theme)
  golden: {
    border: '#FFD700',
    color: '#FFD700',
    active: '#FFD700',
    hover: '#FFD700',
  },

  // Orange accent (autumn theme)
  orange: {
    border: '#FF8C00',
    color: '#FF8C00',
    active: '#FF8C00',
    hover: '#FF8C00',
  },

  // Light blue accent (winter theme)
  lightBlue: {
    border: '#87CEEB',
    color: '#87CEEB',
    active: '#87CEEB',
    hover: '#87CEEB',
  },
};

// =============================================================================
// TOAST NOTIFICATION CONFIGURATION
// =============================================================================

/**
 * Toast notification types
 * Maps to CSS classes: .cp-toast.info, .cp-toast.success, etc.
 */
export const TOAST_TYPES = {
  info: 'info',
  success: 'success',
  warning: 'warning',
  error: 'error',
};

/**
 * Default toast duration in milliseconds
 */
export const TOAST_DEFAULT_DURATION = 3000;

// =============================================================================
// DASHBOARD CONFIGURATION
// =============================================================================

/**
 * Minimum number of topics before showing the search/filter toolbar.
 * Below this threshold the toolbar adds clutter without value.
 */
export const TOPICS_TOOLBAR_MIN_COUNT = 15;

// =============================================================================
// APPLICATION METADATA
// =============================================================================

/**
 * Application repository URL (displayed in footer)
 */
export const APP_REPO_URL = 'https://github.com/Fantasim/silobang';

/**
 * Application display name
 */
export const APP_DISPLAY_NAME = 'SiloBang';
