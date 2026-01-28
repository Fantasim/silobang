import { useState, useRef, useCallback, useLayoutEffect } from 'preact/hooks';
import { createPortal } from 'preact/compat';

// Delay constants (ms)
const SHOW_DELAY_MS = 150;
const HIDE_DELAY_MS = 100;

// Positioning constants
const TOOLTIP_PADDING = 10;
const SCREEN_PADDING = 12;

/**
 * Calculate best tooltip position based on available space.
 * Shared logic with SmartTooltip but with larger padding for the panel.
 */
function calculatePosition(triggerRect, tooltipRect, preferredPosition) {
  const viewportWidth = window.innerWidth;
  const viewportHeight = window.innerHeight;

  const spaceTop = triggerRect.top;
  const spaceBottom = viewportHeight - triggerRect.bottom;
  const spaceLeft = triggerRect.left;
  const spaceRight = viewportWidth - triggerRect.right;

  const canFitTop = spaceTop >= tooltipRect.height + TOOLTIP_PADDING + SCREEN_PADDING;
  const canFitBottom = spaceBottom >= tooltipRect.height + TOOLTIP_PADDING + SCREEN_PADDING;
  const canFitLeft = spaceLeft >= tooltipRect.width + TOOLTIP_PADDING + SCREEN_PADDING;
  const canFitRight = spaceRight >= tooltipRect.width + TOOLTIP_PADDING + SCREEN_PADDING;

  let placement;

  if (preferredPosition === 'top' && canFitTop) placement = 'top';
  else if (preferredPosition === 'bottom' && canFitBottom) placement = 'bottom';
  else if (preferredPosition === 'left' && canFitLeft) placement = 'left';
  else if (preferredPosition === 'right' && canFitRight) placement = 'right';
  else if (canFitRight) placement = 'right';
  else if (canFitLeft) placement = 'left';
  else if (canFitBottom) placement = 'bottom';
  else if (canFitTop) placement = 'top';
  else placement = preferredPosition;

  let x, y;

  switch (placement) {
    case 'top':
      x = triggerRect.left + triggerRect.width / 2 - tooltipRect.width / 2;
      y = triggerRect.top - tooltipRect.height - TOOLTIP_PADDING;
      break;
    case 'bottom':
      x = triggerRect.left + triggerRect.width / 2 - tooltipRect.width / 2;
      y = triggerRect.bottom + TOOLTIP_PADDING;
      break;
    case 'left':
      x = triggerRect.left - tooltipRect.width - TOOLTIP_PADDING;
      y = triggerRect.top + triggerRect.height / 2 - tooltipRect.height / 2;
      break;
    case 'right':
      x = triggerRect.right + TOOLTIP_PADDING;
      y = triggerRect.top + triggerRect.height / 2 - tooltipRect.height / 2;
      break;
  }

  // Clamp to viewport bounds
  x = Math.max(SCREEN_PADDING, Math.min(x, viewportWidth - tooltipRect.width - SCREEN_PADDING));
  y = Math.max(SCREEN_PADDING, Math.min(y, viewportHeight - tooltipRect.height - SCREEN_PADDING));

  return { x, y, placement };
}

/**
 * Get the bounding rect of an element, handling display:contents wrappers.
 */
function getTriggerRect(element) {
  if (!element) return null;

  const rect = element.getBoundingClientRect();
  if (rect.width === 0 && rect.height === 0 && element.firstElementChild) {
    return element.firstElementChild.getBoundingClientRect();
  }
  return rect;
}

/**
 * Hover-triggered detail panel with rich content support.
 * Uses Portal rendering and smart positioning like SmartTooltip,
 * but supports arbitrary children content with a header/body layout.
 *
 * The panel stays open when the mouse moves from the trigger to the panel itself,
 * allowing users to interact with the tooltip content.
 *
 * @param {string} title - Header text displayed in cyan
 * @param {preact.ComponentChildren} children - Panel body content
 * @param {preact.ComponentChildren} trigger - The trigger element that activates the tooltip
 * @param {'top'|'bottom'|'left'|'right'} preferredPosition - Preferred placement (auto-flips if needed)
 */
export function DetailTooltip({
  title,
  children,
  trigger,
  preferredPosition = 'right',
}) {
  const [visible, setVisible] = useState(false);
  const [positioned, setPositioned] = useState(false);
  const [position, setPosition] = useState({ x: 0, y: 0, placement: preferredPosition });
  const triggerRef = useRef(null);
  const panelRef = useRef(null);
  const showTimerRef = useRef(null);
  const hideTimerRef = useRef(null);

  const clearTimers = useCallback(() => {
    if (showTimerRef.current) {
      clearTimeout(showTimerRef.current);
      showTimerRef.current = null;
    }
    if (hideTimerRef.current) {
      clearTimeout(hideTimerRef.current);
      hideTimerRef.current = null;
    }
  }, []);

  const startShow = useCallback(() => {
    clearTimers();
    showTimerRef.current = setTimeout(() => {
      setVisible(true);
    }, SHOW_DELAY_MS);
  }, [clearTimers]);

  const startHide = useCallback(() => {
    clearTimers();
    hideTimerRef.current = setTimeout(() => {
      setVisible(false);
      setPositioned(false);
    }, HIDE_DELAY_MS);
  }, [clearTimers]);

  const cancelHide = useCallback(() => {
    if (hideTimerRef.current) {
      clearTimeout(hideTimerRef.current);
      hideTimerRef.current = null;
    }
  }, []);

  const updatePosition = useCallback(() => {
    const triggerRect = getTriggerRect(triggerRef.current);
    if (!triggerRect || !panelRef.current) return;

    const panelRect = panelRef.current.getBoundingClientRect();
    const newPosition = calculatePosition(triggerRect, panelRect, preferredPosition);
    setPosition(newPosition);
    setPositioned(true);
  }, [preferredPosition]);

  useLayoutEffect(() => {
    if (visible) {
      setPositioned(false);
      updatePosition();
    }
  }, [visible, updatePosition]);

  // Trigger mouse handlers
  const onTriggerEnter = useCallback(() => {
    cancelHide();
    startShow();
  }, [cancelHide, startShow]);

  const onTriggerLeave = useCallback(() => {
    if (showTimerRef.current) {
      clearTimeout(showTimerRef.current);
      showTimerRef.current = null;
    }
    startHide();
  }, [startHide]);

  // Panel mouse handlers — keep panel open when hovering over it
  const onPanelEnter = useCallback(() => {
    cancelHide();
  }, [cancelHide]);

  const onPanelLeave = useCallback(() => {
    startHide();
  }, [startHide]);

  const panelContent = visible && (
    <div
      ref={panelRef}
      class={`detail-tooltip detail-tooltip-${position.placement}`}
      style={{
        position: 'fixed',
        left: `${position.x}px`,
        top: `${position.y}px`,
        visibility: positioned ? 'visible' : 'hidden',
      }}
      onMouseEnter={onPanelEnter}
      onMouseLeave={onPanelLeave}
    >
      {title && <div class="detail-tooltip-header">{title}</div>}
      <div class="detail-tooltip-body">
        {children}
      </div>
    </div>
  );

  return (
    <div
      ref={triggerRef}
      class="smart-tooltip-trigger"
      onMouseEnter={onTriggerEnter}
      onMouseLeave={onTriggerLeave}
    >
      {trigger}
      {createPortal(panelContent, document.body)}
    </div>
  );
}

/**
 * A single stat row for use inside DetailTooltip.
 * Provides consistent label/value layout matching the terminal design.
 *
 * @param {string} label - Left-aligned label text
 * @param {string|number} value - Right-aligned value
 * @param {string} color - Optional CSS color for the value
 */
export function DetailRow({ label, value, color }) {
  return (
    <div class="detail-tooltip-row">
      <span class="detail-tooltip-label">{label}</span>
      <span
        class="detail-tooltip-value"
        style={color ? { color } : undefined}
      >
        {value}
      </span>
    </div>
  );
}

/**
 * A section separator for grouping rows inside DetailTooltip.
 *
 * @param {string} label - Optional section label
 */
export function DetailSection({ label }) {
  return (
    <div class="detail-tooltip-section">
      {label && <span class="detail-tooltip-section-label">{label}</span>}
    </div>
  );
}
