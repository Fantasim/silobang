package audit

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"silobang/internal/constants"
)

// QueryOptions for filtering audit logs
type QueryOptions struct {
	Limit              int
	Offset             int
	Action             string
	IPAddress          string
	Username           string // Filter by specific username
	Since              int64  // Unix timestamp
	Until              int64  // Unix timestamp
	Filter             string // "me" | "others" | "" (for ME/OTHERS filtering)
	RequestingIP       string // IP of the requesting client (used with Filter)
	RequestingUsername  string // Username of the requesting client (used with Filter)
}

// IsValidFilter checks if a filter value is valid
func IsValidFilter(filter string) bool {
	return filter == constants.AuditFilterMe ||
		filter == constants.AuditFilterOthers ||
		filter == constants.AuditFilterAll
}

// Query retrieves audit log entries with filters
func Query(db *sql.DB, opts QueryOptions) ([]Entry, error) {
	// Apply defaults and limits
	if opts.Limit <= 0 {
		opts.Limit = constants.AuditDefaultQueryLimit
	}
	if opts.Limit > constants.AuditMaxQueryLimit {
		opts.Limit = constants.AuditMaxQueryLimit
	}

	query := `SELECT id, timestamp, action, ip_address, username, details_json
              FROM audit_log WHERE 1=1`
	args := []interface{}{}

	if opts.Action != "" {
		query += " AND action = ?"
		args = append(args, opts.Action)
	}

	// Handle explicit username filter
	if opts.Username != "" {
		query += " AND username = ?"
		args = append(args, opts.Username)
	}

	// Handle IP filtering: explicit IPAddress takes precedence over Filter
	if opts.IPAddress != "" {
		query += " AND ip_address = ?"
		args = append(args, opts.IPAddress)
	}

	// Apply ME/OTHERS filter using username
	if opts.Filter != "" && opts.RequestingUsername != "" {
		switch opts.Filter {
		case constants.AuditFilterMe:
			query += " AND username = ?"
			args = append(args, opts.RequestingUsername)
		case constants.AuditFilterOthers:
			query += " AND username != ?"
			args = append(args, opts.RequestingUsername)
		}
	}

	if opts.Since > 0 {
		query += " AND timestamp >= ?"
		args = append(args, opts.Since)
	}
	if opts.Until > 0 {
		query += " AND timestamp <= ?"
		args = append(args, opts.Until)
	}

	query += " ORDER BY id DESC LIMIT ? OFFSET ?"
	args = append(args, opts.Limit, opts.Offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit logs: %w", err)
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var entry Entry
		var detailsJSON sql.NullString

		err := rows.Scan(&entry.ID, &entry.Timestamp, &entry.Action,
			&entry.IPAddress, &entry.Username, &detailsJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit log: %w", err)
		}

		if detailsJSON.Valid {
			var details interface{}
			json.Unmarshal([]byte(detailsJSON.String), &details)
			entry.Details = details
		}

		entries = append(entries, entry)
	}

	return entries, rows.Err()
}

// GetEntry retrieves a single audit entry by ID
func GetEntry(db *sql.DB, id int64) (*Entry, error) {
	var entry Entry
	var detailsJSON sql.NullString

	err := db.QueryRow(`
		SELECT id, timestamp, action, ip_address, username, details_json
		FROM audit_log WHERE id = ?
	`, id).Scan(&entry.ID, &entry.Timestamp, &entry.Action,
		&entry.IPAddress, &entry.Username, &detailsJSON)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if detailsJSON.Valid {
		var details interface{}
		json.Unmarshal([]byte(detailsJSON.String), &details)
		entry.Details = details
	}

	return &entry, nil
}

// Count returns total number of audit entries matching filters
func Count(db *sql.DB, opts QueryOptions) (int64, error) {
	query := `SELECT COUNT(*) FROM audit_log WHERE 1=1`
	args := []interface{}{}

	if opts.Action != "" {
		query += " AND action = ?"
		args = append(args, opts.Action)
	}

	// Handle explicit username filter
	if opts.Username != "" {
		query += " AND username = ?"
		args = append(args, opts.Username)
	}

	// Handle IP filtering
	if opts.IPAddress != "" {
		query += " AND ip_address = ?"
		args = append(args, opts.IPAddress)
	}

	// Apply ME/OTHERS filter using username
	if opts.Filter != "" && opts.RequestingUsername != "" {
		switch opts.Filter {
		case constants.AuditFilterMe:
			query += " AND username = ?"
			args = append(args, opts.RequestingUsername)
		case constants.AuditFilterOthers:
			query += " AND username != ?"
			args = append(args, opts.RequestingUsername)
		}
	}

	if opts.Since > 0 {
		query += " AND timestamp >= ?"
		args = append(args, opts.Since)
	}
	if opts.Until > 0 {
		query += " AND timestamp <= ?"
		args = append(args, opts.Until)
	}

	var count int64
	err := db.QueryRow(query, args...).Scan(&count)
	return count, err
}
