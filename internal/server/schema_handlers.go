package server

import (
	"net/http"
	"strings"

	"meshbank/internal/constants"
)

// =============================================================================
// API Schema Handlers
// =============================================================================

// handleSchema handles GET /api/schema
func (s *Server) handleSchema(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	schema := s.app.Services.Schema.GetAPISchema()
	WriteSuccess(w, schema)
}

// handlePrompts handles GET /api/prompts and GET /api/prompts/:name
func (s *Server) handlePrompts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := r.URL.Path
	prefix := "/api/prompts"

	// GET /api/prompts - list all prompts with full templates
	if path == prefix || path == prefix+"/" {
		promptsList, err := s.app.Services.Schema.ListPrompts()
		if err != nil {
			s.handleServiceError(w, err)
			return
		}
		WriteSuccess(w, map[string]interface{}{
			"prompts": promptsList,
		})
		return
	}

	// GET /api/prompts/:name - get specific prompt
	name := strings.TrimPrefix(path, prefix+"/")
	if name == "" {
		WriteError(w, http.StatusBadRequest, "Prompt name required", constants.ErrCodeInvalidRequest)
		return
	}

	prompt, err := s.app.Services.Schema.GetPrompt(name)
	if err != nil {
		s.handleServiceError(w, err)
		return
	}

	WriteSuccess(w, prompt)
}
