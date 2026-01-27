import { useState, useRef, useEffect, useLayoutEffect, useCallback } from 'preact/hooks';
import { createPortal } from 'preact/compat';

/**
 * Get the bounding rect of an element, handling display:contents wrappers
 * by looking at the first actual child element.
 */
function getTriggerRect(element) {
  if (!element) return null;

  // If the element has no dimensions (display: contents), find first child with dimensions
  const rect = element.getBoundingClientRect();
  if (rect.width === 0 && rect.height === 0 && element.firstElementChild) {
    return element.firstElementChild.getBoundingClientRect();
  }
  return rect;
}

/**
 * Calculate best tooltip position based on available space
 */
function calculateBestPosition(triggerRect, tooltipRect, preferredPosition, padding, screenPadding) {
  const viewportWidth = window.innerWidth;
  const viewportHeight = window.innerHeight;

  const spaceTop = triggerRect.top;
  const spaceBottom = viewportHeight - triggerRect.bottom;
  const spaceLeft = triggerRect.left;
  const spaceRight = viewportWidth - triggerRect.right;

  const canFitTop = spaceTop >= tooltipRect.height + padding + screenPadding;
  const canFitBottom = spaceBottom >= tooltipRect.height + padding + screenPadding;
  const canFitLeft = spaceLeft >= tooltipRect.width + padding + screenPadding;
  const canFitRight = spaceRight >= tooltipRect.width + padding + screenPadding;

  let placement;

  // Try preferred position first, then fallback intelligently
  if (preferredPosition === 'top' && canFitTop) {
    placement = 'top';
  } else if (preferredPosition === 'bottom' && canFitBottom) {
    placement = 'bottom';
  } else if (preferredPosition === 'left' && canFitLeft) {
    placement = 'left';
  } else if (preferredPosition === 'right' && canFitRight) {
    placement = 'right';
  } else if (canFitRight) {
    placement = 'right';
  } else if (canFitLeft) {
    placement = 'left';
  } else if (canFitTop) {
    placement = 'top';
  } else if (canFitBottom) {
    placement = 'bottom';
  } else {
    placement = preferredPosition; // Last resort
  }

  let x, y;

  switch (placement) {
    case 'top':
      x = triggerRect.left + triggerRect.width / 2 - tooltipRect.width / 2;
      y = triggerRect.top - tooltipRect.height - padding;
      break;
    case 'bottom':
      x = triggerRect.left + triggerRect.width / 2 - tooltipRect.width / 2;
      y = triggerRect.bottom + padding;
      break;
    case 'left':
      x = triggerRect.left - tooltipRect.width - padding;
      y = triggerRect.top + triggerRect.height / 2 - tooltipRect.height / 2;
      break;
    case 'right':
      x = triggerRect.right + padding;
      y = triggerRect.top + triggerRect.height / 2 - tooltipRect.height / 2;
      break;
  }

  // Clamp to viewport bounds
  x = Math.max(screenPadding, Math.min(x, viewportWidth - tooltipRect.width - screenPadding));
  y = Math.max(screenPadding, Math.min(y, viewportHeight - tooltipRect.height - screenPadding));

  return { x, y, placement };
}

/**
 * Smart Tooltip component that automatically positions itself to avoid screen overflow.
 * Uses portal rendering to escape parent stacking contexts and ensure visibility.
 *
 * @param {string} title - Optional header text (cyan color)
 * @param {string} description - Optional description text (dim italic)
 * @param {Array<{label: string, value: string|number, color?: string}>} rows - Stats rows
 * @param {ReactNode} children - Trigger element
 * @param {'top'|'bottom'|'left'|'right'} preferredPosition - Preferred position (will flip if needed)
 */
export function SmartTooltip({
  title,
  description,
  rows = [],
  children,
  preferredPosition = 'right',
}) {
  const [visible, setVisible] = useState(false);
  const [positioned, setPositioned] = useState(false);
  const [position, setPosition] = useState({ x: 0, y: 0, placement: preferredPosition });
  const triggerRef = useRef(null);
  const tooltipRef = useRef(null);

  const updatePosition = useCallback(() => {
    const triggerRect = getTriggerRect(triggerRef.current);
    if (!triggerRect || !tooltipRef.current) return;

    const tooltipRect = tooltipRef.current.getBoundingClientRect();
    const newPosition = calculateBestPosition(triggerRect, tooltipRect, preferredPosition, 8, 12);
    setPosition(newPosition);
    setPositioned(true);
  }, [preferredPosition]);

  useLayoutEffect(() => {
    if (visible) {
      setPositioned(false);
      updatePosition();
    }
  }, [visible, updatePosition]);

  const tooltipContent = visible && (
    <div
      ref={tooltipRef}
      class={`smart-tooltip smart-tooltip-${position.placement}`}
      style={{
        position: 'fixed',
        left: `${position.x}px`,
        top: `${position.y}px`,
        visibility: positioned ? 'visible' : 'hidden',
      }}
    >
      {title && <div class="smart-tooltip-header">{title}</div>}
      {description && <div class="smart-tooltip-desc">{description}</div>}
      {rows.length > 0 && (
        <div class="smart-tooltip-content">
          {rows.map((row, i) => (
            <div key={i} class="smart-tooltip-row">
              <span class="smart-tooltip-label">{row.label}</span>
              <span
                class="smart-tooltip-value"
                style={row.color ? { color: row.color } : undefined}
              >
                {row.value}
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  );

  return (
    <div
      ref={triggerRef}
      class="smart-tooltip-trigger"
      onMouseEnter={() => setVisible(true)}
      onMouseLeave={() => setVisible(false)}
    >
      {children}
      {createPortal(tooltipContent, document.body)}
    </div>
  );
}

/**
 * Simple smart tooltip for basic text hints.
 * Automatically positions to avoid screen overflow.
 *
 * @param {string} text - Tooltip text
 * @param {ReactNode} children - Trigger element
 * @param {'top'|'bottom'|'left'|'right'} preferredPosition - Preferred position
 */
export function SimpleTooltip({ text, children, preferredPosition = 'top' }) {
  const [visible, setVisible] = useState(false);
  const [positioned, setPositioned] = useState(false);
  const [position, setPosition] = useState({ x: 0, y: 0, placement: preferredPosition });
  const triggerRef = useRef(null);
  const tooltipRef = useRef(null);

  const updatePosition = useCallback(() => {
    const triggerRect = getTriggerRect(triggerRef.current);
    if (!triggerRect || !tooltipRef.current) return;

    const tooltipRect = tooltipRef.current.getBoundingClientRect();
    const newPosition = calculateBestPosition(triggerRect, tooltipRect, preferredPosition, 6, 8);
    setPosition(newPosition);
    setPositioned(true);
  }, [preferredPosition]);

  useLayoutEffect(() => {
    if (visible) {
      setPositioned(false);
      updatePosition();
    }
  }, [visible, updatePosition]);

  const tooltipContent = visible && (
    <div
      ref={tooltipRef}
      class={`simple-smart-tooltip simple-smart-tooltip-${position.placement}`}
      style={{
        position: 'fixed',
        left: `${position.x}px`,
        top: `${position.y}px`,
        visibility: positioned ? 'visible' : 'hidden',
      }}
    >
      {text}
    </div>
  );

  return (
    <div
      ref={triggerRef}
      class="smart-tooltip-trigger"
      onMouseEnter={() => setVisible(true)}
      onMouseLeave={() => setVisible(false)}
    >
      {children}
      {createPortal(tooltipContent, document.body)}
    </div>
  );
}
