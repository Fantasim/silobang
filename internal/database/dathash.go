package database

import (
	"database/sql"
	"meshbank/internal/storage"
	"os"
	"path/filepath"
	"time"
)

// DatHashRecord represents a complete dat_hashes row
type DatHashRecord struct {
	DatFile     string
	RunningHash string
	EntryCount  int64
	UpdatedAt   int64
}

// GetDatHash queries dat_hashes for the given dat file
// Returns running hash and entry count, or empty values if not found
func GetDatHash(db *sql.DB, datFile string) (runningHash string, entryCount int64, err error) {
	err = db.QueryRow("SELECT running_hash, entry_count FROM dat_hashes WHERE dat_file = ?", datFile).Scan(&runningHash, &entryCount)
	if err == sql.ErrNoRows {
		return "", 0, nil
	}
	if err != nil {
		return "", 0, err
	}
	return runningHash, entryCount, nil
}

// GetDatHashTx queries dat_hashes within a transaction context
// Use this when updating dat_hashes in the same transaction to ensure consistency
// This prevents race conditions where concurrent uploads could read stale prevHash values
func GetDatHashTx(tx *sql.Tx, datFile string) (runningHash string, entryCount int64, err error) {
	err = tx.QueryRow("SELECT running_hash, entry_count FROM dat_hashes WHERE dat_file = ?", datFile).Scan(&runningHash, &entryCount)
	if err == sql.ErrNoRows {
		return "", 0, nil
	}
	if err != nil {
		return "", 0, err
	}
	return runningHash, entryCount, nil
}

// GetDatHashRecord retrieves the complete dat hash record
func GetDatHashRecord(db *sql.DB, datFile string) (*DatHashRecord, error) {
	var rec DatHashRecord
	err := db.QueryRow(`
		SELECT dat_file, running_hash, entry_count, updated_at
		FROM dat_hashes WHERE dat_file = ?
	`, datFile).Scan(&rec.DatFile, &rec.RunningHash, &rec.EntryCount, &rec.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// UpdateDatHash inserts or replaces into dat_hashes using the provided transaction
func UpdateDatHash(tx *sql.Tx, datFile, runningHash string, entryCount int64) error {
	now := time.Now().Unix()
	_, err := tx.Exec(`
		INSERT OR REPLACE INTO dat_hashes (dat_file, running_hash, entry_count, updated_at)
		VALUES (?, ?, ?, ?)
	`, datFile, runningHash, entryCount, now)
	return err
}

// VerifyDatHash verifies that the stored hash matches the actual file hash
// Uses running hash chain verification (replays chain, O(n) in entries)
// Returns true if match, false if mismatch
func VerifyDatHash(db *sql.DB, datFile, topicPath string) (bool, error) {
	// Get stored record from database
	runningHash, entryCount, err := GetDatHash(db, datFile)
	if err != nil {
		return false, err
	}
	if runningHash == "" {
		// No hash stored yet - consider this as not verified
		return false, nil
	}

	// Verify using chain replay
	datPath := filepath.Join(topicPath, datFile)
	match, err := storage.VerifyRunningHash(datPath, runningHash, int(entryCount))
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist - mismatch
			return false, nil
		}
		return false, err
	}

	return match, nil
}

// VerifyAllDatHashes verifies all .dat files in the topic
// Returns list of mismatched files (empty = all good)
func VerifyAllDatHashes(db *sql.DB, topicPath string) ([]string, error) {
	// Query all dat_hashes entries
	rows, err := db.Query("SELECT dat_file FROM dat_hashes")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mismatched []string
	for rows.Next() {
		var datFile string
		if err := rows.Scan(&datFile); err != nil {
			return nil, err
		}

		// Verify this dat file
		match, err := VerifyDatHash(db, datFile, topicPath)
		if err != nil {
			return nil, err
		}

		if !match {
			mismatched = append(mismatched, datFile)
		}
	}

	return mismatched, rows.Err()
}
