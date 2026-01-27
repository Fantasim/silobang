package sanitize

import (
	"path/filepath"
	"strings"
	"unicode"

	"meshbank/internal/constants"
)

// illegalFilenameChars contains characters that are forbidden in filenames
// across common filesystems (NTFS, FAT32, ext4 compatibility).
const illegalFilenameChars = `<>:"|?*`

// Filename sanitizes a raw filename by removing path components, control characters,
// and filesystem-illegal characters. Returns an empty string if the result is empty
// after sanitization (caller decides fallback behavior).
func Filename(raw string) string {
	if raw == "" {
		return ""
	}

	// Step 1: Strip null bytes
	s := strings.ReplaceAll(raw, "\x00", "")
	if s == "" {
		return ""
	}

	// Step 2: Normalize backslashes to forward slashes so filepath.Base handles
	// Windows-style paths correctly on all platforms (Linux treats \ as valid filename char)
	s = strings.ReplaceAll(s, "\\", "/")

	// Step 3: Use filepath.Base to remove any path components (handles / and ..)
	s = filepath.Base(s)
	if s == "." || s == ".." {
		return ""
	}

	// Step 4: Strip leading dots (prevents hidden files and dot-based traversal)
	s = strings.TrimLeft(s, ".")

	// Step 5: Replace control characters with replacement char
	s = replaceControlChars(s)

	// Step 6: Replace filesystem-illegal characters
	s = replaceIllegalChars(s)

	// Step 7: Truncate to max length
	if len(s) > constants.MaxOriginNameLength {
		s = s[:constants.MaxOriginNameLength]
	}

	return s
}

// OriginName sanitizes the name portion of a filename (without extension).
// It applies full filename sanitization and trims leading/trailing whitespace
// and replacement characters from the result.
func OriginName(raw string) string {
	s := Filename(raw)
	s = strings.Trim(s, " "+constants.FilenameReplacementChar)
	return s
}

// Extension sanitizes a file extension by lowercasing it and keeping only
// alphanumeric characters. Returns an empty string if the result is empty.
func Extension(raw string) string {
	if raw == "" {
		return ""
	}

	raw = strings.ToLower(raw)
	raw = strings.TrimLeft(raw, ".")

	var b strings.Builder
	b.Grow(len(raw))
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}

	result := b.String()
	if len(result) > constants.MaxExtensionLength {
		result = result[:constants.MaxExtensionLength]
	}
	return result
}

// ContentDispositionFilename sanitizes a filename for safe use in HTTP
// Content-Disposition headers. It applies full filename sanitization and
// additionally strips characters that could cause header injection.
func ContentDispositionFilename(raw string) string {
	s := Filename(raw)
	if s == "" {
		return ""
	}

	// Strip characters that break Content-Disposition header formatting
	// or could enable header injection
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '"', '\\', '\r', '\n':
			// Skip these characters entirely
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// IsPathTraversal checks whether a string contains path traversal indicators
// including directory separators, parent directory references, null bytes,
// and common percent-encoded bypass variants.
func IsPathTraversal(s string) bool {
	if s == "" {
		return false
	}

	// Check for null bytes
	if strings.Contains(s, "\x00") {
		return true
	}

	// Check for directory separators
	if strings.ContainsAny(s, "/\\") {
		return true
	}

	// Check for parent directory traversal
	if strings.Contains(s, "..") {
		return true
	}

	// Check for common percent-encoded variants (case-insensitive)
	lower := strings.ToLower(s)
	encodedPatterns := []string{
		"%2f",    // /
		"%5c",    // \
		"%2e",    // .
		"%00",    // null
		"%c0%af", // UTF-8 overlong encoding of /
	}
	for _, pattern := range encodedPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	return false
}

// replaceControlChars replaces ASCII control characters (0x00-0x1F, 0x7F)
// with the configured replacement character.
func replaceControlChars(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.IsControl(r) {
			b.WriteString(constants.FilenameReplacementChar)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// replaceIllegalChars replaces filesystem-illegal characters with the
// configured replacement character.
func replaceIllegalChars(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if strings.ContainsRune(illegalFilenameChars, r) {
			b.WriteString(constants.FilenameReplacementChar)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
