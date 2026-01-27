package queries

import (
	"fmt"
	"os"
	"sort"

	"gopkg.in/yaml.v3"
)

// QueriesConfig represents the entire queries.yaml file
type QueriesConfig struct {
	TopicStats []TopicStat       `yaml:"topic_stats"`
	Presets    map[string]Preset `yaml:"presets"`
}

// TopicStat defines a stat shown on the topic list
type TopicStat struct {
	Name   string `yaml:"name"`
	Label  string `yaml:"label"`
	SQL    string `yaml:"sql,omitempty"`    // SQL query (mutually exclusive with Type)
	Type   string `yaml:"type,omitempty"`   // Special type: "file_size" or "dat_total"
	Format string `yaml:"format,omitempty"` // Display format: bytes|number|date|text
}

// Preset defines a query preset
type Preset struct {
	Description string        `yaml:"description"`
	SQL         string        `yaml:"sql"`
	Params      []PresetParam `yaml:"params,omitempty"`
}

// PresetParam defines a parameter for a preset query
type PresetParam struct {
	Name     string `yaml:"name"`
	Required bool   `yaml:"required,omitempty"`
	Default  string `yaml:"default,omitempty"`
}

// PresetInfo contains metadata about a preset for API responses
type PresetInfo struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Params      []PresetParamInfo `json:"params"`
}

// PresetParamInfo contains parameter info for API responses
type PresetParamInfo struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
	Default  string `json:"default,omitempty"`
}

// LoadQueriesConfig loads and parses the queries.yaml file
func LoadQueriesConfig(path string) (*QueriesConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read queries config: %w", err)
	}

	var config QueriesConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse queries config: %w", err)
	}

	return &config, nil
}

// GetPreset returns a preset by name
func (c *QueriesConfig) GetPreset(name string) (*Preset, error) {
	preset, exists := c.Presets[name]
	if !exists {
		return nil, fmt.Errorf("preset not found: %s", name)
	}
	return &preset, nil
}

// ListPresets returns info about all available presets
func (c *QueriesConfig) ListPresets() []PresetInfo {
	result := make([]PresetInfo, 0, len(c.Presets))

	for name, preset := range c.Presets {
		params := make([]PresetParamInfo, len(preset.Params))
		for i, p := range preset.Params {
			params[i] = PresetParamInfo{
				Name:     p.Name,
				Required: p.Required,
				Default:  p.Default,
			}
		}

		result = append(result, PresetInfo{
			Name:        name,
			Description: preset.Description,
			Params:      params,
		})
	}

	// Sort by name for consistent ordering
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}
