import { createPortal } from 'preact/compat';
import { useEffect, useState } from 'preact/hooks';

/**
 * Portal component that renders children at the root level of the DOM.
 * This escapes any parent stacking contexts, ensuring overlays/dropdowns
 * always render above other content regardless of parent z-index.
 *
 * @param {Object} props
 * @param {preact.ComponentChildren} props.children - Content to render in portal
 * @param {string} props.containerId - Optional custom container ID (default: 'portal-root')
 */
export function Portal({ children, containerId = 'portal-root' }) {
  const [container, setContainer] = useState(null);

  useEffect(() => {
    // Find or create the portal container
    let portalContainer = document.getElementById(containerId);

    if (!portalContainer) {
      portalContainer = document.createElement('div');
      portalContainer.id = containerId;
      document.body.appendChild(portalContainer);
    }

    setContainer(portalContainer);

    // Cleanup: remove container if empty when component unmounts
    return () => {
      if (portalContainer && portalContainer.childNodes.length === 0) {
        portalContainer.remove();
      }
    };
  }, [containerId]);

  if (!container) return null;

  return createPortal(children, container);
}
