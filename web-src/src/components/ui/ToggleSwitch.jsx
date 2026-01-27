/**
 * ToggleSwitch - Terminal-styled boolean toggle.
 *
 * @param {Object} props
 * @param {boolean} props.checked - Whether the toggle is on
 * @param {(checked: boolean) => void} props.onChange - Called when toggled
 * @param {boolean} [props.disabled] - Whether the toggle is disabled
 * @param {string} [props.className] - Optional additional class
 */
export function ToggleSwitch({ checked, onChange, disabled = false, className = '' }) {
  const classes = [
    'toggle-switch',
    checked && 'toggle-switch--active',
    disabled && 'toggle-switch--disabled',
    className,
  ].filter(Boolean).join(' ');

  const handleClick = () => {
    if (!disabled) {
      onChange(!checked);
    }
  };

  const handleKeyDown = (e) => {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      handleClick();
    }
  };

  return (
    <button
      type="button"
      class={classes}
      role="switch"
      aria-checked={checked}
      onClick={handleClick}
      onKeyDown={handleKeyDown}
      disabled={disabled}
    >
      <span class="toggle-switch-thumb" />
    </button>
  );
}
