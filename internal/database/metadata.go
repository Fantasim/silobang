package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// MetadataLogEntry represents an entry in the metadata_log table
type MetadataLogEntry struct {
	ID               int64
	AssetID          string
	Op               string // "set" | "delete"
	Key              string
	Value            string // original value (for set)
	Processor        string
	ProcessorVersion string
	Timestamp        int64
}

// DetectValueType determines whether a value should be stored as text, number, or both
// Returns text representation and optionally a numeric representation
func DetectValueType(value string) (text string, num *float64, err error) {
	// Rule 1: Empty string -> error (reject)
	if value == "" {
		return "", nil, fmt.Errorf("empty string is not allowed")
	}

	// Rule 5: Boolean strings -> text only
	if value == "true" || value == "false" {
		return value, nil, nil
	}

	// Rule 2: Try parse as float64
	parsed, parseErr := strconv.ParseFloat(value, 64)

	// Rule 4: If parse fails -> text only
	if parseErr != nil {
		return value, nil, nil
	}

	// Rule 3: If parse succeeds, check if string representation matches original
	// We need to check for:
	// - Leading zeros (e.g., "01234")
	// - Trailing zeros after decimal (e.g., "1.0", "1.00")
	// - Scientific notation (e.g., "1e10", "1E5")

	// Check for leading zeros (except for "0" or "0.xxx")
	if len(value) > 1 && value[0] == '0' && value[1] != '.' {
		return value, nil, nil
	}

	// Check for scientific notation
	if strings.ContainsAny(value, "eE") {
		return value, nil, nil
	}

	// Check for trailing zeros after decimal point
	if strings.Contains(value, ".") {
		// Reconstruct the string from the parsed number
		reconstructed := strconv.FormatFloat(parsed, 'f', -1, 64)
		if reconstructed != value {
			// The reconstructed version doesn't match - likely trailing zeros
			return value, nil, nil
		}
	}

	// Valid integer or decimal without leading/trailing zeros, not scientific notation
	return value, &parsed, nil
}

// InsertMetadataLog inserts into metadata_log and updates the computed view atomically.
// Both operations run within a single transaction to prevent inconsistency between
// metadata_log and metadata_computed under concurrent writes or process crashes.
func InsertMetadataLog(db *sql.DB, entry MetadataLogEntry) (int64, error) {
	// Detect value type
	var valueText *string
	var valueNum *float64

	if entry.Op == "set" {
		text, num, err := DetectValueType(entry.Value)
		if err != nil {
			return 0, err
		}
		valueText = &text
		valueNum = num
	}

	// If no timestamp provided, use current time
	if entry.Timestamp == 0 {
		entry.Timestamp = time.Now().Unix()
	}

	// Begin transaction for atomic insert + computed update
	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("failed to begin metadata transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert into metadata_log
	result, err := tx.Exec(`
		INSERT INTO metadata_log (asset_id, op, key, value_text, value_num, processor, processor_version, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, entry.AssetID, entry.Op, entry.Key, valueText, valueNum, entry.Processor, entry.ProcessorVersion, entry.Timestamp)
	if err != nil {
		return 0, fmt.Errorf("failed to insert metadata log: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert id: %w", err)
	}

	// Update computed view within the same transaction
	if err := updateMetadataComputedTx(tx, entry.AssetID); err != nil {
		return 0, fmt.Errorf("failed to update metadata computed: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit metadata transaction: %w", err)
	}

	return id, nil
}

// UpdateMetadataComputed rebuilds metadata_computed for the given asset.
// For new code, prefer InsertMetadataLog which handles both the log insert and
// computed update atomically within a transaction. This standalone function is
// retained for recovery/rebuild scenarios where the log already exists.
func UpdateMetadataComputed(db *sql.DB, assetID string) error {
	// Query all metadata_log entries for this asset, ordered by id
	rows, err := db.Query(`
		SELECT op, key, value_text, value_num
		FROM metadata_log
		WHERE asset_id = ?
		ORDER BY id
	`, assetID)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Build metadata map by applying operations in order
	metadata := make(map[string]interface{})

	for rows.Next() {
		var op, key string
		var valueText sql.NullString
		var valueNum sql.NullFloat64

		if err := rows.Scan(&op, &key, &valueText, &valueNum); err != nil {
			return err
		}

		if op == "set" {
			// Prefer numeric value if available, otherwise use text
			if valueNum.Valid {
				metadata[key] = valueNum.Float64
			} else if valueText.Valid {
				metadata[key] = valueText.String
			}
		} else if op == "delete" {
			delete(metadata, key)
		}
	}

	if err := rows.Err(); err != nil {
		return err
	}

	// Serialize to JSON
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	now := time.Now().Unix()

	// Insert or replace into metadata_computed
	_, err = db.Exec(`
		INSERT OR REPLACE INTO metadata_computed (asset_id, metadata_json, updated_at)
		VALUES (?, ?, ?)
	`, assetID, string(metadataJSON), now)

	return err
}

// GetMetadataComputed queries metadata_computed and parses JSON
// Returns nil if no metadata exists
func GetMetadataComputed(db *sql.DB, assetID string) (map[string]interface{}, error) {
	var metadataJSON string
	err := db.QueryRow("SELECT metadata_json FROM metadata_computed WHERE asset_id = ?", assetID).Scan(&metadataJSON)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
		return nil, err
	}

	return metadata, nil
}

// MetadataWithProcessor represents a metadata key with its processor info
type MetadataWithProcessor struct {
	Key              string      `json:"key"`
	Value            interface{} `json:"value"`
	Processor        string      `json:"processor"`
	ProcessorVersion string      `json:"processor_version"`
	Timestamp        int64       `json:"timestamp"`
}

// GetMetadataWithProcessor returns computed metadata with processor info for each key
func GetMetadataWithProcessor(db *sql.DB, assetID string) ([]MetadataWithProcessor, error) {
	entries, err := GetMetadataLog(db, assetID)
	if err != nil {
		return nil, err
	}

	// Apply log entries in order, track processor for final state of each key
	metadata := make(map[string]MetadataWithProcessor)

	for _, entry := range entries {
		if entry.Op == "set" {
			metadata[entry.Key] = MetadataWithProcessor{
				Key:              entry.Key,
				Value:            entry.Value,
				Processor:        entry.Processor,
				ProcessorVersion: entry.ProcessorVersion,
				Timestamp:        entry.Timestamp,
			}
		} else if entry.Op == "delete" {
			delete(metadata, entry.Key)
		}
	}

	result := make([]MetadataWithProcessor, 0, len(metadata))
	for _, m := range metadata {
		result = append(result, m)
	}

	return result, nil
}

// GetMetadataLog queries all log entries for an asset
func GetMetadataLog(db *sql.DB, assetID string) ([]MetadataLogEntry, error) {
	rows, err := db.Query(`
		SELECT id, asset_id, op, key, value_text, processor, processor_version, timestamp
		FROM metadata_log
		WHERE asset_id = ?
		ORDER BY id
	`, assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []MetadataLogEntry
	for rows.Next() {
		var entry MetadataLogEntry
		var valueText sql.NullString

		if err := rows.Scan(&entry.ID, &entry.AssetID, &entry.Op, &entry.Key, &valueText,
			&entry.Processor, &entry.ProcessorVersion, &entry.Timestamp); err != nil {
			return nil, err
		}

		if valueText.Valid {
			entry.Value = valueText.String
		}

		entries = append(entries, entry)
	}

	return entries, rows.Err()
}
