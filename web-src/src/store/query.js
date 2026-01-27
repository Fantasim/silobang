import { signal } from '@preact/signals';
import { api } from '@services/api';

// State
export const presets = signal([]);
export const presetsLoading = signal(false);
export const selectedPreset = signal(null);
export const queryParams = signal({});
export const selectedTopics = signal([]);
export const queryResult = signal(null);
export const queryLoading = signal(false);
export const queryError = signal(null);

// Pagination
export const currentPage = signal(1);
export const pageSize = signal(100);

// Sorting
export const sortColumn = signal(null);
export const sortDirection = signal('asc');

// UI State
export const presetSelectorExpanded = signal(false);  // Whether the preset selection panel is open
export const presetSearchQuery = signal('');  // Search filter for presets

// Asset Drawer State
export const assetDrawerOpen = signal(false);
export const assetDrawerHash = signal(null);

// Asset Drawer Actions
export function openAssetDrawer(hash) {
  assetDrawerHash.value = hash;
  assetDrawerOpen.value = true;
}

export function closeAssetDrawer() {
  assetDrawerOpen.value = false;
  assetDrawerHash.value = null;
}

// Row Selection State
export const selectedRows = signal(new Set());

export function toggleRowSelection(hash) {
  const newSet = new Set(selectedRows.value);
  if (newSet.has(hash)) {
    newSet.delete(hash);
  } else {
    newSet.add(hash);
  }
  selectedRows.value = newSet;
}

export function selectAllRows(hashes) {
  selectedRows.value = new Set(hashes);
}

export function clearSelection() {
  selectedRows.value = new Set();
}

export function getSelectedHashes() {
  return Array.from(selectedRows.value);
}

// Actions
export async function fetchPresets() {
  presetsLoading.value = true;

  try {
    const data = await api.getQueries();
    presets.value = data.presets || [];
  } catch (err) {
    console.error('Failed to fetch presets:', err);
  } finally {
    presetsLoading.value = false;
  }
}

export function selectPreset(presetName) {
  const preset = presets.value.find(p => p.name === presetName);
  selectedPreset.value = preset || null;

  // Initialize params with defaults
  if (preset) {
    const defaults = {};
    for (const param of preset.params) {
      defaults[param.name] = param.default || '';
    }
    queryParams.value = defaults;
  } else {
    queryParams.value = {};
  }

  // Reset results
  queryResult.value = null;
  queryError.value = null;
  currentPage.value = 1;
}

export function setQueryParam(name, value) {
  queryParams.value = { ...queryParams.value, [name]: value };
}

export async function runQuery() {
  if (!selectedPreset.value) return;

  queryLoading.value = true;
  queryError.value = null;

  try {
    const result = await api.runQuery(
      selectedPreset.value.name,
      queryParams.value,
      selectedTopics.value
    );
    queryResult.value = result;
    currentPage.value = 1;
    sortColumn.value = null;
    sortDirection.value = 'asc';
  } catch (err) {
    queryError.value = err.message;
    queryResult.value = null;
  } finally {
    queryLoading.value = false;
  }
}

export function setSorting(column) {
  if (sortColumn.value === column) {
    sortDirection.value = sortDirection.value === 'asc' ? 'desc' : 'asc';
  } else {
    sortColumn.value = column;
    sortDirection.value = 'asc';
  }
}
