import { useEffect, useState, useMemo } from 'preact/hooks';
import { Button } from '@components/ui/Button';
import { Spinner } from '@components/ui/Spinner';
import { ErrorBanner } from '@components/ui/ErrorBanner';
import { api } from '@services/api';
import { showToast } from '@store/ui';
import { TOAST_TYPES } from '@constants/ui.js';

const CATEGORIES = [
  { id: 'all', label: 'All' },
  { id: 'metadata', label: 'Metadata' },
  { id: 'assets', label: 'Assets' },
];

export function ApiPage() {
  const [schema, setSchema] = useState(null);
  const [prompts, setPrompts] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [schemaExpanded, setSchemaExpanded] = useState(false);
  const [categoryFilter, setCategoryFilter] = useState('all');

  useEffect(() => {
    fetchData();
  }, []);

  const fetchData = async () => {
    setLoading(true);
    setError(null);
    try {
      const [schemaData, promptsData] = await Promise.all([
        api.getSchema(),
        api.getPrompts(),
      ]);
      setSchema(schemaData);
      setPrompts(promptsData.prompts || []);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const filteredPrompts = useMemo(() => {
    if (categoryFilter === 'all') return prompts;
    return prompts.filter(p => p.category === categoryFilter);
  }, [prompts, categoryFilter]);

  const groupedEndpoints = useMemo(() => {
    if (!schema?.endpoints) return {};
    const groups = {};
    for (const endpoint of schema.endpoints) {
      const category = endpoint.category || 'other';
      if (!groups[category]) groups[category] = [];
      groups[category].push(endpoint);
    }
    return groups;
  }, [schema]);

  const copyToClipboard = async (text, label) => {
    try {
      await navigator.clipboard.writeText(text);
      showToast(`${label} copied to clipboard`, TOAST_TYPES.success);
    } catch (err) {
      showToast('Failed to copy', TOAST_TYPES.error);
    }
  };

  const copySchemaAsMarkdown = () => {
    if (!schema?.endpoints) return;

    const lines = ['# MeshBank API Reference', '', `Base URL: ${schema.base_url}`, ''];

    const groups = groupedEndpoints;
    for (const [category, endpoints] of Object.entries(groups)) {
      lines.push(`## ${category.charAt(0).toUpperCase() + category.slice(1)}`, '');
      for (const ep of endpoints) {
        lines.push(`### ${ep.method} ${ep.path}`);
        lines.push(ep.description);
        lines.push('');
      }
    }

    copyToClipboard(lines.join('\n'), 'Schema markdown');
  };

  if (loading) {
    return (
      <div class="api-page">
        <div class="api-page-loading">
          <Spinner />
          <span>Loading API documentation...</span>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div class="api-page">
        <ErrorBanner message={error} />
        <Button onClick={fetchData}>Retry</Button>
      </div>
    );
  }

  return (
    <div class="api-page">
      {/* Prompt Templates */}
      <section class="api-section">
        <h2 class="api-section-title">AI Prompt Templates</h2>
        <p class="api-section-description">
          Copy a prompt, add your context at the bottom, and paste into ChatGPT or Claude to generate a working script.
        </p>

        {/* Category Filter */}
        <div class="api-category-filter">
          {CATEGORIES.map(cat => (
            <Button
              key={cat.id}
              variant={categoryFilter === cat.id ? 'default' : 'ghost'}
              onClick={() => setCategoryFilter(cat.id)}
            >
              {cat.label}
            </Button>
          ))}
        </div>

        {/* Expanded Prompts List */}
        <div class="api-prompts-list">
          {filteredPrompts.map(prompt => (
            <div key={prompt.name} class="api-prompt-expanded">
              <div class="api-prompt-expanded-header">
                <div class="api-prompt-expanded-info">
                  <span class="api-prompt-expanded-name">{prompt.name}</span>
                  <span class="api-prompt-expanded-category">{prompt.category}</span>
                </div>
                <p class="api-prompt-expanded-description">{prompt.description}</p>
                <div class="api-prompt-expanded-actions">
                  <Button onClick={() => copyToClipboard(prompt.template, 'Prompt')}>
                    Copy Prompt
                  </Button>
                </div>
              </div>
              <pre class="api-prompt-expanded-content">{prompt.template}</pre>
            </div>
          ))}
        </div>
      </section>

      {/* API Schema */}
      <section class="api-section">
        <div class="api-schema-header">
          <h2 class="api-section-title">API Schema</h2>
          <div class="api-schema-actions">
            <Button variant="ghost" onClick={() => setSchemaExpanded(!schemaExpanded)}>
              {schemaExpanded ? 'Collapse' : 'Expand'}
            </Button>
            <Button variant="ghost" onClick={copySchemaAsMarkdown}>
              Copy as Markdown
            </Button>
            <Button variant="ghost" onClick={() => copyToClipboard(JSON.stringify(schema, null, 2), 'Schema JSON')}>
              Copy JSON
            </Button>
          </div>
        </div>

        {schemaExpanded && schema && (
          <div class="api-schema-content">
            <div class="api-schema-info">
              <span class="api-schema-info-item">
                <strong>Version:</strong> {schema.version}
              </span>
              <span class="api-schema-info-item">
                <strong>Base URL:</strong> {schema.base_url}
              </span>
              <span class="api-schema-info-item">
                <strong>Endpoints:</strong> {schema.endpoints?.length || 0}
              </span>
            </div>

            {/* Grouped Endpoints */}
            <div class="api-endpoints-grouped">
              {Object.entries(groupedEndpoints).map(([category, endpoints]) => (
                <div key={category} class="api-endpoint-group">
                  <h3 class="api-endpoint-group-title">{category}</h3>
                  <div class="api-endpoints-list">
                    {endpoints.map((endpoint, i) => (
                      <div key={i} class="api-endpoint-item">
                        <div class="api-endpoint-header">
                          <span class={`api-endpoint-method api-endpoint-method-${endpoint.method.toLowerCase()}`}>
                            {endpoint.method}
                          </span>
                          <span class="api-endpoint-path">{endpoint.path}</span>
                        </div>
                        <p class="api-endpoint-description">{endpoint.description}</p>
                      </div>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {!schemaExpanded && (
          <p class="api-schema-collapsed-hint">
            Click "Expand" to see all {schema?.endpoints?.length || 0} API endpoints grouped by category
          </p>
        )}
      </section>
    </div>
  );
}
