package security

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/pbkdf2"
)

const (
	PBKDF2Iterations = 100000
	KeyLength        = 32
	SaltLength       = 16
)

// HashPassword hashes a password using PBKDF2-HMAC-SHA256 with 100,000 iterations.
// Compatible with Python's hashlib.pbkdf2_hmac.
func HashPassword(password string, salt string) (hash string, generatedSalt string, err error) {
	if salt == "" {
		saltBytes := make([]byte, SaltLength)
		if _, err := rand.Read(saltBytes); err != nil {
			return "", "", fmt.Errorf("failed to generate random salt: %w", err)
		}
		salt = hex.EncodeToString(saltBytes)
	}

	key := pbkdf2.Key([]byte(password), []byte(salt), PBKDF2Iterations, KeyLength, sha256.New)
	return hex.EncodeToString(key), salt, nil
}

// VerifyPassword verifies a plaintext password against a salt and stored hash.
func VerifyPassword(password, salt, storedHash string) bool {
	computedHash, _, err := HashPassword(password, salt)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(computedHash), []byte(storedHash)) == 1
}

// GenerateToken generates a cryptographically secure random URL-safe token.
func GenerateToken(byteLength int) (string, error) {
	b := make([]byte, byteLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
