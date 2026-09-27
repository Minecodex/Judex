// SPDX-License-Identifier: Apache-2.0

// Package keys centralizes hashing and comparison helpers for opaque
// secrets (session cookies, tokens, recovery codes): store only hashes,
// compare in constant time, never log the plaintext.
package keys

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// NewRandom returns 32 bytes of CSPRNG output, urlsafe-base64 encoded.
func NewRandom() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("keys: entropy source failed: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// Hash derives the 32-byte SHA-256 digest of a secret as hex text for
// storage columns. Plain SHA-256 is only used for high-entropy random
// secrets (never passwords — those use Argon2id in internal/identity).
func Hash(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return []byte(hex.EncodeToString(sum[:]))
}

// Equal compares two stored hashes in constant time.
func Equal(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := range a {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}
