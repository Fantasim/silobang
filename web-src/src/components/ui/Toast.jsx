import { toastState } from '@store/ui';
import { useEffect, useState } from 'preact/hooks';
import { TOAST_TYPES } from '@constants/ui.js';

/**
 * Toast notification component.
 * Displays brief messages at the bottom of the screen.
 * Supports types: info, success, warning, error
 */
export function Toast() {
  const toast = toastState.value;
  const [displayContent, setDisplayContent] = useState(null);
  const [displayType, setDisplayType] = useState(TOAST_TYPES.info);
  const [isHiding, setIsHiding] = useState(false);

  useEffect(() => {
    if (toast) {
      setDisplayContent(toast.message);
      setDisplayType(toast.type || TOAST_TYPES.info);
      setIsHiding(false);
    } else {
      setIsHiding(true);
      const timer = setTimeout(() => {
        setDisplayContent(null);
      }, 300);
      return () => clearTimeout(timer);
    }
  }, [toast]);

  const classes = [
    'cp-toast',
    toast && !isHiding ? 'show' : '',
    displayType,
  ].filter(Boolean).join(' ');

  return (
    <div class={classes}>
      {displayContent}
    </div>
  );
}
