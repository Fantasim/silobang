import { signal, batch } from '@preact/signals';
import { api } from '@services/api';
import { MONITORING_REFRESH_INTERVAL_MS } from '@constants/monitoring';

// =============================================================================
// STATE SIGNALS
// =============================================================================

export const monitoringData = signal(null);
export const monitoringLoading = signal(false);
export const monitoringError = signal(null);

// =============================================================================
// INTERNAL
// =============================================================================

let _refreshInterval = null;

// =============================================================================
// ACTIONS
// =============================================================================

export async function fetchMonitoring() {
  monitoringLoading.value = true;
  monitoringError.value = null;

  try {
    const data = await api.getMonitoring();
    monitoringData.value = data;
  } catch (err) {
    monitoringError.value = err.message || 'Failed to fetch monitoring data';
  } finally {
    monitoringLoading.value = false;
  }
}

export function startAutoRefresh() {
  stopAutoRefresh();
  _refreshInterval = setInterval(fetchMonitoring, MONITORING_REFRESH_INTERVAL_MS);
}

export function stopAutoRefresh() {
  if (_refreshInterval) {
    clearInterval(_refreshInterval);
    _refreshInterval = null;
  }
}

export function cleanupMonitoring() {
  stopAutoRefresh();
  batch(() => {
    monitoringData.value = null;
    monitoringLoading.value = false;
    monitoringError.value = null;
  });
}
