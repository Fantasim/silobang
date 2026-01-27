package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"silobang/internal/audit"
	"silobang/internal/auth"
	"silobang/internal/constants"
)

// handleAuditQuery handles GET /api/audit - Query audit logs
func (s *Server) handleAuditQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	identity := s.requireAuth(w, r)
	if identity == nil {
		return
	}

	if !s.authorize(w, identity, &auth.ActionContext{Action: constants.AuthActionViewAudit}) {
		return
	}

	if s.app.OrchestratorDB == nil {
		WriteError(w, http.StatusBadRequest, "Not configured", constants.ErrCodeNotConfigured)
		return
	}

	// Get requesting client IP and username for filter support
	clientIP := getClientIP(r)

	// Parse query parameters
	opts := audit.QueryOptions{
		RequestingIP:      clientIP,
		RequestingUsername: getAuditUsername(identity),
	}

	if limit := r.URL.Query().Get("limit"); limit != "" {
		opts.Limit, _ = strconv.Atoi(limit)
	}
	if offset := r.URL.Query().Get("offset"); offset != "" {
		opts.Offset, _ = strconv.Atoi(offset)
	}
	if action := r.URL.Query().Get("action"); action != "" {
		if !audit.IsValidAction(action) {
			WriteError(w, http.StatusBadRequest, "Invalid action type",
				constants.ErrCodeAuditInvalidAction)
			return
		}
		opts.Action = action
	}
	opts.IPAddress = r.URL.Query().Get("ip")
	opts.Username = r.URL.Query().Get("username")

	// Parse filter parameter for ME/OTHERS filtering
	if filter := r.URL.Query().Get("filter"); filter != "" {
		if !audit.IsValidFilter(filter) {
			WriteError(w, http.StatusBadRequest, "Invalid filter. Must be: me, others, or empty",
				constants.ErrCodeAuditInvalidFilter)
			return
		}
		opts.Filter = filter
	}

	if since := r.URL.Query().Get("since"); since != "" {
		opts.Since, _ = strconv.ParseInt(since, 10, 64)
	}
	if until := r.URL.Query().Get("until"); until != "" {
		opts.Until, _ = strconv.ParseInt(until, 10, 64)
	}

	entries, err := audit.Query(s.app.OrchestratorDB, opts)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error(),
			constants.ErrCodeAuditLogError)
		return
	}

	total, _ := audit.Count(s.app.OrchestratorDB, opts)

	// Default limit if not specified
	limit := opts.Limit
	if limit <= 0 {
		limit = constants.AuditDefaultQueryLimit
	}

	WriteSuccess(w, map[string]interface{}{
		"entries": entries,
		"total":   total,
		"limit":   limit,
		"offset":  opts.Offset,
	})
}

// handleAuditStream handles GET /api/audit/stream - SSE stream of new audit entries
func (s *Server) handleAuditStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	identity := s.requireAuth(w, r)
	if identity == nil {
		return
	}

	if !s.authorize(w, identity, &auth.ActionContext{
		Action:    constants.AuthActionViewAudit,
		SubAction: "stream",
	}) {
		return
	}

	if s.app.AuditLogger == nil {
		WriteError(w, http.StatusBadRequest, "Audit logging not configured",
			constants.ErrCodeNotConfigured)
		return
	}

	// Get client IP and parse filter
	clientIP := getClientIP(r)
	filter := r.URL.Query().Get("filter")
	if filter != "" && !audit.IsValidFilter(filter) {
		WriteError(w, http.StatusBadRequest, "Invalid filter. Must be: me, others, or empty",
			constants.ErrCodeAuditInvalidFilter)
		return
	}

	sse, err := NewSSEWriter(w)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "Streaming not supported",
			constants.ErrCodeStreamingError)
		return
	}

	// Log the "connected" audit event
	username := getAuditUsername(identity)
	s.app.AuditLogger.Log(constants.AuditActionConnected, clientIP, username, audit.ConnectedDetails{
		UserAgent: r.Header.Get("User-Agent"),
	})

	// Subscribe to audit events
	ch := s.app.AuditLogger.Subscribe()
	defer s.app.AuditLogger.Unsubscribe(ch)

	ctx := r.Context()

	// Send connected event to client with their IP and username
	connectedEvent := audit.Event{
		Type:      "connected",
		Timestamp: time.Now().Unix(),
		Data: map[string]interface{}{
			"message":   "Audit stream connected",
			"client_ip": clientIP,
			"username":  username,
		},
	}
	jsonData, _ := json.Marshal(connectedEvent)
	fmt.Fprintf(w, "data: %s\n\n", jsonData)
	sse.flusher.Flush()

	for {
		select {
		case <-ctx.Done():
			return
		case entry, ok := <-ch:
			if !ok {
				return
			}

			// Apply filter to SSE entries using username
			if filter != "" {
				switch filter {
				case constants.AuditFilterMe:
					if entry.Username != username {
						continue // Skip entries not from this user
					}
				case constants.AuditFilterOthers:
					if entry.Username == username {
						continue // Skip entries from this user
					}
				}
			}

			event := audit.Event{
				Type:      "audit_entry",
				Timestamp: time.Now().Unix(),
				Data:      entry,
			}
			jsonData, err := json.Marshal(event)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", jsonData)
			sse.flusher.Flush()
		}
	}
}

// handleAuditActions handles GET /api/audit/actions - List valid action types
func (s *Server) handleAuditActions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	identity := s.requireAuth(w, r)
	if identity == nil {
		return
	}

	if !s.authorize(w, identity, &auth.ActionContext{Action: constants.AuthActionViewAudit}) {
		return
	}

	WriteSuccess(w, map[string]interface{}{
		"actions": audit.ValidActions(),
	})
}
