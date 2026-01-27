package database

import (
	"database/sql"
)

// CheckHashExists queries asset_index for the given hash
// Returns whether it exists and where (topic + dat_file)
func CheckHashExists(db *sql.DB, hash string) (exists bool, topic string, datFile string, err error) {
	var t, df string
	err = db.QueryRow("SELECT topic, dat_file FROM asset_index WHERE hash = ?", hash).Scan(&t, &df)

	if err == sql.ErrNoRows {
		return false, "", "", nil
	}
	if err != nil {
		return false, "", "", err
	}

	return true, t, df, nil
}

// InsertAssetIndex inserts into asset_index table using the provided transaction
// Used for atomic writes as part of the write pipeline
func InsertAssetIndex(tx *sql.Tx, hash, topic, datFile string) error {
	_, err := tx.Exec("INSERT INTO asset_index (hash, topic, dat_file) VALUES (?, ?, ?)", hash, topic, datFile)
	return err
}

// InsertAssetIndexIgnore uses INSERT OR IGNORE for re-indexing discovered topics
// Uses its own transaction (not part of write pipeline)
func InsertAssetIndexIgnore(db *sql.DB, hash, topic, datFile string) error {
	_, err := db.Exec("INSERT OR IGNORE INTO asset_index (hash, topic, dat_file) VALUES (?, ?, ?)", hash, topic, datFile)
	return err
}

// DeleteAssetIndex deletes from asset_index (for future use when deletion is supported)
func DeleteAssetIndex(tx *sql.Tx, hash string) error {
	_, err := tx.Exec("DELETE FROM asset_index WHERE hash = ?", hash)
	return err
}

// ListIndexedTopics returns all distinct topic names referenced in asset_index
func ListIndexedTopics(db *sql.DB) ([]string, error) {
	rows, err := db.Query("SELECT DISTINCT topic FROM asset_index")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var topics []string
	for rows.Next() {
		var topic string
		if err := rows.Scan(&topic); err != nil {
			return nil, err
		}
		topics = append(topics, topic)
	}
	return topics, rows.Err()
}

// DeleteAssetIndexByTopic removes all asset_index entries for a given topic.
// Returns the number of rows deleted.
func DeleteAssetIndexByTopic(db *sql.DB, topic string) (int64, error) {
	result, err := db.Exec("DELETE FROM asset_index WHERE topic = ?", topic)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
