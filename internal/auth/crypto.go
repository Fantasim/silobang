package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/zeebo/blake3"
	"golang.org/x/crypto/bcrypt"

	"meshbank/internal/constants"
)

// base62Alphabet is used for human-friendly token encoding (no special chars).
const base62Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// HashPassword hashes a plaintext password using bcrypt with the configured cost.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), constants.AuthBcryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hash), nil
}

// VerifyPassword checks a plaintext password against a bcrypt hash.
// Returns nil on success, error on mismatch or failure.
func VerifyPassword(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// HashToken computes a BLAKE3 hash of a token or API key for storage.
// The plaintext is never stored — only the hash.
func HashToken(token string) string {
	hasher := blake3.New()
	hasher.Write([]byte(token))
	return hex.EncodeToString(hasher.Sum(nil))
}

// GenerateAPIKey creates a new API key with the mbk_ prefix.
// Returns the plaintext key (shown once to the user).
func GenerateAPIKey() (string, error) {
	encoded, err := generateBase62(constants.AuthAPIKeyRandomBytes)
	if err != nil {
		return "", fmt.Errorf("failed to generate API key: %w", err)
	}
	return constants.APIKeyPrefix + encoded, nil
}

// GenerateSessionToken creates a new session token with the mbs_ prefix.
// Returns the plaintext token (sent to the client).
func GenerateSessionToken() (string, error) {
	encoded, err := generateBase62(constants.AuthSessionTokenBytes)
	if err != nil {
		return "", fmt.Errorf("failed to generate session token: %w", err)
	}
	return constants.SessionTokenPrefix + encoded, nil
}

// GeneratePassword creates a cryptographically secure random password.
// Uses a mix of lowercase, uppercase, digits, and special characters.
func GeneratePassword() (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%&*"
	length := constants.AuthPasswordGenLength

	password := make([]byte, length)
	charsetLen := big.NewInt(int64(len(charset)))

	for i := range password {
		idx, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			return "", fmt.Errorf("failed to generate password: %w", err)
		}
		password[i] = charset[idx.Int64()]
	}

	return string(password), nil
}

// ExtractTokenPrefix returns the first N characters of a token for logging.
func ExtractTokenPrefix(token string) string {
	if len(token) <= constants.AuthAPIKeyPrefixLength {
		return token
	}
	return token[:constants.AuthAPIKeyPrefixLength]
}

// IsAPIKey checks if a token has the API key prefix.
func IsAPIKey(token string) bool {
	return strings.HasPrefix(token, constants.APIKeyPrefix)
}

// IsSessionToken checks if a token has the session token prefix.
func IsSessionToken(token string) bool {
	return strings.HasPrefix(token, constants.SessionTokenPrefix)
}

// generateBase62 generates random bytes and encodes them to base62.
func generateBase62(numBytes int) (string, error) {
	randomBytes := make([]byte, numBytes)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	return base62Encode(randomBytes), nil
}

// base62Encode encodes raw bytes to a base62 string.
func base62Encode(data []byte) string {
	// Convert bytes to a big integer
	num := new(big.Int).SetBytes(data)
	base := big.NewInt(int64(len(base62Alphabet)))

	if num.Sign() == 0 {
		return string(base62Alphabet[0])
	}

	var result []byte
	zero := big.NewInt(0)
	mod := new(big.Int)

	for num.Cmp(zero) > 0 {
		num.DivMod(num, base, mod)
		result = append(result, base62Alphabet[mod.Int64()])
	}

	// Reverse the result
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return string(result)
}
