import { signal, computed } from '@preact/signals';
import { api } from '@services/api';
import {
  getStoredToken,
  setStoredToken,
  clearStoredToken,
  setUnauthorizedHandler,
} from '@services/token';
import { AUTH_STATUS, AUTH_ACTIONS } from '@constants/auth';

// =============================================================================
// STATE SIGNALS
// =============================================================================

export const authStatus = signal(AUTH_STATUS.LOADING);
export const authError = signal(null);
export const currentUser = signal(null);
export const currentGrants = signal([]);
export const authMethod = signal(null);

// Bootstrap credentials (shown once after first-time setup)
export const bootstrapCredentials = signal(null);

// =============================================================================
// COMPUTED — identity
// =============================================================================

export const isAuthenticated = computed(() =>
  authStatus.value === AUTH_STATUS.AUTHENTICATED
);

export const isLoading = computed(() =>
  authStatus.value === AUTH_STATUS.LOADING
);

export const isUnconfigured = computed(() =>
  authStatus.value === AUTH_STATUS.UNCONFIGURED
);

// Set of currently granted action strings for fast lookup
export const grantedActions = computed(() =>
  new Set(
    currentGrants.value
      .filter(g => g.is_active)
      .map(g => g.action)
  )
);

// =============================================================================
// COMPUTED — per-action permission checks
// =============================================================================

export const canUpload = computed(() =>
  grantedActions.value.has(AUTH_ACTIONS.UPLOAD)
);

export const canDownload = computed(() =>
  grantedActions.value.has(AUTH_ACTIONS.DOWNLOAD)
);

export const canQuery = computed(() =>
  grantedActions.value.has(AUTH_ACTIONS.QUERY)
);

export const canManageUsers = computed(() =>
  grantedActions.value.has(AUTH_ACTIONS.MANAGE_USERS)
);

export const canManageTopics = computed(() =>
  grantedActions.value.has(AUTH_ACTIONS.MANAGE_TOPICS)
);

export const canMetadata = computed(() =>
  grantedActions.value.has(AUTH_ACTIONS.METADATA)
);

export const canBulkDownload = computed(() =>
  grantedActions.value.has(AUTH_ACTIONS.BULK_DOWNLOAD)
);

export const canViewAudit = computed(() =>
  grantedActions.value.has(AUTH_ACTIONS.VIEW_AUDIT)
);

export const canVerify = computed(() =>
  grantedActions.value.has(AUTH_ACTIONS.VERIFY)
);

export const canManageConfig = computed(() =>
  grantedActions.value.has(AUTH_ACTIONS.MANAGE_CONFIG)
);

// =============================================================================
// ACTIONS
// =============================================================================

/**
 * Check initial auth status on app mount.
 * Determines: unconfigured → unauthenticated → authenticated
 */
export async function checkAuthStatus() {
  authStatus.value = AUTH_STATUS.LOADING;
  authError.value = null;

  try {
    const result = await api.getAuthStatus();
    const status = result.data || result;

    if (!status.configured) {
      authStatus.value = AUTH_STATUS.UNCONFIGURED;
      return;
    }

    // System is configured — try to restore session from stored token
    const token = getStoredToken();
    if (!token) {
      authStatus.value = AUTH_STATUS.UNAUTHENTICATED;
      return;
    }

    // Validate token by fetching current identity
    try {
      const meResult = await api.getAuthMe();
      const me = meResult.data || meResult;
      currentUser.value = me.user;
      currentGrants.value = me.grants || [];
      authMethod.value = me.method;
      authStatus.value = AUTH_STATUS.AUTHENTICATED;
    } catch {
      // Token invalid or expired
      clearStoredToken();
      authStatus.value = AUTH_STATUS.UNAUTHENTICATED;
    }
  } catch (err) {
    authError.value = err.message;
    authStatus.value = AUTH_STATUS.UNAUTHENTICATED;
  }
}

/**
 * Log in with username/password. Returns {success, error?}.
 */
export async function login(username, password) {
  authError.value = null;

  try {
    const loginResult = await api.login(username, password);
    const loginData = loginResult.data || loginResult;
    setStoredToken(loginData.token);
    currentUser.value = loginData.user;

    // Fetch full identity (includes grants)
    const meResult = await api.getAuthMe();
    const me = meResult.data || meResult;
    currentGrants.value = me.grants || [];
    authMethod.value = me.method;
    authStatus.value = AUTH_STATUS.AUTHENTICATED;

    return { success: true };
  } catch (err) {
    return { success: false, error: err };
  }
}

/**
 * Log out — invalidate server session and clear local state.
 */
export async function logout() {
  try {
    await api.logout();
  } catch {
    // Server-side invalidation may fail (e.g. already expired), but clear locally regardless
  } finally {
    clearStoredToken();
    currentUser.value = null;
    currentGrants.value = [];
    authMethod.value = null;
    authStatus.value = AUTH_STATUS.UNAUTHENTICATED;
  }
}

/**
 * Handle global 401 — called from api.js via token.js callback.
 * Clears session and redirects to login.
 */
export function handleUnauthorized() {
  clearStoredToken();
  currentUser.value = null;
  currentGrants.value = [];
  authMethod.value = null;
  authStatus.value = AUTH_STATUS.UNAUTHENTICATED;
}

/**
 * Check if the current user has a specific grant.
 */
export function hasGrant(action) {
  return grantedActions.value.has(action);
}

// =============================================================================
// BOOTSTRAP CREDENTIALS
// =============================================================================

export function setBootstrapCredentials(creds) {
  bootstrapCredentials.value = creds;
}

export function clearBootstrapCredentials() {
  bootstrapCredentials.value = null;
}

/**
 * Transition from unconfigured → unauthenticated after setup + credential acknowledgement.
 */
export function markConfigured() {
  authStatus.value = AUTH_STATUS.UNAUTHENTICATED;
}

// =============================================================================
// REGISTER GLOBAL 401 HANDLER
// =============================================================================

setUnauthorizedHandler(handleUnauthorized);
