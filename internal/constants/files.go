package constants

import "os"

// File Permissions
const (
	DirPermissions  os.FileMode = 0755 // Directory creation permissions
	FilePermissions os.FileMode = 0644 // File creation permissions
)

// Form Field Names (multipart form uploads)
const (
	FormFieldFile     = "file"
	FormFieldParentID = "parent_id"
)

// Filename Sanitization
const (
	MaxOriginNameLength     = 255 // Maximum allowed length for an asset origin name
	MaxExtensionLength      = 32  // Maximum allowed length for a file extension
	FilenameReplacementChar = "_" // Character used to replace invalid characters in filenames
)
