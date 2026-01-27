import { signal } from '@preact/signals';
import { TOAST_TYPES, TOAST_DEFAULT_DURATION } from '@constants/ui.js';

// Toast notification - stores { message, type } object
export const toastState = signal(null);
let toastTimeout = null;

/**
 * Show a toast notification
 * @param {string} message - The message to display
 * @param {string} type - Toast type: 'info' | 'success' | 'warning' | 'error'
 * @param {number} duration - Duration in ms before auto-dismiss
 */
export function showToast(message, type = TOAST_TYPES.info, duration = TOAST_DEFAULT_DURATION) {
  if (toastTimeout) clearTimeout(toastTimeout);
  toastState.value = { message, type };
  toastTimeout = setTimeout(() => {
    toastState.value = null;
    toastTimeout = null;
  }, duration);
}

// Modal state helper
export function createModal(name) {
  const isOpen = signal(false);

  return {
    isOpen,
    open: () => { isOpen.value = true; },
    close: () => { isOpen.value = false; },
    toggle: () => { isOpen.value = !isOpen.value; },
  };
}

// Create topic modal
export const createTopicModal = createModal('create-topic');
