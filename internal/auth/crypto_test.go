package auth

import (
	"strings"
	"testing"

	"silobang/internal/constants"
)

func TestHashPassword(t *testing.T) {
	password := "securePassword123!"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if hash == "" {
		t.Fatal("HashPassword returned empty hash")
	}

	if hash == password {
		t.Fatal("HashPassword returned plaintext password")
	}

	// Verify the hash starts with bcrypt prefix
	if !strings.HasPrefix(hash, "$2a$") && !strings.HasPrefix(hash, "$2b$") {
		t.Fatalf("expected bcrypt hash prefix, got: %s", hash[:4])
	}
}

func TestVerifyPassword(t *testing.T) {
	password := "securePassword123!"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	// Correct password should verify
	if err := VerifyPassword(password, hash); err != nil {
		t.Fatalf("VerifyPassword failed for correct password: %v", err)
	}

	// Wrong password should fail
	if err := VerifyPassword("wrongPassword", hash); err == nil {
		t.Fatal("VerifyPassword should fail for wrong password")
	}
}

func TestHashToken(t *testing.T) {
	token := "mbk_abc123def456"
	hash := HashToken(token)

	if hash == "" {
		t.Fatal("HashToken returned empty hash")
	}

	if hash == token {
		t.Fatal("HashToken returned the token itself")
	}

	// Same input should produce same hash
	hash2 := HashToken(token)
	if hash != hash2 {
		t.Fatal("HashToken is not deterministic")
	}

	// Different input should produce different hash
	hash3 := HashToken("different_token")
	if hash == hash3 {
		t.Fatal("HashToken produced same hash for different inputs")
	}
}

func TestGenerateAPIKey(t *testing.T) {
	key, err := GenerateAPIKey()
	if err != nil {
		t.Fatalf("GenerateAPIKey failed: %v", err)
	}

	if !strings.HasPrefix(key, constants.APIKeyPrefix) {
		t.Fatalf("API key should start with %q, got: %s", constants.APIKeyPrefix, key[:8])
	}

	// Should be reasonably long
	if len(key) < 20 {
		t.Fatalf("API key too short: %d chars", len(key))
	}

	// Two keys should be different
	key2, _ := GenerateAPIKey()
	if key == key2 {
		t.Fatal("GenerateAPIKey produced duplicate keys")
	}
}

func TestGenerateSessionToken(t *testing.T) {
	token, err := GenerateSessionToken()
	if err != nil {
		t.Fatalf("GenerateSessionToken failed: %v", err)
	}

	if !strings.HasPrefix(token, constants.SessionTokenPrefix) {
		t.Fatalf("Session token should start with %q, got: %s", constants.SessionTokenPrefix, token[:8])
	}

	if len(token) < 20 {
		t.Fatalf("Session token too short: %d chars", len(token))
	}

	// Two tokens should be different
	token2, _ := GenerateSessionToken()
	if token == token2 {
		t.Fatal("GenerateSessionToken produced duplicate tokens")
	}
}

func TestGeneratePassword(t *testing.T) {
	password, err := GeneratePassword()
	if err != nil {
		t.Fatalf("GeneratePassword failed: %v", err)
	}

	if len(password) != constants.AuthPasswordGenLength {
		t.Fatalf("expected password length %d, got %d", constants.AuthPasswordGenLength, len(password))
	}

	// Two passwords should be different
	password2, _ := GeneratePassword()
	if password == password2 {
		t.Fatal("GeneratePassword produced duplicate passwords")
	}
}

func TestIsAPIKey(t *testing.T) {
	tests := []struct {
		token    string
		expected bool
	}{
		{"mbk_abc123", true},
		{"mbs_abc123", false},
		{"Bearer abc123", false},
		{"", false},
		{"mbk_", true},
	}

	for _, tt := range tests {
		if got := IsAPIKey(tt.token); got != tt.expected {
			t.Errorf("IsAPIKey(%q) = %v, want %v", tt.token, got, tt.expected)
		}
	}
}

func TestIsSessionToken(t *testing.T) {
	tests := []struct {
		token    string
		expected bool
	}{
		{"mbs_abc123", true},
		{"mbk_abc123", false},
		{"Bearer abc123", false},
		{"", false},
		{"mbs_", true},
	}

	for _, tt := range tests {
		if got := IsSessionToken(tt.token); got != tt.expected {
			t.Errorf("IsSessionToken(%q) = %v, want %v", tt.token, got, tt.expected)
		}
	}
}

func TestExtractTokenPrefix(t *testing.T) {
	key := "mbk_abcdefghijklmnop"
	prefix := ExtractTokenPrefix(key)

	if len(prefix) != constants.AuthAPIKeyPrefixLength {
		t.Fatalf("expected prefix length %d, got %d", constants.AuthAPIKeyPrefixLength, len(prefix))
	}

	if prefix != "mbk_abcd" {
		t.Fatalf("expected prefix %q, got %q", "mbk_abcd", prefix)
	}
}

func TestBase62Encode(t *testing.T) {
	// Basic test: encoding should produce non-empty string
	data := []byte{0xff, 0xfe, 0xfd, 0xfc}
	encoded := base62Encode(data)
	if encoded == "" {
		t.Fatal("base62Encode returned empty string")
	}

	// Should only contain base62 characters
	for _, c := range encoded {
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')) {
			t.Fatalf("base62Encode produced invalid character: %c", c)
		}
	}

	// Zero bytes
	zeroEncoded := base62Encode([]byte{0})
	if zeroEncoded == "" {
		t.Fatal("base62Encode returned empty for zero byte")
	}
}
