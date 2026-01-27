package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"silobang/internal/audit"
	"silobang/internal/auth"
	"silobang/internal/constants"
)

// VerifyEvent represents an SSE event for verification progress
type VerifyEvent struct {
	Type      string      `json:"type"`
	Timestamp int64       `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// Event payload structures

type ScanStartData struct {
	Topics     []string `json:"topics"`
	CheckIndex bool     `json:"check_index"`
}

type TopicStartData struct {
	Topic    string `json:"topic"`
	DatFiles int    `json:"dat_files"`
}

type DatProgressData struct {
	Topic            string `json:"topic"`
	DatFile          string `json:"dat_file"`
	EntriesProcessed int    `json:"entries_processed"`
	TotalEntries     int    `json:"total_entries"`
}

type DatCompleteData struct {
	Topic   string `json:"topic"`
	DatFile string `json:"dat_file"`
	Valid   bool   `json:"valid"`
	Entries int    `json:"entries"`
	Error   string `json:"error,omitempty"`
}

type TopicCompleteData struct {
	Topic           string   `json:"topic"`
	Valid           bool     `json:"valid"`
	DatFilesChecked int      `json:"dat_files_checked"`
	Errors          []string `json:"errors,omitempty"`
}

type IndexStartData struct {
	TotalEntries int `json:"total_entries"`
}

type IndexIssue struct {
	Type    string `json:"type"` // "orphan", "missing", "mismatch"
	Hash    string `json:"hash"`
	Topic   string `json:"topic,omitempty"`
	DatFile string `json:"dat_file,omitempty"`
	Detail  string `json:"detail,omitempty"`
}

type IndexCompleteData struct {
	OrphanCount   int          `json:"orphan_count"`
	MissingCount  int          `json:"missing_count"`
	MismatchCount int          `json:"mismatch_count"`
	Issues        []IndexIssue `json:"issues,omitempty"`
}

type CompleteData struct {
	TopicsChecked int  `json:"topics_checked"`
	TopicsValid   int  `json:"topics_valid"`
	IndexValid    bool `json:"index_valid"`
	DurationMs    int  `json:"duration_ms"`
}

type ErrorData struct {
	Message string `json:"message"`
	Code    string `json:"code"`
	Topic   string `json:"topic,omitempty"`
}

// VerifyOptions holds verification parameters
type VerifyOptions struct {
	Topics           []string
	CheckIndex       bool
	ProgressInterval int // Report progress every N entries (0 = no progress events)
}

// SSEWriter handles Server-Sent Events formatting and flushing
type SSEWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

// NewSSEWriter creates a new SSE writer with proper headers
func NewSSEWriter(w http.ResponseWriter) (*SSEWriter, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported")
	}

	// Set SSE headers
	w.Header().Set(constants.HeaderContentType, constants.ContentTypeSSE)
	w.Header().Set(constants.HeaderCacheControl, constants.SSECacheControl)
	w.Header().Set(constants.HeaderConnection, constants.SSEConnection)
	w.Header().Set(constants.HeaderXAccelBuffering, constants.SSEXAccelBuffering) // Disable nginx buffering

	return &SSEWriter{w: w, flusher: flusher}, nil
}

// Send sends an SSE event with the given type and data
func (s *SSEWriter) Send(eventType string, data interface{}) error {
	event := VerifyEvent{
		Type:      eventType,
		Timestamp: time.Now().Unix(),
		Data:      data,
	}

	jsonData, err := json.Marshal(event)
	if err != nil {
		return err
	}

	// SSE format: "data: {json}\n\n"
	_, err = fmt.Fprintf(s.w, "data: %s\n\n", jsonData)
	if err != nil {
		return err
	}

	s.flusher.Flush()
	return nil
}

// handleVerify handles GET /api/verify with SSE streaming
func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	identity := s.requireAuth(w, r)
	if identity == nil {
		return
	}

	if !s.authorize(w, identity, &auth.ActionContext{Action: constants.AuthActionVerify}) {
		return
	}

	// Check if configured
	if s.app.Config.WorkingDirectory == "" {
		WriteError(w, http.StatusBadRequest, "Working directory not configured", constants.ErrCodeNotConfigured)
		return
	}

	// Parse query parameters
	opts := s.parseVerifyOptions(r)

	// Validate topics exist
	if len(opts.Topics) > 0 {
		for _, topic := range opts.Topics {
			if !s.app.TopicExists(topic) {
				WriteError(w, http.StatusNotFound, "Topic not found: "+topic, constants.ErrCodeTopicNotFound)
				return
			}
		}
	} else {
		// Default to all topics
		opts.Topics = s.app.ListTopics()
	}

	// Set up SSE writer
	sse, err := NewSSEWriter(w)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "Streaming not supported", constants.ErrCodeStreamingError)
		return
	}

	// Run verification with streaming
	s.runVerification(r.Context(), sse, opts, getClientIP(r), getAuditUsername(identity))
}

func (s *Server) parseVerifyOptions(r *http.Request) VerifyOptions {
	opts := VerifyOptions{
		CheckIndex:       true,
		ProgressInterval: constants.DefaultVerifyProgressInterval,
	}

	// Parse topics
	if topicsParam := r.URL.Query().Get("topics"); topicsParam != "" {
		opts.Topics = strings.Split(topicsParam, ",")
		for i := range opts.Topics {
			opts.Topics[i] = strings.TrimSpace(opts.Topics[i])
		}
	}

	// Parse check_index
	if checkIndex := r.URL.Query().Get("check_index"); checkIndex == "false" {
		opts.CheckIndex = false
	}

	return opts
}

func (s *Server) runVerification(ctx context.Context, sse *SSEWriter, opts VerifyOptions, clientIP string, username string) {
	startTime := time.Now()

	s.logger.Info("Starting verification: %d topics, check_index=%v", len(opts.Topics), opts.CheckIndex)

	// Send scan_start event
	sse.Send("scan_start", ScanStartData{
		Topics:     opts.Topics,
		CheckIndex: opts.CheckIndex,
	})

	topicsValid := 0

	// Verify each topic
	for _, topicName := range opts.Topics {
		// Check for cancellation
		select {
		case <-ctx.Done():
			sse.Send("error", ErrorData{
				Message: "Verification cancelled",
				Code:    constants.ErrCodeStreamingError,
			})
			return
		default:
		}

		valid := s.verifyTopic(ctx, sse, topicName, opts.ProgressInterval)
		if valid {
			topicsValid++
		}
	}

	// Verify index consistency if requested
	indexValid := true
	if opts.CheckIndex && s.app.OrchestratorDB != nil {
		indexValid = s.verifyIndex(ctx, sse, opts.Topics)
	}

	// Send complete event
	duration := time.Since(startTime)
	durationMs := int(duration.Milliseconds())

	s.logger.Info("Verification complete: %d/%d topics valid, index_valid=%v, duration=%dms", topicsValid, len(opts.Topics), indexValid, durationMs)

	sse.Send("complete", CompleteData{
		TopicsChecked: len(opts.Topics),
		TopicsValid:   topicsValid,
		IndexValid:    indexValid,
		DurationMs:    durationMs,
	})

	// Audit log for verification complete
	if s.app.AuditLogger != nil {
		s.app.AuditLogger.Log(constants.AuditActionVerified, clientIP, username, audit.VerifiedDetails{
			TopicsChecked: len(opts.Topics),
			TopicsValid:   topicsValid,
			IndexValid:    indexValid,
			DurationMs:    durationMs,
		})
	}
}

func (s *Server) verifyTopic(ctx context.Context, sse *SSEWriter, topicName string, progressInterval int) bool {
	// List .dat files via service
	datFiles, err := s.app.Services.Verify.ListDatFiles(topicName)
	if err != nil {
		sse.Send("error", ErrorData{
			Message: fmt.Sprintf("Failed to list dat files: %v", err),
			Code:    constants.ErrCodeInternalError,
			Topic:   topicName,
		})
		return false
	}

	// Send topic_start
	sse.Send("topic_start", TopicStartData{
		Topic:    topicName,
		DatFiles: len(datFiles),
	})

	// Check topic database is accessible
	_, dbErr := s.app.Services.Verify.GetTopicDB(topicName)
	if dbErr != nil {
		sse.Send("topic_complete", TopicCompleteData{
			Topic:  topicName,
			Valid:  false,
			Errors: []string{fmt.Sprintf("Failed to open database: %v", dbErr)},
		})
		return false
	}

	var errors []string
	datFilesChecked := 0

	// Verify each .dat file via service
	for _, datFile := range datFiles {
		select {
		case <-ctx.Done():
			return false
		default:
		}

		// Create progress callback for SSE
		progressCallback := func(entriesProcessed, totalEntries int) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			if progressInterval > 0 {
				sse.Send("dat_progress", DatProgressData{
					Topic:            topicName,
					DatFile:          datFile,
					EntriesProcessed: entriesProcessed,
					TotalEntries:     totalEntries,
				})
			}
			return nil
		}

		result, _ := s.app.Services.Verify.VerifyDatFile(ctx, topicName, datFile, progressInterval, progressCallback)

		sse.Send("dat_complete", DatCompleteData{
			Topic:   topicName,
			DatFile: datFile,
			Valid:   result.Valid,
			Entries: result.EntryCount,
			Error:   result.Error,
		})

		datFilesChecked++
		if !result.Valid {
			errors = append(errors, fmt.Sprintf("%s: %s", datFile, result.Error))
		}
	}

	// Send topic_complete
	sse.Send("topic_complete", TopicCompleteData{
		Topic:           topicName,
		Valid:           len(errors) == 0,
		DatFilesChecked: datFilesChecked,
		Errors:          errors,
	})

	return len(errors) == 0
}

func (s *Server) verifyIndex(ctx context.Context, sse *SSEWriter, topics []string) bool {
	// Get total entries for progress
	totalEntries, _ := s.app.Services.Verify.GetTotalIndexEntries()

	sse.Send("index_start", IndexStartData{
		TotalEntries: totalEntries,
	})

	// Verify index via service
	result, err := s.app.Services.Verify.VerifyIndex(ctx, topics)
	if err != nil {
		sse.Send("error", ErrorData{
			Message: fmt.Sprintf("Failed to verify index: %v", err),
			Code:    constants.ErrCodeInternalError,
		})
		return false
	}

	// Convert service issues to handler issues
	issues := make([]IndexIssue, len(result.Issues))
	for i, issue := range result.Issues {
		issues[i] = IndexIssue{
			Type:    issue.Type,
			Hash:    issue.Hash,
			Topic:   issue.Topic,
			DatFile: issue.DatFile,
			Detail:  issue.Detail,
		}
	}

	sse.Send("index_complete", IndexCompleteData{
		OrphanCount:   result.OrphanCount,
		MissingCount:  result.MissingCount,
		MismatchCount: result.MismatchCount,
		Issues:        issues,
	})

	return result.Valid
}
