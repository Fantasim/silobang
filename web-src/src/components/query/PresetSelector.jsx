import { useState } from 'preact/hooks';
import { Button } from '@components/ui/Button';
import { Spinner } from '@components/ui/Spinner';
import {
  presets,
  selectedPreset,
  selectPreset,
  queryParams,
  setQueryParam,
  selectedTopics,
  queryLoading,
  runQuery,
  presetSelectorExpanded,
  presetSearchQuery,
} from '@store/query';
import { healthyTopics } from '@store/topics';

export function PresetSelector({ hasResults = false }) {
  const preset = selectedPreset.value;
  const isExpanded = presetSelectorExpanded.value;
  const topics = healthyTopics.value;
  const searchQuery = presetSearchQuery.value;

  // Toggle panel expansion
  const toggleExpanded = () => {
    presetSelectorExpanded.value = !presetSelectorExpanded.value;
  };

  // Close panel
  const closePanel = () => {
    presetSelectorExpanded.value = false;
  };

  // Toggle topic selection
  const toggleTopic = (topicName) => {
    const current = selectedTopics.value;
    if (current.includes(topicName)) {
      selectedTopics.value = current.filter(t => t !== topicName);
    } else {
      selectedTopics.value = [...current, topicName];
    }
  };

  // Select all healthy topics
  const selectAllTopics = () => {
    selectedTopics.value = topics.map(t => t.name);
  };

  // Clear all topics
  const clearAllTopics = () => {
    selectedTopics.value = [];
  };

  // Handle preset selection
  const handleSelectPreset = (presetName) => {
    selectPreset(presetName);
  };

  // Infer input type from default value
  const inferType = (param) => {
    const defaultVal = param.default || '';

    if (/^\d+$/.test(defaultVal)) {
      return 'number';
    }

    if (/^\d{4}-\d{2}-\d{2}/.test(defaultVal)) {
      return 'date';
    }

    return 'text';
  };

  // Filter presets based on search query
  const filteredPresets = presets.value.filter(p => {
    if (!searchQuery) return true;
    const query = searchQuery.toLowerCase();
    return p.name.toLowerCase().includes(query) ||
           p.description.toLowerCase().includes(query);
  });

  // Get topic filter summary
  const getTopicSummary = () => {
    if (selectedTopics.value.length === 0) {
      return 'All topics';
    }
    if (selectedTopics.value.length === 1) {
      return selectedTopics.value[0];
    }
    return `${selectedTopics.value.length} topics`;
  };

  // Compact bar - always shown
  const renderCompactBar = () => (
    <div class="preset-compact-bar" onClick={toggleExpanded}>
      <div class="preset-compact-bar-left">
        <span class="preset-compact-bar-icon">{isExpanded ? '\u25BC' : '\u25B6'}</span>
        {preset ? (
          <>
            <span class="preset-compact-bar-name">{preset.name}</span>
            <div class="preset-compact-bar-params">
              {preset.params?.map(param => (
                <span key={param.name} class="preset-compact-bar-param">
                  <span class="preset-compact-bar-param-name">{param.name}:</span>
                  <span class="preset-compact-bar-param-value">
                    {queryParams.value[param.name] || param.default || '-'}
                  </span>
                </span>
              ))}
            </div>
            <span class="preset-compact-bar-topics">{getTopicSummary()}</span>
          </>
        ) : (
          <span class="preset-compact-bar-placeholder">Select a query preset...</span>
        )}
      </div>
      <div class="preset-compact-bar-right" onClick={(e) => e.stopPropagation()}>
        <Button
          onClick={runQuery}
          disabled={queryLoading.value || !preset}
        >
          {queryLoading.value ? <Spinner /> : 'Run Query'}
        </Button>
      </div>
    </div>
  );

  // Expandable selection panel
  const renderExpandedPanel = () => {
    if (!isExpanded) return null;

    return (
      <div class="preset-expanded-panel">
        <div class="preset-expanded-panel-content">
          {/* Left: Preset list with search */}
          <div class="preset-expanded-list-section">
            <div class="preset-search-bar">
              <input
                type="text"
                class="preset-search-input"
                placeholder="Search presets..."
                value={searchQuery}
                onInput={(e) => presetSearchQuery.value = e.target.value}
              />
            </div>
            <div class="preset-cards-grid">
              {filteredPresets.map(p => (
                <button
                  key={p.name}
                  type="button"
                  class={`preset-card-compact ${preset?.name === p.name ? 'selected' : ''}`}
                  onClick={() => handleSelectPreset(p.name)}
                >
                  <div class="preset-card-compact-header">
                    <span class="preset-card-compact-name">{p.name}</span>
                    {preset?.name === p.name && (
                      <span class="preset-card-compact-check">{'\u2713'}</span>
                    )}
                  </div>
                  <p class="preset-card-compact-description">{p.description}</p>
                  <span class="preset-card-compact-badge">
                    {p.params && p.params.length > 0
                      ? `${p.params.length} param${p.params.length !== 1 ? 's' : ''}`
                      : 'No params'}
                  </span>
                </button>
              ))}
            </div>
          </div>

          {/* Right: Details/configuration panel */}
          <div class="preset-config-panel">
            {!preset ? (
              <div class="preset-config-empty">
                <div class="preset-config-empty-icon">{'\uD83D\uDCCB'}</div>
                <div class="preset-config-empty-text">Select a preset to configure</div>
                <div class="preset-config-empty-hint">
                  Choose from {presets.value.length} available query presets
                </div>
              </div>
            ) : (
              <>
                <div class="preset-config-header">
                  <div class="preset-config-name">{preset.name}</div>
                  <div class="preset-config-description">{preset.description}</div>
                </div>

                {/* Parameters */}
                {preset.params && preset.params.length > 0 ? (
                  <div class="preset-config-params">
                    <span class="preset-config-section-title">Parameters</span>
                    <div class="preset-config-params-grid">
                      {preset.params.map(param => (
                        <div key={param.name} class="preset-config-param">
                          <label>
                            {param.name}
                            {param.required && <span class="preset-config-param-required">*</span>}
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
                  </div>
                ) : (
                  <div class="preset-config-no-params">
                    <span class="preset-config-section-title">Parameters</span>
                    <div class="preset-config-no-params-text">No parameters required</div>
                  </div>
                )}

                {/* Topic filter */}
                <div class="preset-config-topics">
                  <div class="preset-config-topics-header">
                    <span class="preset-config-section-title">Scope to topics (optional)</span>
                    <div class="preset-config-topics-actions">
                      <button type="button" class="topic-action-link" onClick={selectAllTopics}>
                        All
                      </button>
                      <button type="button" class="topic-action-link" onClick={clearAllTopics}>
                        None
                      </button>
                    </div>
                  </div>
                  <div class="preset-config-topics-hint">
                    {selectedTopics.value.length === 0
                      ? 'Searches all available topics'
                      : `${selectedTopics.value.length} topic${selectedTopics.value.length !== 1 ? 's' : ''} selected`}
                  </div>
                  <div class="preset-config-topic-chips">
                    {topics.map(topic => (
                      <button
                        key={topic.name}
                        type="button"
                        class={`topic-chip ${selectedTopics.value.includes(topic.name) ? 'selected' : ''}`}
                        onClick={() => toggleTopic(topic.name)}
                      >
                        {topic.name}
                      </button>
                    ))}
                  </div>
                </div>

                {/* Actions */}
                <div class="preset-config-actions">
                  <Button
                    onClick={() => { runQuery(); closePanel(); }}
                    disabled={queryLoading.value}
                    style={{ flex: 1 }}
                  >
                    {queryLoading.value ? <Spinner /> : 'Run Query'}
                  </Button>
                  <Button
                    variant="ghost"
                    onClick={closePanel}
                  >
                    Close
                  </Button>
                </div>
              </>
            )}
          </div>
        </div>
      </div>
    );
  };

  return (
    <div class="preset-selector-v2">
      {renderCompactBar()}
      {renderExpandedPanel()}
    </div>
  );
}
