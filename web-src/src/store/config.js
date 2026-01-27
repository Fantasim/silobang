import { signal, computed } from '@preact/signals';
import { api } from '@services/api';
import { setBootstrapCredentials } from '@store/auth';

// State
export const config = signal(null);
export const configLoading = signal(false);
export const configError = signal(null);

// Computed
export const isConfigured = computed(() =>
  config.value?.configured === true
);

// Actions
export async function fetchConfig() {
  configLoading.value = true;
  configError.value = null;

  try {
    const data = await api.getConfig();
    config.value = data;
  } catch (err) {
    configError.value = err.message;
  } finally {
    configLoading.value = false;
  }
}

export async function setWorkingDirectory(path) {
  configLoading.value = true;
  configError.value = null;

  try {
    const result = await api.setConfig(path);

    // Extract bootstrap credentials from response (first-time setup)
    const data = result.data || result;
    if (data.bootstrap) {
      setBootstrapCredentials(data.bootstrap);
    }

    await fetchConfig();
    return true;
  } catch (err) {
    configError.value = err.message;
    return false;
  } finally {
    configLoading.value = false;
  }
}
