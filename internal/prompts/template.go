package prompts

import (
	"strings"
)

// Template variable placeholders
const (
	VarBaseURL = "{{base_url}}"
)

// renderTemplate substitutes template variables with their values
// This method is called with the read lock held
func (m *Manager) renderTemplate(template string) string {
	result := template

	// Substitute base_url
	if m.baseURL != "" {
		result = strings.ReplaceAll(result, VarBaseURL, m.baseURL)
	}

	return result
}
