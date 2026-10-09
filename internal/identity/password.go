// SPDX-License-Identifier: Apache-2.0

// Package identity implements accounts, credentials and sessions per
// docs/plans/v1/02 §1-§3: free intranet registration (email+password, no
// verification), Argon2id hashing with progressive rehash, opaque server-side
// sessions, and operator-audited recovery. Login errors never distinguish
// unknown account / wrong password / disabled account.
package identity

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2 parameters per docs/plans/v1/02 §1 engineering defaults; must be
// load-tested before changing and never silently weakened.
const (
	argonMemoryKib  = 64 * 1024 // 64 MiB
	argonIterations = 3
	argonParallel   = 1
	argonSaltLen    = 16
	argonKeyLen     = 32
)

// HashPassword derives the PHC-style encoded Argon2id string:
// $argon2id$v=19$m=65536,t=3,p=1$<salt-b64>$<hash-b64>
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argonIterations, argonMemoryKib, argonParallel, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemoryKib, argonIterations, argonParallel,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

// VerifyPassword checks a password against the encoded hash in constant time
// and reports whether the stored parameters differ from current defaults
// (caller then rehashes on successful login).
func VerifyPassword(encoded, password string) (ok bool, needsRehash bool) {
	params, salt, want, err := decodeArgon(encoded)
	if err != nil {
		return false, false
	}
	got := argon2.IDKey([]byte(password), salt, params.iterations, params.memory, params.parallelism, uint32(len(want)))
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return false, false
	}
	return true, params.memory != argonMemoryKib || params.iterations != argonIterations || params.parallelism != argonParallel
}

type argonParams struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
}

func decodeArgon(encoded string) (argonParams, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	// ["", "argon2id", "v=19", "m=...,t=...,p=...", salt, hash]
	if len(parts) != 6 || parts[1] != "argon2id" {
		return argonParams{}, nil, nil, fmt.Errorf("identity: unsupported hash format")
	}
	var memory, iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return argonParams{}, nil, nil, fmt.Errorf("identity: bad argon2 params: %w", err)
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return argonParams{}, nil, nil, err
	}
	hash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return argonParams{}, nil, nil, err
	}
	return argonParams{memory: memory, iterations: iterations, parallelism: parallelism}, salt, hash, nil
}

// NormalizeEmail applies the first-version rule: trim + case-fold, keeping
// dots and plus signs (docs/plans/v1/02 §1). Uniqueness lives in the DB
// unique index, never in a check-then-insert race.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
