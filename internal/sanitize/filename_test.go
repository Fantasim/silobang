package sanitize

import (
	"strings"
	"testing"

	"meshbank/internal/constants"
)

func TestFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Normal filenames
		{"normal_file", "photo.jpg", "photo.jpg"},
		{"normal_with_spaces", "my file.txt", "my file.txt"},
		{"normal_with_hyphens", "my-file-name.txt", "my-file-name.txt"},
		{"normal_with_underscores", "my_file_name.txt", "my_file_name.txt"},
		{"no_extension", "README", "README"},
		{"multiple_dots", "archive.tar.gz", "archive.tar.gz"},

		// Path traversal
		{"unix_path_traversal", "../../../etc/passwd", "passwd"},
		{"windows_path_traversal", "..\\..\\..\\windows\\system32", "system32"},
		{"mixed_separators", "..\\../..\\../etc/passwd", "passwd"},
		{"double_dot_slash", "....//....//etc/passwd", "passwd"},
		{"dot_dot_slash_concat", "..././..././etc/passwd", "passwd"},
		{"semicolon_traversal", "..;/..;/etc/passwd", "passwd"},
		{"absolute_unix_path", "/etc/passwd", "passwd"},
		{"absolute_windows_path", "C:\\Windows\\system32\\config", "config"},

		// Null bytes
		{"null_byte_in_name", "file\x00evil.txt", "fileevil.txt"},
		{"null_byte_with_traversal", "../\x00../etc/passwd", "passwd"},
		{"only_null_bytes", "\x00\x00\x00", ""},

		// Control characters
		{"control_chars", "file\x01\x02\x03.txt", "file___.txt"},
		{"tab_in_name", "file\tname.txt", "file_name.txt"},
		{"newline_in_name", "file\nname.txt", "file_name.txt"},
		{"carriage_return", "file\rname.txt", "file_name.txt"},
		{"delete_char", "file\x7fname.txt", "file_name.txt"},

		// Filesystem-illegal characters
		{"angle_brackets", "file<name>.txt", "file_name_.txt"},
		{"colon", "file:name.txt", "file_name.txt"},
		{"double_quote", "file\"name.txt", "file_name.txt"},
		{"pipe", "file|name.txt", "file_name.txt"},
		{"question_mark", "file?name.txt", "file_name.txt"},
		{"asterisk", "file*name.txt", "file_name.txt"},
		{"all_illegal_chars", "<>:\"|?*.txt", "_______.txt"},

		// Leading dots
		{"hidden_file", ".hidden", "hidden"},
		{"double_dot_prefix", "..hidden", "hidden"},
		{"triple_dot_prefix", "...hidden", "hidden"},
		{"dots_only", "...", ""},
		{"single_dot", ".", ""},

		// Empty and edge cases
		{"empty_string", "", ""},
		{"only_spaces", "   ", "   "},

		// Length truncation
		{"max_length", strings.Repeat("a", constants.MaxOriginNameLength), strings.Repeat("a", constants.MaxOriginNameLength)},
		{"over_max_length", strings.Repeat("a", constants.MaxOriginNameLength+100), strings.Repeat("a", constants.MaxOriginNameLength)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := Filename(tc.input)
			if result != tc.expected {
				t.Errorf("Filename(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestOriginName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Normal names
		{"normal_name", "photo", "photo"},
		{"name_with_spaces", "my photo", "my photo"},
		{"name_with_hyphens", "my-photo", "my-photo"},

		// Path traversal stripped
		{"path_traversal", "../../../etc/photo", "photo"},
		{"windows_traversal", "..\\..\\photo", "photo"},

		// Trims replacement chars and spaces
		{"leading_underscores", "___photo", "photo"},
		{"trailing_underscores", "photo___", "photo"},
		{"leading_spaces", "   photo", "photo"},
		{"trailing_spaces", "photo   ", "photo"},
		{"both_sides", "___photo___", "photo"},

		// Edge cases
		{"empty", "", ""},
		{"only_underscores", "___", ""},
		{"only_dots", "...", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := OriginName(tc.input)
			if result != tc.expected {
				t.Errorf("OriginName(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestExtension(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Normal extensions
		{"lowercase", "jpg", "jpg"},
		{"uppercase", "JPG", "jpg"},
		{"mixed_case", "JpG", "jpg"},
		{"longer_ext", "glb", "glb"},
		{"numbers", "mp4", "mp4"},

		// Leading dot
		{"with_dot_prefix", ".jpg", "jpg"},

		// Special characters stripped
		{"special_chars", "j p g", "jpg"},
		{"path_in_ext", "../../../etc", "etc"},
		{"null_in_ext", "jp\x00g", "jpg"},
		{"symbols_in_ext", "j.p.g", "jpg"},

		// Length
		{"max_length", strings.Repeat("a", constants.MaxExtensionLength), strings.Repeat("a", constants.MaxExtensionLength)},
		{"over_max_length", strings.Repeat("a", constants.MaxExtensionLength+10), strings.Repeat("a", constants.MaxExtensionLength)},

		// Edge cases
		{"empty", "", ""},
		{"only_special", "...", ""},
		{"only_spaces", "   ", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := Extension(tc.input)
			if result != tc.expected {
				t.Errorf("Extension(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestContentDispositionFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Normal filenames
		{"normal_file", "photo.jpg", "photo.jpg"},
		{"with_spaces", "my file.txt", "my file.txt"},

		// Header injection prevention
		{"double_quote", "file\"name.txt", "file_name.txt"},
		{"backslash_stripped_by_base", "file\\name.txt", "name.txt"},
		{"newline_injection", "file\r\nX-Evil: yes.txt", "file__X-Evil_ yes.txt"},
		{"carriage_return", "file\rname.txt", "file_name.txt"},

		// Path traversal
		{"path_traversal", "../../../etc/passwd.txt", "passwd.txt"},

		// Edge cases
		{"empty", "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ContentDispositionFilename(tc.input)
			if result != tc.expected {
				t.Errorf("ContentDispositionFilename(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestIsPathTraversal(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// Positive cases (is traversal)
		{"unix_traversal", "../something", true},
		{"windows_traversal", "..\\something", true},
		{"forward_slash", "path/file", true},
		{"backslash", "path\\file", true},
		{"double_dot", "..file", true},
		{"null_byte", "file\x00.txt", true},
		{"encoded_slash", "..%2f..%2f", true},
		{"encoded_backslash", "..%5c..%5c", true},
		{"encoded_dot", "%2e%2e/etc", true},
		{"encoded_null", "file%00.txt", true},
		{"utf8_overlong", "%c0%af..%c0%af", true},
		{"uppercase_encoded", "..%2F..%2F", true},

		// Negative cases (not traversal)
		{"normal_file", "photo.jpg", false},
		{"normal_name", "filename", false},
		{"empty", "", false},
		{"single_dot", "file.txt", false},
		{"underscore", "file_name.txt", false},
		{"hyphen", "file-name.txt", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := IsPathTraversal(tc.input)
			if result != tc.expected {
				t.Errorf("IsPathTraversal(%q) = %v, want %v", tc.input, result, tc.expected)
			}
		})
	}
}

// TestFilename_SecurityPayloads tests with real-world attack payloads
func TestFilename_SecurityPayloads(t *testing.T) {
	payloads := []string{
		"../../../etc/passwd",
		"..\\..\\..\\windows\\system32\\config",
		"....//....//....//etc/passwd",
		"..%2F..%2F..%2Fetc/passwd",
		"..%252F..%252F..%252Fetc/passwd",
		"..%c0%af..%c0%afetc/passwd",
		"%2e%2e%2f%2e%2e%2f%2e%2e%2fetc/passwd",
		"..././..././..././etc/passwd",
		"..;/..;/..;/etc/passwd",
		"file\x00.txt",
		"file.txt\x00.exe",
		"../\x00../etc/passwd",
		"test\x00/../../../etc/passwd",
	}

	for _, payload := range payloads {
		t.Run("payload", func(t *testing.T) {
			result := Filename(payload)

			// Result must not contain path traversal sequences
			if strings.Contains(result, "..") {
				t.Errorf("Filename(%q) = %q, still contains path traversal", payload, result)
			}
			if strings.ContainsAny(result, "/\\") {
				t.Errorf("Filename(%q) = %q, still contains directory separator", payload, result)
			}
			if strings.Contains(result, "\x00") {
				t.Errorf("Filename(%q) = %q, still contains null byte", payload, result)
			}
		})
	}
}

// TestFilename_NoFalsePositives ensures normal filenames pass through unchanged
func TestFilename_NoFalsePositives(t *testing.T) {
	normalNames := []string{
		"document.pdf",
		"image.png",
		"model.glb",
		"archive.tar.gz",
		"file-with-dashes.txt",
		"file_with_underscores.txt",
		"file with spaces.txt",
		"UPPERCASE.TXT",
		"MixedCase.Jpg",
		"123456.dat",
	}

	for _, name := range normalNames {
		t.Run(name, func(t *testing.T) {
			result := Filename(name)
			if result != name {
				t.Errorf("Filename(%q) = %q, normal filename should pass through unchanged", name, result)
			}
		})
	}
}
