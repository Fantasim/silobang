import { AUTH_TOKEN_STORAGE_KEY, AUTH_TOKEN_QUERY_PARAM } from '@constants/auth';

// Callback for when a 401 is received (registered by auth store to avoid circular deps)
let _onUnauthorized = null;

export function setUnauthorizedHandler(handler) {
  _onUnauthorized = handler;
}

export function notifyUnauthorized() {
  if (_onUnauthorized) _onUnauthorized();
}

// =============================================================================
// TOKEN PERSISTENCE
// =============================================================================

export function getStoredToken() {
  try {
    return localStorage.getItem(AUTH_TOKEN_STORAGE_KEY);
  } catch {
    return null;
  }
}

export function setStoredToken(token) {
  try {
    localStorage.setItem(AUTH_TOKEN_STORAGE_KEY, token);
  } catch {
    // localStorage unavailable (private browsing, etc.)
  }
}

export function clearStoredToken() {
  try {
    localStorage.removeItem(AUTH_TOKEN_STORAGE_KEY);
  } catch {
    // localStorage unavailable
  }
}

// =============================================================================
// URL TOKEN INJECTION — for SSE (EventSource) and download (window.open)
// =============================================================================

export function appendAuthToken(url) {
  const token = getStoredToken();
  if (!token) return url;
  const separator = url.includes('?') ? '&' : '?';
  return `${url}${separator}${AUTH_TOKEN_QUERY_PARAM}=${encodeURIComponent(token)}`;
}
