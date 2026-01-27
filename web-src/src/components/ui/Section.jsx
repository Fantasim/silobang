import { sectionStates, toggleSection } from '@store/ui';

/**
 * Collapsible section component.
 *
 * @param {string} id - Section identifier for state management
 * @param {string} title - Section title
 * @param {ReactNode} children - Section content
 * @param {boolean} defaultOpen - Initial open state (if not in stored state)
 */
export function Section({ id, title, children, defaultOpen = true }) {
  // Use stored state if available, otherwise use defaultOpen
  const isOpen = sectionStates.value[id] ?? defaultOpen;

  const handleToggle = () => {
    toggleSection(id);
  };

  return (
    <section class={`cp-section ${isOpen ? '' : 'cp-section-collapsed'}`}>
      <div class="cp-section-header" onClick={handleToggle}>
        <span class="cp-section-title">{title}</span>
        <span class="cp-section-toggle">{isOpen ? '−' : '+'}</span>
      </div>
      {isOpen && <div class="cp-section-content">{children}</div>}
    </section>
  );
}
