package queries

import "meshbank/internal/constants"

// GetDefaultStats returns the embedded default topic stats
func GetDefaultStats() []TopicStat {
	return []TopicStat{
		{
			Name:   "total_size",
			Label:  "Total Size",
			SQL:    "SELECT SUM(asset_size) FROM assets",
			Format: constants.StatFormatBytes,
		},
		{
			Name:  "db_size",
			Label: "DB Size",
			Type:  "file_size",
		},
		{
			Name:  "dat_size",
			Label: "DAT Size",
			Type:  "dat_total",
		},
		{
			Name:   "file_count",
			Label:  "Files",
			SQL:    "SELECT COUNT(*) FROM assets",
			Format: constants.StatFormatNumber,
		},
		{
			Name:   "avg_size",
			Label:  "Avg Size",
			SQL:    "SELECT AVG(asset_size) FROM assets",
			Format: constants.StatFormatFloat,
		},
		{
			Name:   "last_added",
			Label:  "Last Added",
			SQL:    "SELECT MAX(created_at) FROM assets",
			Format: constants.StatFormatDate,
		},
		{
			Name:   "last_hash",
			Label:  "Last Hash",
			SQL:    "SELECT asset_id FROM assets ORDER BY created_at DESC LIMIT 1",
			Format: constants.StatFormatText,
		},
		{
			Name:   "unique_extensions",
			Label:  "Extensions",
			SQL:    "SELECT COUNT(DISTINCT extension) FROM assets",
			Format: constants.StatFormatNumber,
		},
		{
			Name:   "versioned_count",
			Label:  "Versioned",
			SQL:    "SELECT COUNT(*) FROM assets WHERE parent_id IS NOT NULL",
			Format: constants.StatFormatNumber,
		},
		{
			Name:   "root_count",
			Label:  "Root Assets",
			SQL:    "SELECT COUNT(*) FROM assets WHERE parent_id IS NULL",
			Format: constants.StatFormatNumber,
		},
		{
			Name:   "metadata_coverage",
			Label:  "With Metadata",
			SQL:    "SELECT COUNT(*) FROM metadata_computed",
			Format: constants.StatFormatNumber,
		},
		{
			Name:   "oldest_asset",
			Label:  "Oldest",
			SQL:    "SELECT MIN(created_at) FROM assets",
			Format: constants.StatFormatDate,
		},
		{
			Name:   "dat_file_count",
			Label:  "DAT Files",
			SQL:    "SELECT COUNT(*) FROM dat_hashes",
			Format: constants.StatFormatNumber,
		},
	}
}

// GetDefaultPresets returns the embedded default query presets
func GetDefaultPresets() map[string]Preset {
	return map[string]Preset{
		// Basic Queries
		"recent-imports": {
			Description: "Assets imported in last N days",
			SQL: `SELECT asset_id, origin_name, extension, asset_size, parent_id, blob_name, created_at
FROM assets
WHERE created_at >= strftime('%s', 'now') - (:days * 86400)
ORDER BY created_at DESC
LIMIT :limit`,
			Params: []PresetParam{
				{Name: "days", Default: constants.DefaultPresetDays},
				{Name: "limit", Default: constants.DefaultPresetLimit},
			},
		},
		"by-hash": {
			Description: "Find by hash prefix",
			SQL: `SELECT asset_id, origin_name, extension, asset_size, parent_id, blob_name, created_at
FROM assets
WHERE asset_id LIKE :hash || '%'
LIMIT 10`,
			Params: []PresetParam{
				{Name: "hash", Required: true},
			},
		},
		"large-files": {
			Description: "Files larger than N bytes",
			SQL: `SELECT asset_id, origin_name, extension, asset_size, parent_id, blob_name, created_at
FROM assets
WHERE asset_size > :min_size
ORDER BY asset_size DESC
LIMIT :limit`,
			Params: []PresetParam{
				{Name: "min_size", Default: constants.DefaultLargeFileSize},
				{Name: "limit", Default: constants.DefaultPresetSmallLimit},
			},
		},
		"count": {
			Description: "Total file count",
			SQL:         "SELECT COUNT(*) as count FROM assets",
		},

		// Analytics & Aggregation
		"extension-summary": {
			Description: "Summary of files by extension",
			SQL: `SELECT extension,
       COUNT(*) as count,
       SUM(asset_size) as total_size,
       AVG(asset_size) as avg_size,
       MIN(created_at) as oldest,
       MAX(created_at) as newest
FROM assets
GROUP BY extension
ORDER BY count DESC
LIMIT :limit`,
			Params: []PresetParam{
				{Name: "limit", Default: constants.DefaultPresetMediumLimit},
			},
		},
		"size-distribution": {
			Description: "Assets grouped by size range",
			SQL: `SELECT
  CASE
    WHEN asset_size < 1024 THEN '1-tiny (<1KB)'
    WHEN asset_size < 1048576 THEN '2-small (1KB-1MB)'
    WHEN asset_size < 10485760 THEN '3-medium (1MB-10MB)'
    WHEN asset_size < 104857600 THEN '4-large (10MB-100MB)'
    ELSE '5-huge (>100MB)'
  END as size_range,
  COUNT(*) as count,
  SUM(asset_size) as total_size,
  AVG(asset_size) as avg_size
FROM assets
GROUP BY size_range
ORDER BY size_range`,
		},
		"time-series": {
			Description: "Upload activity by day",
			SQL: `SELECT
  date(created_at, 'unixepoch') as date,
  COUNT(*) as count,
  SUM(asset_size) as total_size
FROM assets
WHERE created_at >= strftime('%s', 'now') - (:days * 86400)
GROUP BY date
ORDER BY date DESC`,
			Params: []PresetParam{
				{Name: "days", Default: constants.DefaultTimeSeriesDays},
			},
		},

		// Search & Filter
		"by-extension": {
			Description: "Find assets by file extension",
			SQL: `SELECT asset_id, origin_name, extension, asset_size, parent_id, blob_name, created_at
FROM assets
WHERE extension = :ext
ORDER BY created_at DESC
LIMIT :limit`,
			Params: []PresetParam{
				{Name: "ext", Required: true},
				{Name: "limit", Default: constants.DefaultPresetLimit},
			},
		},
		"by-origin-name": {
			Description: "Search assets by original filename",
			SQL: `SELECT asset_id, origin_name, extension, asset_size, parent_id, blob_name, created_at
FROM assets
WHERE origin_name LIKE '%' || :name || '%'
ORDER BY origin_name, created_at DESC
LIMIT :limit`,
			Params: []PresetParam{
				{Name: "name", Required: true},
				{Name: "limit", Default: constants.DefaultPresetLimit},
			},
		},

		// Lineage Analysis
		"lineage": {
			Description: "Get asset lineage chain (ancestors)",
			SQL: `WITH RECURSIVE chain AS (
  SELECT asset_id, parent_id, origin_name, extension, asset_size, blob_name, created_at, 0 as depth
  FROM assets WHERE asset_id = :hash
  UNION ALL
  SELECT a.asset_id, a.parent_id, a.origin_name, a.extension, a.asset_size, a.blob_name, a.created_at, c.depth + 1
  FROM assets a JOIN chain c ON a.asset_id = c.parent_id
)
SELECT * FROM chain ORDER BY depth`,
			Params: []PresetParam{
				{Name: "hash", Required: true},
			},
		},
		"derived": {
			Description: "All versions derived from asset (descendants)",
			SQL: `WITH RECURSIVE descendants AS (
  SELECT asset_id, parent_id, origin_name, extension, asset_size, blob_name, created_at, 1 as depth
  FROM assets WHERE parent_id = :hash
  UNION ALL
  SELECT a.asset_id, a.parent_id, a.origin_name, a.extension, a.asset_size, a.blob_name, a.created_at, d.depth + 1
  FROM assets a JOIN descendants d ON a.parent_id = d.asset_id
)
SELECT * FROM descendants ORDER BY depth, created_at`,
			Params: []PresetParam{
				{Name: "hash", Required: true},
			},
		},
		"orphans": {
			Description: "Root assets with no children (potential cleanup candidates)",
			SQL: `SELECT a.asset_id, a.origin_name, a.extension, a.asset_size, a.blob_name, a.created_at
FROM assets a
LEFT JOIN assets children ON children.parent_id = a.asset_id
WHERE a.parent_id IS NULL
  AND children.asset_id IS NULL
ORDER BY a.created_at DESC
LIMIT :limit`,
			Params: []PresetParam{
				{Name: "limit", Default: constants.DefaultPresetLimit},
			},
		},
		"roots-with-children": {
			Description: "Root assets that have derived versions",
			SQL: `SELECT a.asset_id, a.origin_name, a.extension, a.asset_size, a.blob_name, a.created_at,
       (SELECT COUNT(*) FROM assets d WHERE d.parent_id = a.asset_id) as direct_children
FROM assets a
WHERE a.parent_id IS NULL
  AND EXISTS (SELECT 1 FROM assets c WHERE c.parent_id = a.asset_id)
ORDER BY direct_children DESC, a.created_at DESC
LIMIT :limit`,
			Params: []PresetParam{
				{Name: "limit", Default: constants.DefaultPresetLimit},
			},
		},

		// Metadata Queries
		"with-metadata": {
			Description: "Assets with specific metadata key",
			SQL: `SELECT a.asset_id, a.origin_name, a.extension, a.asset_size, a.created_at, mc.metadata_json, mc.updated_at
FROM assets a
JOIN metadata_computed mc ON a.asset_id = mc.asset_id
WHERE mc.metadata_json LIKE '%' || :key || '%'
LIMIT :limit`,
			Params: []PresetParam{
				{Name: "key", Required: true},
				{Name: "limit", Default: constants.DefaultPresetLimit},
			},
		},
		"by-processor": {
			Description: "Metadata from specific processor",
			SQL: `SELECT asset_id, key, value_text, value_num, processor_version, timestamp
FROM metadata_log
WHERE processor = :processor
ORDER BY timestamp DESC
LIMIT :limit`,
			Params: []PresetParam{
				{Name: "processor", Required: true},
				{Name: "limit", Default: constants.DefaultPresetLimit},
			},
		},
		"metadata-history": {
			Description: "View metadata change history for an asset",
			SQL: `SELECT id, op, key, value_text, value_num, processor, processor_version, timestamp
FROM metadata_log
WHERE asset_id = :hash
ORDER BY timestamp DESC
LIMIT :limit`,
			Params: []PresetParam{
				{Name: "hash", Required: true},
				{Name: "limit", Default: constants.DefaultPresetLimit},
			},
		},
		"without-metadata": {
			Description: "Assets without computed metadata",
			SQL: `SELECT a.asset_id, a.origin_name, a.extension, a.asset_size, a.blob_name, a.created_at
FROM assets a
LEFT JOIN metadata_computed mc ON a.asset_id = mc.asset_id
WHERE mc.asset_id IS NULL
ORDER BY a.created_at DESC
LIMIT :limit`,
			Params: []PresetParam{
				{Name: "limit", Default: constants.DefaultPresetLimit},
			},
		},

		// Storage Analysis
		"dat-file-stats": {
			Description: "Statistics per DAT file",
			SQL: `SELECT dh.dat_file,
       dh.entry_count,
       dh.running_hash,
       dh.updated_at,
       (SELECT SUM(asset_size) FROM assets WHERE blob_name = dh.dat_file) as total_data_size,
       (SELECT COUNT(*) FROM assets WHERE blob_name = dh.dat_file) as asset_count
FROM dat_hashes dh
ORDER BY dh.dat_file`,
		},
	}
}

// GetDefaultConfig returns the complete default queries configuration
func GetDefaultConfig() *QueriesConfig {
	return &QueriesConfig{
		TopicStats: GetDefaultStats(),
		Presets:    GetDefaultPresets(),
	}
}
