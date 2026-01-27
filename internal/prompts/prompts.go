package prompts

import (
	"sync"
)

// PromptFile represents a prompt loaded from YAML file
type PromptFile struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description"`
	Category    string `yaml:"category" json:"category"`
	Template    string `yaml:"template" json:"template"`
}

// PromptInfo is the summary shown in list view (without template content)
type PromptInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

// RenderedPrompt is a prompt with template variables substituted
type RenderedPrompt struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Template    string `json:"template"`
}

// Manager handles loading, caching, and rendering prompts
type Manager struct {
	promptsDir string
	prompts    map[string]*PromptFile
	baseURL    string
	mu         sync.RWMutex
}

// NewManager creates a new prompts manager for the given working directory
func NewManager(workingDir string, baseURL string) *Manager {
	return &Manager{
		promptsDir: "", // Will be set by EnsurePromptsDir
		prompts:    make(map[string]*PromptFile),
		baseURL:    baseURL,
	}
}

// SetBaseURL updates the base URL used for template rendering
func (m *Manager) SetBaseURL(baseURL string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.baseURL = baseURL
}

// GetPrompt returns a specific prompt by name with template variables substituted
func (m *Manager) GetPrompt(name string) (*RenderedPrompt, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	prompt, exists := m.prompts[name]
	if !exists {
		return nil, &PromptNotFoundError{Name: name}
	}

	return &RenderedPrompt{
		Name:        prompt.Name,
		Description: prompt.Description,
		Category:    prompt.Category,
		Template:    m.renderTemplate(prompt.Template),
	}, nil
}

// ListPrompts returns info about all available prompts (without template content)
func (m *Manager) ListPrompts() []PromptInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]PromptInfo, 0, len(m.prompts))
	for _, p := range m.prompts {
		result = append(result, PromptInfo{
			Name:        p.Name,
			Description: p.Description,
			Category:    p.Category,
		})
	}
	return result
}

// ListPromptsFull returns all prompts with rendered templates
func (m *Manager) ListPromptsFull() []RenderedPrompt {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]RenderedPrompt, 0, len(m.prompts))
	for _, p := range m.prompts {
		result = append(result, RenderedPrompt{
			Name:        p.Name,
			Description: p.Description,
			Category:    p.Category,
			Template:    m.renderTemplate(p.Template),
		})
	}
	return result
}

// PromptNotFoundError indicates a prompt was not found
type PromptNotFoundError struct {
	Name string
}

func (e *PromptNotFoundError) Error() string {
	return "prompt not found: " + e.Name
}
