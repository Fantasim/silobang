import { signal, computed } from '@preact/signals';
import { api } from '@services/api';
import {
  FileStatus,
  MAX_CONCURRENT_UPLOADS,
  MAX_DISPLAY_ITEMS,
  UPLOAD_UI_UPDATE_INTERVAL_MS,
} from '@constants/upload';

// Re-export for backward compatibility
export { FileStatus } from '@constants/upload';

// Public signals (updated on throttled schedule)
export const isUploading = signal(false);
export const parentId = signal('');
export const totalStats = signal({ total: 0, added: 0, skipped: 0, errors: 0 });
export const displayQueue = signal([]);
export const uploadQueue = displayQueue;  // Alias for compatibility

// Internal mutable state (NOT reactive - avoids memory thrashing)
let _stats = { total: 0, added: 0, skipped: 0, errors: 0 };
let _displayItems = [];
let _updateScheduled = false;
let _cancelRequested = false;

// Throttled UI update - batches all changes into single signal update
function scheduleUIUpdate() {
  if (_updateScheduled) return;
  _updateScheduled = true;

  setTimeout(() => {
    _updateScheduled = false;
    totalStats.value = { ..._stats };
    displayQueue.value = _displayItems.slice();
  }, UPLOAD_UI_UPDATE_INTERVAL_MS);
}

// Force immediate UI update (for initial/final state)
function flushUIUpdate() {
  _updateScheduled = false;
  totalStats.value = { ..._stats };
  displayQueue.value = _displayItems.slice();
}

// Mutable display queue management (max 100 items)
function addToDisplay(item) {
  if (_displayItems.length >= MAX_DISPLAY_ITEMS) {
    _displayItems.shift();
  }
  _displayItems.push(item);
  scheduleUIUpdate();
}

function updateDisplayStatus(fileName, status, error = null) {
  const item = _displayItems.find(i => i.fileName === fileName);
  if (item) {
    item.status = status;
    if (error) item.error = error;
    scheduleUIUpdate();
  }
}

// Generator for streaming file consumption - yields files one at a time
// Nulls array entries after yielding to release memory
function* createFileIterator(fileArray) {
  for (let i = 0; i < fileArray.length; i++) {
    const file = fileArray[i];
    fileArray[i] = null;  // Release reference immediately after yielding
    yield file;
  }
}

// Upload single file (file reference is local, eligible for GC after function returns)
async function uploadFile(topicName, file) {
  const fileName = file.name;
  const fileSize = file.size;

  addToDisplay({
    fileName,
    fileSize,
    status: FileStatus.UPLOADING,
    error: null,
  });

  try {
    const result = await api.uploadAsset(topicName, file, parentId.value || null);
    // file reference goes out of scope here - eligible for GC

    updateDisplayStatus(fileName, result.skipped ? FileStatus.SKIPPED : FileStatus.SUCCESS);
    _stats[result.skipped ? 'skipped' : 'added']++;
    scheduleUIUpdate();

  } catch (err) {
    updateDisplayStatus(fileName, FileStatus.ERROR, err.message);
    _stats.errors++;
    scheduleUIUpdate();
  }
}

// Main upload function - streaming pattern with generator
export async function startUpload(topicName, files) {
  if (isUploading.value || !files || files.length === 0) return;

  // CRITICAL: Convert FileList to array IMMEDIATELY (synchronously)
  // FileList becomes invalid after the event handler returns
  const fileArray = Array.from(files);

  // Reset state
  _stats = { total: fileArray.length, added: 0, skipped: 0, errors: 0 };
  _displayItems = [];
  _cancelRequested = false;

  isUploading.value = true;
  flushUIUpdate();

  const fileIterator = createFileIterator(fileArray);
  const activeUploads = new Map();

  try {
    while (true) {
      if (_cancelRequested) break;

      // Fill concurrent upload slots (max 3)
      while (activeUploads.size < MAX_CONCURRENT_UPLOADS) {
        const { done, value: file } = fileIterator.next();
        if (done) break;

        const uploadId = Math.random().toString(36);
        const promise = uploadFile(topicName, file)
          .finally(() => activeUploads.delete(uploadId));
        activeUploads.set(uploadId, promise);
      }

      if (activeUploads.size === 0) break;
      await Promise.race(activeUploads.values());
    }
  } finally {
    isUploading.value = false;
    _cancelRequested = false;
    flushUIUpdate();
  }
}

export function clearQueue() {
  if (isUploading.value) return;
  _stats = { total: 0, added: 0, skipped: 0, errors: 0 };
  _displayItems = [];
  flushUIUpdate();
}

export function cancelUpload() {
  _cancelRequested = true;
}

// Computed for compatibility with existing UI
export const uploadStats = computed(() => {
  const stats = totalStats.value;
  const completed = stats.added + stats.skipped + stats.errors;
  return {
    total: stats.total,
    pending: Math.max(0, stats.total - completed),
    success: stats.added,
    skipped: stats.skipped,
    errors: stats.errors,
  };
});

export const uploadProgress = computed(() => {
  const stats = totalStats.value;
  const completed = stats.added + stats.skipped + stats.errors;
  return stats.total > 0 ? Math.round((completed / stats.total) * 100) : 0;
});
