package identity

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
)

// hashHMAC returns the hex HMAC-SHA256 of value under secret.
func hashHMAC(secret, value string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(value))
	return hex.EncodeToString(m.Sum(nil))
}

// hashEqual compares two hex hashes in constant time.
func hashEqual(a, b string) bool {
	return hmac.Equal([]byte(a), []byte(b))
}

// newCode returns a uniformly random 6-digit code from crypto/rand.
func newCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", fmt.Errorf("identity.newCode: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// newRefreshToken returns 32 random bytes, base64url encoded.
func newRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("identity.newRefreshToken: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// newFamilyID returns a random version-4 UUID string.
func newFamilyID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("identity.newFamilyID: %w", err)
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
