import { selectedPreset, queryParams, setQueryParam } from '@store/query';

export function QueryForm() {
  const preset = selectedPreset.value;
  if (!preset || !preset.params || preset.params.length === 0) {
    return null;
  }

  // Infer input type from default value
  const inferType = (param) => {
    const defaultVal = param.default || '';

    // Check if default is a number
    if (/^\d+$/.test(defaultVal)) {
      return 'number';
    }

    // Check for date-like patterns
    if (/^\d{4}-\d{2}-\d{2}/.test(defaultVal)) {
      return 'date';
    }

    return 'text';
  };

  return (
    <div class="query-params">
      {preset.params.map(param => (
        <div key={param.name} class="query-param">
          <label>
            {param.name}
            {param.required && <span style={{ color: 'var(--terminal-red)' }}> *</span>}
          </label>
          <input
            type={inferType(param)}
            value={queryParams.value[param.name] || ''}
            placeholder={param.default || ''}
            onChange={(e) => setQueryParam(param.name, e.target.value)}
          />
        </div>
      ))}
    </div>
  );
}
