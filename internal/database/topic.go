package database

import (
	"database/sql"
)

// Asset represents an asset record in the database
type Asset struct {
	AssetID    string  // BLAKE3 hash (64 hex chars)
	AssetSize  int64   // bytes
	OriginName string  // original filename without extension
	ParentID   *string // nullable, for lineage
	Extension  string  // file extension without dot
	BlobName   string  // which .dat file (e.g., "003.dat")
	ByteOffset int64   // offset in .dat file for O(1) lookup
	CreatedAt  int64   // unix timestamp
}

// InsertAsset inserts an asset into the assets table using the provided transaction
func InsertAsset(tx *sql.Tx, asset Asset) error {
	_, err := tx.Exec(`
		INSERT INTO assets (asset_id, asset_size, origin_name, parent_id, extension, blob_name, byte_offset, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, asset.AssetID, asset.AssetSize, asset.OriginName, asset.ParentID, asset.Extension, asset.BlobName, asset.ByteOffset, asset.CreatedAt)
	return err
}

// GetAsset queries a single asset by hash
func GetAsset(db *sql.DB, assetID string) (*Asset, error) {
	var asset Asset
	var parentID sql.NullString

	err := db.QueryRow(`
		SELECT asset_id, asset_size, origin_name, parent_id, extension, blob_name, byte_offset, created_at
		FROM assets WHERE asset_id = ?
	`, assetID).Scan(
		&asset.AssetID,
		&asset.AssetSize,
		&asset.OriginName,
		&parentID,
		&asset.Extension,
		&asset.BlobName,
		&asset.ByteOffset,
		&asset.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if parentID.Valid {
		asset.ParentID = &parentID.String
	}

	return &asset, nil
}

// GetAssetsByParent queries all assets with given parent_id
func GetAssetsByParent(db *sql.DB, parentID string) ([]Asset, error) {
	rows, err := db.Query(`
		SELECT asset_id, asset_size, origin_name, parent_id, extension, blob_name, byte_offset, created_at
		FROM assets WHERE parent_id = ?
	`, parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var assets []Asset
	for rows.Next() {
		var asset Asset
		var pid sql.NullString

		err := rows.Scan(
			&asset.AssetID,
			&asset.AssetSize,
			&asset.OriginName,
			&pid,
			&asset.Extension,
			&asset.BlobName,
			&asset.ByteOffset,
			&asset.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		if pid.Valid {
			asset.ParentID = &pid.String
		}

		assets = append(assets, asset)
	}

	return assets, rows.Err()
}

// ValidateParentExists checks if parent_id exists in ANY topic via orchestrator.db
// Returns error if not found
func ValidateParentExists(orchestratorDB *sql.DB, parentID string) error {
	var exists bool
	err := orchestratorDB.QueryRow("SELECT EXISTS(SELECT 1 FROM asset_index WHERE hash = ?)", parentID).Scan(&exists)
	if err != nil {
		return err
	}

	if !exists {
		return sql.ErrNoRows
	}

	return nil
}
