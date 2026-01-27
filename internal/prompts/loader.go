package prompts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	"silobang/internal/constants"
	"silobang/internal/logger"
)

// GetPromptsDir returns the path to the prompts directory for a working directory
func GetPromptsDir(workingDir string) string {
	return filepath.Join(workingDir, constants.InternalDir, constants.PromptsDir)
}

// EnsurePromptsDir creates the prompts directory and copies default prompts if needed
func (m *Manager) EnsurePromptsDir(workingDir string, log *logger.Logger) error {
	promptsDir := GetPromptsDir(workingDir)
	m.promptsDir = promptsDir

	// Check if prompts directory exists
	info, err := os.Stat(promptsDir)
	if err == nil && info.IsDir() {
		log.Debug("Prompts directory already exists: %s", promptsDir)
		return nil
	}

	if !os.IsNotExist(err) && err != nil {
		return fmt.Errorf("failed to check prompts directory: %w", err)
	}

	// Directory doesn't exist, create it and write defaults
	log.Info("Prompts directory not found, creating with defaults: %s", promptsDir)

	if err := os.MkdirAll(promptsDir, constants.DirPermissions); err != nil {
		return fmt.Errorf("failed to create prompts directory: %w", err)
	}

	// Write default prompt files
	defaults := GetDefaultPrompts()
	for name, content := range defaults {
		filename := name + constants.PromptFileExtension
		filePath := filepath.Join(promptsDir, filename)

		if err := os.WriteFile(filePath, []byte(content), constants.FilePermissions); err != nil {
			log.Warn("Failed to write default prompt %s: %v", name, err)
			continue
		}
		log.Debug("Created default prompt file: %s", filename)
	}

	log.Info("Created %d default prompt files in %s", len(defaults), promptsDir)
	return nil
}

// LoadPrompts loads all prompt files from the prompts directory
func (m *Manager) LoadPrompts(log *logger.Logger) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.promptsDir == "" {
		return fmt.Errorf("prompts directory not set, call EnsurePromptsDir first")
	}

	log.Debug("Loading prompts from directory: %s", m.promptsDir)

	// Check if directory exists
	if _, err := os.Stat(m.promptsDir); os.IsNotExist(err) {
		return fmt.Errorf("prompts directory does not exist: %s", m.promptsDir)
	}

	entries, err := os.ReadDir(m.promptsDir)
	if err != nil {
		return fmt.Errorf("failed to read prompts directory: %w", err)
	}

	// Clear existing prompts
	m.prompts = make(map[string]*PromptFile)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		if !strings.HasSuffix(filename, constants.PromptFileExtension) {
			log.Debug("Skipping non-prompt file: %s", filename)
			continue
		}

		filePath := filepath.Join(m.promptsDir, filename)
		prompt, err := loadPromptFile(filePath)
		if err != nil {
			log.Warn("Skipping invalid prompt file %s: %v", filename, err)
			continue
		}

		// Derive expected name from filename
		expectedName := strings.TrimSuffix(filename, constants.PromptFileExtension)
		if prompt.Name != expectedName {
			log.Warn("Prompt name mismatch in %s: expected %s, got %s (using filename)", filename, expectedName, prompt.Name)
			prompt.Name = expectedName
		}

		if err := validatePrompt(prompt); err != nil {
			log.Warn("Skipping invalid prompt %s: %v", filename, err)
			continue
		}

		log.Debug("Loaded prompt: %s (%s)", prompt.Name, prompt.Category)
		m.prompts[prompt.Name] = prompt
	}

	log.Info("Loaded %d prompts from %s", len(m.prompts), m.promptsDir)
	return nil
}

// ReloadPrompts reloads all prompts from disk
func (m *Manager) ReloadPrompts(log *logger.Logger) error {
	return m.LoadPrompts(log)
}

// loadPromptFile loads and parses a single prompt file
func loadPromptFile(filePath string) (*PromptFile, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var prompt PromptFile
	if err := yaml.Unmarshal(data, &prompt); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return &prompt, nil
}

// validatePrompt validates a PromptFile structure
func validatePrompt(prompt *PromptFile) error {
	if prompt.Name == "" {
		return fmt.Errorf("prompt name is required")
	}

	if prompt.Description == "" {
		return fmt.Errorf("prompt description is required")
	}

	if prompt.Category == "" {
		return fmt.Errorf("prompt category is required")
	}

	if prompt.Template == "" {
		return fmt.Errorf("prompt template is required")
	}

	return nil
}

// PromptCount returns the number of loaded prompts
func (m *Manager) PromptCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.prompts)
}
