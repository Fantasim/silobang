package constants

// Storage
const (
	DefaultMaxDatSize   = 1073741824   // 1GB in bytes
	DatFilePattern      = "%06d.dat"   // 000001.dat, 000002.dat, etc. (supports up to 999,999 files)
	MinDatDigits        = 3            // Minimum digits for regex matching (backward compatible with 3-digit files)
	FirstDatFilename    = "000001.dat" // Initial dat file for new topics
	HeaderSize          = 110          // bytes per entry header (see phase-03 for format)
	BlobVersion         = uint16(1)
	ReservedHeaderBytes = 32
)

var MagicBytes = []byte("MSHB")

// Header field offsets (all multi-byte integers are little-endian)
const (
	MagicBytesOffset = 0   // 4 bytes: "MSHB"
	VersionOffset    = 4   // 2 bytes: uint16
	DataLengthOffset = 6   // 8 bytes: uint64
	HashOffset       = 14  // 64 bytes: ASCII hex
	ReservedOffset   = 78  // 32 bytes: zero-filled
	DataStartOffset  = 110 // where asset data begins
)

// Paths
const (
	ConfigDir      = ".config/meshbank"
	ConfigFile     = "config.yaml"
	InternalDir    = ".internal"
	OrchestratorDB = "orchestrator.db"
)

// Prompts
const (
	PromptsDir          = "prompts"
	PromptFileExtension = ".prompt.yaml"
)

// Queries Directory Structure
const (
	QueriesDir        = "queries"
	QueriesStatsDir   = "stats"
	QueriesPresetsDir = "presets"
	QueryFileExt      = ".yaml"
)

// Query Validation
const (
	QueryNameRegex  = `^[a-z0-9_-]+$`
	MinQueryNameLen = 1
	MaxQueryNameLen = 64
)

// Orchestrator Queries
const (
	OrchestratorCountHashesQuery = "SELECT COUNT(*) FROM asset_index"
)

// API
const (
	DefaultPort       = 2369
	DefaultQueryLimit = 1000
	MaxQueryLimit     = 10000
)

// Query Preset Defaults
const (
	DefaultPresetDays        = "7"
	DefaultPresetLimit       = "100"
	DefaultLargeFileSize     = "10000000" // 10MB in bytes
	DefaultTimeSeriesDays    = "30"
	DefaultPresetSmallLimit  = "20"
	DefaultPresetMediumLimit = "50"
)

// Stat Format Types
const (
	StatFormatBytes  = "bytes"
	StatFormatNumber = "number"
	StatFormatFloat  = "float"
	StatFormatDate   = "date"
	StatFormatText   = "text"
)

// Validation
const (
	TopicNameRegex  = `^[a-z0-9_-]+$`
	MinTopicNameLen = 1
	MaxTopicNameLen = 64
	HashLength      = 64 // BLAKE3 hex string length (32 bytes = 64 hex chars)
)

// Database pragmas (optimized for low memory: < 2GB RAM)
var SQLitePragmas = []string{
	"PRAGMA journal_mode=WAL",
	"PRAGMA busy_timeout=5000",
	"PRAGMA synchronous=NORMAL",
	"PRAGMA cache_size=-8000", // 8MB per connection (reduced for low memory)
	"PRAGMA foreign_keys=ON",
}

// Logging
const (
	DefaultLogLevel    = "debug"
	LogsDir            = "logs"
	LogsDirDebug       = "debug"
	LogsDirInfo        = "info"
	LogsDirWarn        = "warn"
	LogsDirError       = "error"
	LogFileExtension   = ".log"
	LogTimestampFormat = "2006-01-02 15:04:05"
)

// Shutdown
const (
	ShutdownTimeoutSecs = 10
)

// Pagination
const (
	DefaultPageSize = 100
	MaxPageSize     = 1000
)

// MIME types
var ExtensionMimeTypes = map[string]string{
	"glb":  "model/gltf-binary",
	"gltf": "model/gltf+json",
	"obj":  "text/plain",
	"fbx":  "application/octet-stream",
	"png":  "image/png",
	"jpg":  "image/jpeg",
	"jpeg": "image/jpeg",
}

const DefaultMimeType = "application/octet-stream"

// Bulk Download
const (
	MimeTypeZIP             = "application/zip"
	BulkDownloadMaxAssets   = 900_000_000
	DefaultFilenameFormat   = FilenameFormatOriginal
	ManifestFilename        = "manifest.json"
	BulkDownloadAssetsDir   = "assets"
	BulkDownloadMetadataDir = "metadata"
)

// Filename formats for bulk download
const (
	FilenameFormatHash         = "hash"
	FilenameFormatOriginal     = "original"
	FilenameFormatHashOriginal = "hash_original"
)

// Bulk Download SSE
const (
	BulkDownloadTempDir          = "downloads" // Subdirectory under .internal
	BulkDownloadSessionTTLMins   = 120         // Session expiration in minutes
	BulkDownloadCleanupMins      = 5           // Cleanup check interval in minutes
	BulkDownloadProgressInterval = 100         // Report progress every N assets
	BulkDownloadIDLength         = 16          // Length of random download ID
	BulkDownloadFilePattern      = "*.zip"     // Pattern for cleanup glob
)

// Batch Metadata Operations
const (
	BatchMetadataMaxOperations = 100000   // Maximum operations per batch request
	BatchMetadataOpSet         = "set"    // Set metadata operation
	BatchMetadataOpDelete      = "delete" // Delete metadata operation
)

// Metadata Processors
const (
	ProcessorAPI = "api" // Direct API calls
)

// Metadata Validation
const (
	MaxMetadataKeyLength  = 256      // Maximum characters for metadata key
	MaxMetadataValueBytes = 10485760 // Maximum bytes for metadata value (10MB)
)

// Verification
const (
	DefaultVerifyProgressInterval = 100 // Report progress every N entries
	MaxVerifyIssuesInResponse     = 100 // Maximum issues to include in response
)

// Monitoring
const (
	MonitoringLogFileMaxReadBytes = 5 * 1024 * 1024 // 5MB cap per log file read
)
