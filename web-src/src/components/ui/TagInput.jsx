import { useState } from 'preact/hooks';
import { Icon } from './Icon';

/**
 * TagInput - String array input rendered as editable tag chips.
 * Type a value and press Enter (or comma) to add as a tag.
 * Click X on a tag to remove it. Backspace removes the last tag when input is empty.
 *
 * Optionally displays clickable suggestion chips above the input area.
 * Clicking a suggestion toggles it in/out of the selected values.
 *
 * @param {Object} props
 * @param {string[]} props.values - Current array of values
 * @param {(values: string[]) => void} props.onChange - Called when values change
 * @param {string} [props.placeholder] - Placeholder when no values
 * @param {boolean} [props.disabled] - Whether the input is disabled
 * @param {string} [props.className] - Optional additional class
 * @param {Array<{value: string, label: string, description?: string}>} [props.suggestions] - Clickable suggestion chips
 * @param {boolean} [props.suggestionsLoading] - Show loading state for suggestions
 */
export function TagInput({
  values, onChange, placeholder = '', disabled = false, className = '',
  suggestions, suggestionsLoading = false,
}) {
  const [inputValue, setInputValue] = useState('');

  const addValue = (raw) => {
    const val = raw.trim();
    if (val && !values.includes(val)) {
      onChange([...values, val]);
    }
    setInputValue('');
  };

  const removeValue = (index) => {
    onChange(values.filter((_, i) => i !== index));
  };

  const toggleSuggestion = (suggestionValue) => {
    if (disabled) return;
    if (values.includes(suggestionValue)) {
      onChange(values.filter((v) => v !== suggestionValue));
    } else {
      onChange([...values, suggestionValue]);
    }
  };

  const handleKeyDown = (e) => {
    if (e.key === 'Enter' || e.key === ',') {
      e.preventDefault();
      if (inputValue.trim()) {
        addValue(inputValue);
      }
    }
    if (e.key === 'Backspace' && inputValue === '' && values.length > 0) {
      removeValue(values.length - 1);
    }
  };

  const handleInput = (e) => {
    const val = e.target.value;
    // Auto-add on comma
    if (val.includes(',')) {
      const parts = val.split(',');
      parts.forEach((part, i) => {
        if (i < parts.length - 1 && part.trim()) {
          addValue(part);
        }
      });
      setInputValue(parts[parts.length - 1]);
    } else {
      setInputValue(val);
    }
  };

  const hasSuggestions = suggestions && suggestions.length > 0;
  const containerClasses = [
    'tag-input',
    hasSuggestions && 'tag-input--with-suggestions',
    disabled && 'tag-input--disabled',
    className,
  ].filter(Boolean).join(' ');

  return (
    <div class={containerClasses}>
      {/* Suggestion chips */}
      {(hasSuggestions || suggestionsLoading) && (
        <div class="tag-input-suggestions">
          {suggestionsLoading ? (
            <span class="tag-input-suggestions-loading">Loading...</span>
          ) : suggestions.length === 0 ? (
            <span class="tag-input-suggestions-empty">No suggestions available</span>
          ) : (
            suggestions.map((s) => {
              const isSelected = values.includes(s.value);
              return (
                <button
                  key={s.value}
                  type="button"
                  class={`tag-input-suggestion ${isSelected ? 'tag-input-suggestion--selected' : ''}`}
                  onClick={() => toggleSuggestion(s.value)}
                  disabled={disabled}
                  title={s.description || s.label}
                >
                  {s.label}
                  {isSelected && <Icon name="check" size={10} />}
                </button>
              );
            })
          )}
        </div>
      )}

      {/* Selected tags + text input area */}
      <div class="tag-input-area" onClick={(e) => {
        const input = e.currentTarget.querySelector('.tag-input-field');
        if (input) input.focus();
      }}>
        {values.length > 0 && (
          <div class="tag-input-tags">
            {values.map((val, i) => (
              <span key={`${val}-${i}`} class="tag-input-tag">
                <span class="tag-input-tag-text">{val}</span>
                {!disabled && (
                  <button
                    type="button"
                    class="tag-input-tag-remove"
                    onClick={(e) => { e.stopPropagation(); removeValue(i); }}
                    title={`Remove "${val}"`}
                  >
                    <Icon name="close" size={10} />
                  </button>
                )}
              </span>
            ))}
          </div>
        )}
        <input
          type="text"
          class="tag-input-field"
          placeholder={values.length === 0 ? placeholder : ''}
          value={inputValue}
          onInput={handleInput}
          onKeyDown={handleKeyDown}
          disabled={disabled}
        />
      </div>
    </div>
  );
}
