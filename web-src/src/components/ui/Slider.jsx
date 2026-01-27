/**
 * Range slider component.
 *
 * @param {number} value - Current value
 * @param {Function} onChange - Callback when value changes
 * @param {number} min - Minimum value
 * @param {number} max - Maximum value
 * @param {number} step - Step increment
 * @param {string} label - Optional label text
 * @param {Function} format - Optional formatter for display value
 * @param {string} className - Additional CSS classes
 */
export function Slider({
  value,
  onChange,
  min = 0,
  max = 1,
  step = 0.01,
  label,
  format,
  className = '',
}) {
  const displayValue = format ? format(value) : value.toFixed(2);

  return (
    <div class={`cp-row ${className}`}>
      {label && <label>{label}</label>}
      <div class="cp-slider-wrap">
        <input
          type="range"
          class="cp-slider"
          min={min}
          max={max}
          step={step}
          value={value}
          onInput={(e) => onChange(parseFloat(e.target.value))}
        />
        <span class="cp-slider-value">{displayValue}</span>
      </div>
    </div>
  );
}
