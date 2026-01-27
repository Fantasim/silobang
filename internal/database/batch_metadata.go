package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"meshbank/internal/constants"
)

// BatchOperation represents a single operation in a batch request
type BatchOperation struct {
	Hash             string
	Op               string // "set" | "delete"
	Key              string
	Value            string
	Processor        string
	ProcessorVersion string
}

// BatchOperationResult tracks the result for each operation
type BatchOperationResult struct {
	Hash    string `json:"hash"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	LogID   int64  `json:"log_id,omitempty"`
}

// GroupedOperations groups batch operations by topic
type GroupedOperations struct {
	Topic      string
	Operations []BatchOperation
}

// GroupOperationsByTopic looks up each hash in the orchestrator and groups by topic
// Returns grouped operations and any hashes that weren't found
func GroupOperationsByTopic(orchestratorDB *sql.DB, operations []BatchOperation) ([]GroupedOperations, []BatchOperationResult) {
	topicMap := make(map[string][]BatchOperation)
	var notFound []BatchOperationResult

	for _, op := range operations {
		exists, topic, _, err := CheckHashExists(orchestratorDB, op.Hash)
		if err != nil {
			notFound = append(notFound, BatchOperationResult{
				Hash:    op.Hash,
				Success: false,
				Error:   "orchestrator lookup failed: " + err.Error(),
			})
			continue
		}
		if !exists {
			notFound = append(notFound, BatchOperationResult{
				Hash:    op.Hash,
				Success: false,
				Error:   "asset not found",
			})
			continue
		}
		topicMap[topic] = append(topicMap[topic], op)
	}

	var grouped []GroupedOperations
	for topic, ops := range topicMap {
		grouped = append(grouped, GroupedOperations{
			Topic:      topic,
			Operations: ops,
		})
	}

	return grouped, notFound
}

// ExecuteBatchMetadataTx executes a batch of metadata operations within a single transaction
// All operations succeed or fail together (atomic per topic)
func ExecuteBatchMetadataTx(tx *sql.Tx, operations []BatchOperation) ([]BatchOperationResult, error) {
	results := make([]BatchOperationResult, 0, len(operations))
	timestamp := time.Now().Unix()

	// Prepare statement for efficiency
	stmt, err := tx.Prepare(`
		INSERT INTO metadata_log (asset_id, op, key, value_text, value_num, processor, processor_version, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	// Track which assets need computed update
	affectedAssets := make(map[string]bool)

	for _, op := range operations {
		// Validate operation type
		if op.Op != constants.BatchMetadataOpSet && op.Op != constants.BatchMetadataOpDelete {
			results = append(results, BatchOperationResult{
				Hash:    op.Hash,
				Success: false,
				Error:   "invalid operation: " + op.Op,
			})
			continue
		}

		// Validate key is not empty
		if op.Key == "" {
			results = append(results, BatchOperationResult{
				Hash:    op.Hash,
				Success: false,
				Error:   "key cannot be empty",
			})
			continue
		}

		// Validate key length
		if len(op.Key) > constants.MaxMetadataKeyLength {
			results = append(results, BatchOperationResult{
				Hash:    op.Hash,
				Success: false,
				Error:   fmt.Sprintf("key exceeds maximum length of %d characters", constants.MaxMetadataKeyLength),
			})
			continue
		}

		// Validate value length for set operations
		if op.Op == constants.BatchMetadataOpSet && len(op.Value) > constants.MaxMetadataValueBytes {
			results = append(results, BatchOperationResult{
				Hash:    op.Hash,
				Success: false,
				Error:   fmt.Sprintf("value exceeds maximum size of %d bytes", constants.MaxMetadataValueBytes),
			})
			continue
		}

		// Detect value type for set operations
		var valueText *string
		var valueNum *float64

		if op.Op == constants.BatchMetadataOpSet {
			text, num, err := DetectValueType(op.Value)
			if err != nil {
				results = append(results, BatchOperationResult{
					Hash:    op.Hash,
					Success: false,
					Error:   "invalid value: " + err.Error(),
				})
				continue
			}
			valueText = &text
			valueNum = num
		}

		// Insert log entry
		result, err := stmt.Exec(op.Hash, op.Op, op.Key, valueText, valueNum, op.Processor, op.ProcessorVersion, timestamp)
		if err != nil {
			results = append(results, BatchOperationResult{
				Hash:    op.Hash,
				Success: false,
				Error:   "insert failed: " + err.Error(),
			})
			continue
		}

		logID, _ := result.LastInsertId()
		results = append(results, BatchOperationResult{
			Hash:    op.Hash,
			Success: true,
			LogID:   logID,
		})

		affectedAssets[op.Hash] = true
	}

	// Update computed metadata for all affected assets
	for assetID := range affectedAssets {
		if err := updateMetadataComputedTx(tx, assetID); err != nil {
			// If computed update fails, we need to report it but not fail the whole batch
			// The log entries are still immutable and correct
			for i := range results {
				if results[i].Hash == assetID && results[i].Success {
					results[i].Error = "computed update failed: " + err.Error()
				}
			}
		}
	}

	return results, nil
}

// updateMetadataComputedTx updates the metadata_computed table within a transaction
func updateMetadataComputedTx(tx *sql.Tx, assetID string) error {
	// Query all metadata_log entries for this asset, ordered by id
	rows, err := tx.Query(`
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

		if op == constants.BatchMetadataOpSet {
			// Prefer numeric value if available, otherwise use text
			if valueNum.Valid {
				metadata[key] = valueNum.Float64
			} else if valueText.Valid {
				metadata[key] = valueText.String
			}
		} else if op == constants.BatchMetadataOpDelete {
			delete(metadata, key)
		}
	}

	if err := rows.Err(); err != nil {
		return err
	}

	// Serialize to JSON
	metadataJSON, err := serializeMetadata(metadata)
	if err != nil {
		return err
	}

	now := time.Now().Unix()

	// Insert or replace into metadata_computed
	_, err = tx.Exec(`
		INSERT OR REPLACE INTO metadata_computed (asset_id, metadata_json, updated_at)
		VALUES (?, ?, ?)
	`, assetID, metadataJSON, now)

	return err
}

// serializeMetadata converts metadata map to JSON string
func serializeMetadata(metadata map[string]interface{}) (string, error) {
	if len(metadata) == 0 {
		return "{}", nil
	}

	bytes, err := json.Marshal(metadata)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
