import { useState, useEffect, useMemo } from 'preact/hooks';
import {
  AUTH_CONSTRAINT_FIELDS, USERS_NO_CONSTRAINTS_TEXT,
  CONSTRAINT_SUGGEST, BYTE_UNITS, BYTE_UNIT_DEFAULT,
  ALL_AUTH_ACTIONS, AUTH_ACTION_LABELS,
} from '@constants/auth';
import { ToggleSwitch } from '@components/ui/ToggleSwitch';
import { TagInput } from '@components/ui/TagInput';
import { api } from '@services/api';

// =============================================================================
// Module-level suggestion caches — persist across mounts until page reload
// =============================================================================

let _topicsCache = null;
let _presetsCache = null;

// =============================================================================
// useSuggestions — fetches topic/preset lists with module-level caching
// =============================================================================

function useSuggestions(fields) {
  const [topicSuggestions, setTopicSuggestions] = useState(_topicsCache);
  const [presetSuggestions, setPresetSuggestions] = useState(_presetsCache);
  const [loading, setLoading] = useState({ topics: false, presets: false });

  const needsTopics = fields.some((f) => f.suggest === CONSTRAINT_SUGGEST.TOPICS);
  const needsPresets = fields.some((f) => f.suggest === CONSTRAINT_SUGGEST.PRESETS);

  useEffect(() => {
    if (needsTopics && !_topicsCache) {
      setLoading((prev) => ({ ...prev, topics: true }));
      api.getTopics()
        .then((data) => {
          const mapped = (data.topics || []).map((t) => ({ value: t.name, label: t.name }));
          _topicsCache = mapped;
          setTopicSuggestions(mapped);
        })
        .catch(() => {
          _topicsCache = [];
          setTopicSuggestions([]);
        })
        .finally(() => setLoading((prev) => ({ ...prev, topics: false })));
    }
  }, [needsTopics]);

  useEffect(() => {
    if (needsPresets && !_presetsCache) {
      setLoading((prev) => ({ ...prev, presets: true }));
      api.getQueries()
        .then((data) => {
          const mapped = (data.presets || []).map((p) => ({
            value: p.name,
            label: p.name,
            description: p.description,
          }));
          _presetsCache = mapped;
          setPresetSuggestions(mapped);
        })
        .catch(() => {
          _presetsCache = [];
          setPresetSuggestions([]);
        })
        .finally(() => setLoading((prev) => ({ ...prev, presets: false })));
    }
  }, [needsPresets]);

  // Actions are static — derived from constants, no fetch needed
  const actionSuggestions = useMemo(() =>
    ALL_AUTH_ACTIONS.map((a) => ({
      value: a,
      label: AUTH_ACTION_LABELS[a] || a,
    })),
    []
  );

  return {
    getSuggestions(suggestType) {
      if (suggestType === CONSTRAINT_SUGGEST.TOPICS) return topicSuggestions;
      if (suggestType === CONSTRAINT_SUGGEST.PRESETS) return presetSuggestions;
      if (suggestType === CONSTRAINT_SUGGEST.ACTIONS) return actionSuggestions;
      return null;
    },
    isLoading(suggestType) {
      if (suggestType === CONSTRAINT_SUGGEST.TOPICS) return loading.topics;
      if (suggestType === CONSTRAINT_SUGGEST.PRESETS) return loading.presets;
      return false;
    },
  };
}

// =============================================================================
// ByteInput — Number + unit selector for byte values
// =============================================================================

function decomposeBytes(rawBytes) {
  if (!rawBytes || rawBytes === 0) {
    return { amount: '', unit: BYTE_UNIT_DEFAULT };
  }
  // Find the largest unit that divides evenly (or fall back to bytes)
  for (let i = BYTE_UNITS.length - 1; i >= 1; i--) {
    const u = BYTE_UNITS[i];
    if (rawBytes >= u.value && rawBytes % u.value === 0) {
      return { amount: rawBytes / u.value, unit: u.label };
    }
  }
  return { amount: rawBytes, unit: 'B' };
}

function ByteInput({ value, onChange, disabled }) {
  const decomposed = decomposeBytes(value);
  const [amount, setAmount] = useState(decomposed.amount);
  const [unit, setUnit] = useState(decomposed.unit);

  // Sync when value changes externally (e.g. parent resets)
  useEffect(() => {
    const d = decomposeBytes(value);
    setAmount(d.amount);
    setUnit(d.unit);
  }, [value]);

  const recompose = (newAmount, newUnit) => {
    if (newAmount === '' || newAmount === undefined) {
      onChange(undefined);
      return;
    }
    const unitDef = BYTE_UNITS.find((u) => u.label === newUnit);
    if (unitDef) {
      onChange(Number(newAmount) * unitDef.value);
    }
  };

  const handleAmountChange = (e) => {
    const v = e.target.value;
    setAmount(v);
    recompose(v === '' ? '' : Number(v), unit);
  };

  const handleUnitChange = (e) => {
    const newUnit = e.target.value;
    setUnit(newUnit);
    recompose(amount === '' ? '' : Number(amount), newUnit);
  };

  return (
    <div class="byte-input">
      <input
        type="number"
        class="byte-input-number"
        value={amount}
        placeholder="No limit"
        min={0}
        disabled={disabled}
        onInput={handleAmountChange}
      />
      <select
        class="byte-input-unit"
        value={unit}
        onChange={handleUnitChange}
        disabled={disabled}
      >
        {BYTE_UNITS.map((u) => (
          <option key={u.label} value={u.label}>{u.label}</option>
        ))}
      </select>
    </div>
  );
}

// =============================================================================
// ConstraintEditor — Main component
// =============================================================================

/**
 * ConstraintEditor - Visual form editor for grant constraints.
 * Reads AUTH_CONSTRAINT_FIELDS[action] and renders typed form inputs
 * instead of a raw JSON textarea.
 *
 * @param {Object} props
 * @param {string} props.action - The auth action (e.g., 'upload', 'download')
 * @param {Object} props.constraints - Current constraints as a plain JS object
 * @param {(constraints: Object) => void} props.onChange - Called when any field changes
 * @param {boolean} [props.disabled] - Whether the editor is read-only
 */
export function ConstraintEditor({ action, constraints = {}, onChange, disabled = false }) {
  const fields = AUTH_CONSTRAINT_FIELDS[action] || [];
  const { getSuggestions, isLoading } = useSuggestions(fields);

  if (fields.length === 0) {
    return (
      <p class="constraint-editor-empty">
        {USERS_NO_CONSTRAINTS_TEXT}
      </p>
    );
  }

  const updateField = (key, value) => {
    const next = { ...constraints };
    // Remove undefined/null/empty values to keep JSON clean
    if (value === undefined || value === null || value === '' ||
        (Array.isArray(value) && value.length === 0) ||
        value === false) {
      delete next[key];
    } else {
      next[key] = value;
    }
    onChange(next);
  };

  return (
    <div class="constraint-editor-form">
      {fields.map((field) => (
        <div key={field.key} class="constraint-field">
          {field.type === 'boolean' && (
            <div class="constraint-field-row constraint-field-row--toggle">
              <label class="constraint-field-label">{field.label}</label>
              <ToggleSwitch
                checked={constraints[field.key] === true}
                onChange={(checked) => updateField(field.key, checked || undefined)}
                disabled={disabled}
              />
            </div>
          )}

          {field.type === 'number' && (
            <div class="constraint-field-row">
              <label class="constraint-field-label">{field.label}</label>
              <input
                type="number"
                class="constraint-field-number user-field-input"
                value={constraints[field.key] ?? ''}
                placeholder="No limit"
                min={0}
                disabled={disabled}
                onInput={(e) => {
                  const v = e.target.value;
                  updateField(field.key, v === '' ? undefined : Number(v));
                }}
              />
            </div>
          )}

          {field.type === 'bytes' && (
            <div class="constraint-field-row">
              <label class="constraint-field-label">{field.label}</label>
              <ByteInput
                value={constraints[field.key]}
                onChange={(v) => updateField(field.key, v)}
                disabled={disabled}
              />
            </div>
          )}

          {field.type === 'string_array' && (
            <div class="constraint-field-row constraint-field-row--stacked">
              <label class="constraint-field-label">{field.label}</label>
              <TagInput
                values={constraints[field.key] || []}
                placeholder={field.placeholder}
                onChange={(vals) => updateField(field.key, vals.length > 0 ? vals : undefined)}
                disabled={disabled}
                suggestions={field.suggest ? getSuggestions(field.suggest) : undefined}
                suggestionsLoading={field.suggest ? isLoading(field.suggest) : false}
              />
            </div>
          )}
        </div>
      ))}
    </div>
  );
}
