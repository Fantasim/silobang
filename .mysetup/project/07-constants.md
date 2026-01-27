# Constants

File: `internal/constants/constants.go`

## Storage
```go
DefaultMaxDatSize     = 1073741824  // 1GB in bytes
DatFilePattern        = "%03d.dat"  // 001.dat, 002.dat, etc.
MinDatDigits          = 3
HeaderSize            = 118         // bytes per entry header
MagicBytes            = []byte("MSHB")
BlobVersion           = uint16(1)
ReservedHeaderBytes   = 32
```

## Paths
```go
ConfigDir             = ".config/meshbank"
ConfigFile            = "config.yaml"
QueriesFile           = "queries.yaml"
InternalDir           = ".internal"
OrchestratorDB        = "orchestrator.db"
MappingFile           = "mapping.json"
```

## API
```go
DefaultPort           = 2369
MaxUploadSize         = 0  // no limit, handled by max_dat_size
DefaultQueryLimit     = 1000
MaxQueryLimit         = 10000
BatchStreamBufferSize = 100
```

## Validation
```go
TopicNameRegex        = "^[a-z0-9_-]+$"
MinTopicNameLen       = 1
MaxTopicNameLen       = 64
HashLength            = 64  // BLAKE3 hex string length
```

## Database
```go
SQLitePragmas = []string{
    "PRAGMA journal_mode=WAL",
    "PRAGMA busy_timeout=5000",
    "PRAGMA synchronous=NORMAL",
    "PRAGMA cache_size=-64000",  // 64MB
    "PRAGMA foreign_keys=ON",
}
```

## Logging
```go
DefaultLogLevel       = "debug"
MaxLogEntries         = 1000  // kept in memory for /logs endpoint
```

## Pagination
```go
DefaultPageSize       = 100
MaxPageSize           = 1000
```

## File Extensions
```go
// Common 3D formats for Content-Type mapping
ExtensionMimeTypes = map[string]string{
    "glb":  "model/gltf-binary",
    "gltf": "model/gltf+json",
    "obj":  "text/plain",
    "fbx":  "application/octet-stream",
    "png":  "image/png",
    "jpg":  "image/jpeg",
    "jpeg": "image/jpeg",
}
DefaultMimeType = "application/octet-stream"
```
