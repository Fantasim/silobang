package server

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"silobang/internal/constants"
)

// =============================================================================
// Cached Response Type
// =============================================================================

// cachedResponse holds pre-serialized and pre-compressed data for immutable
// endpoints. Once built, the cached data never changes for the server lifetime.
type cachedResponse struct {
	raw  []byte // JSON-encoded bytes
	gzip []byte // Pre-compressed gzip bytes
	etag string // ETag header value (sha256 fingerprint)
}

// buildCachedResponse serializes data to JSON, pre-compresses it with gzip,
// and computes a SHA-256 ETag for conditional request support.
func buildCachedResponse(data interface{}) (*cachedResponse, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	// Pre-compress with gzip
	var compressed bytes.Buffer
	gz, err := gzip.NewWriterLevel(&compressed, constants.CompressionLevel)
	if err != nil {
		return nil, err
	}
	if _, err := gz.Write(raw); err != nil {
		gz.Close()
		return nil, err
	}
	gz.Close()

	// Compute ETag from raw JSON content (SHA-256 truncated to 16 hex chars)
	hash := sha256.Sum256(raw)
	etag := `"` + hex.EncodeToString(hash[:8]) + `"`

	return &cachedResponse{
		raw:  raw,
		gzip: compressed.Bytes(),
		etag: etag,
	}, nil
}

// serveCachedResponse writes a cached response with proper ETag, Cache-Control,
// and conditional request (If-None-Match → 304) support.
// If the client accepts gzip, pre-compressed bytes are served directly.
func serveCachedResponse(w http.ResponseWriter, r *http.Request, cache *cachedResponse) {
	// Conditional request: If-None-Match → 304 Not Modified
	if r.Header.Get("If-None-Match") == cache.etag {
		w.Header().Set("ETag", cache.etag)
		w.Header().Set("Cache-Control", constants.CacheControlImmutable)
		w.WriteHeader(http.StatusNotModified)
		return
	}

	// Override SecurityHeaders' "Cache-Control: no-store" for immutable content
	w.Header().Set("Cache-Control", constants.CacheControlImmutable)
	w.Header().Set("ETag", cache.etag)
	w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)

	// Serve pre-compressed if client accepts gzip
	if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		w.Header().Set("Content-Encoding", "gzip")
		w.Write(cache.gzip) //nolint:errcheck
		return
	}

	// Serve raw JSON
	w.Write(cache.raw) //nolint:errcheck
}

// =============================================================================
// API Schema Handlers
// =============================================================================

// handleSchema handles GET /api/schema.
// The schema is immutable at runtime, so it is cached on first request
// with pre-compression and ETag support for conditional requests.
func (s *Server) handleSchema(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cache := s.getSchemaCache()
	if cache != nil {
		s.logger.Debug("Schema: serving from cache (etag=%s)", cache.etag)
		serveCachedResponse(w, r, cache)
		return
	}

	// Cache miss or build failed — serve fresh (fallback)
	s.logger.Debug("Schema: serving fresh (cache not available)")
	schema := s.app.Services.Schema.GetAPISchema()
	WriteSuccess(w, schema)
}

// getSchemaCache returns the cached schema response, building it on first call.
// Thread-safe via double-checked locking with RWMutex.
func (s *Server) getSchemaCache() *cachedResponse {
	s.schemaCacheMu.RLock()
	c := s.schemaCache
	s.schemaCacheMu.RUnlock()
	if c != nil {
		return c
	}

	s.schemaCacheMu.Lock()
	defer s.schemaCacheMu.Unlock()
	if s.schemaCache != nil {
		return s.schemaCache
	}

	// Build cache
	schema := s.app.Services.Schema.GetAPISchema()
	cache, err := buildCachedResponse(schema)
	if err != nil {
		s.logger.Warn("Schema: failed to build cache: %v", err)
		return nil
	}

	s.logger.Info("Schema: cache built (raw=%d bytes, gzip=%d bytes, etag=%s)",
		len(cache.raw), len(cache.gzip), cache.etag)
	s.schemaCache = cache
	return cache
}

// handlePrompts handles GET /api/prompts and GET /api/prompts/:name.
// The prompts list (GET /api/prompts) is immutable at runtime and cached.
// Individual prompts (GET /api/prompts/:name) are not cached.
func (s *Server) handlePrompts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := r.URL.Path
	prefix := "/api/prompts"

	// GET /api/prompts - list all prompts with full templates (cached)
	if path == prefix || path == prefix+"/" {
		cache := s.getPromptsCache()
		if cache != nil {
			s.logger.Debug("Prompts: serving list from cache (etag=%s)", cache.etag)
			serveCachedResponse(w, r, cache)
			return
		}

		// Cache miss — serve fresh (may fail if not configured)
		s.logger.Debug("Prompts: serving list fresh (cache not available)")
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

	// GET /api/prompts/:name - get specific prompt (not cached)
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

// getPromptsCache returns the cached prompts list, building it on first
// successful call. Returns nil if prompts are not yet available (working
// directory not configured). Thread-safe via double-checked locking.
func (s *Server) getPromptsCache() *cachedResponse {
	s.promptsCacheMu.RLock()
	c := s.promptsCache
	s.promptsCacheMu.RUnlock()
	if c != nil {
		return c
	}

	s.promptsCacheMu.Lock()
	defer s.promptsCacheMu.Unlock()
	if s.promptsCache != nil {
		return s.promptsCache
	}

	// Try to build cache — may fail if working directory not configured
	promptsList, err := s.app.Services.Schema.ListPrompts()
	if err != nil {
		s.logger.Debug("Prompts: cache build skipped (not yet available): %v", err)
		return nil
	}

	data := map[string]interface{}{
		"prompts": promptsList,
	}
	cache, err := buildCachedResponse(data)
	if err != nil {
		s.logger.Warn("Prompts: failed to build cache: %v", err)
		return nil
	}

	s.logger.Info("Prompts: cache built (raw=%d bytes, gzip=%d bytes, etag=%s)",
		len(cache.raw), len(cache.gzip), cache.etag)
	s.promptsCache = cache
	return cache
}
