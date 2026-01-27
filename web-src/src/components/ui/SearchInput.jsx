import { Icon } from './Icon';

/**
 * SearchInput - Unified search input with icon, input field, and clear button.
 * Used in TagSettings and FilterSearchBar.
 *
 * @param {Object} props
 * @param {string} props.value - Controlled input value
 * @param {(value: string) => void} props.onChange - Called on input change
 * @param {() => void} props.onClear - Called when clear button clicked
 * @param {string} [props.placeholder='Search...'] - Input placeholder text
 * @param {string} [props.className] - Optional additional class
 * @param {'sm' | 'md'} [props.size='md'] - Icon size (sm: 14px, md: 16px)
 */
export function SearchInput({
  value,
  onChange,
  onClear,
  placeholder = 'Search...',
  className = '',
  size = 'md',
}) {
  const iconSize = size === 'sm' ? 14 : 16;

  const handleInput = (e) => {
    onChange(e.target.value);
  };

  const handleKeyDown = (e) => {
    if (e.key === 'Escape') {
      onClear();
      e.target.blur();
    }
  };

  const containerClass = ['search-input-container', className].filter(Boolean).join(' ');

  return (
    <div class={containerClass}>
      <span class="search-input-icon">
        <Icon name="search" size={iconSize} />
      </span>
      <input
        type="text"
        class="search-input-field"
        placeholder={placeholder}
        value={value}
        onInput={handleInput}
        onKeyDown={handleKeyDown}
      />
      {value && (
        <button
          class="search-input-clear"
          onClick={onClear}
          title="Clear search"
        >
          <Icon name="close" size={iconSize} />
        </button>
      )}
    </div>
  );
}
