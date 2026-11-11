// Package apitoken holds what is done with a plain API token: generating it,
// hashing it for storage and lookup, and the parts that may be shown again.
package apitoken

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// The CLI stores tokens in this format, so it must not change.
const tokenPrefix = "odp_"

// prefixLen covers "odp_" and four random characters, enough to tell tokens apart in a list.
const prefixLen = 8

const lastFourLen = 4

func Generate() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return tokenPrefix + hex.EncodeToString(b), nil
}

// Hash needs no salt or slow KDF: the token carries 256 random bits, so the hash
// cannot be brute forced, and a plain digest can be looked up by an index.
func Hash(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

func DisplayParts(plain string) (prefix, lastFour string) {
	prefix = plain[:min(prefixLen, len(plain))]
	lastFour = plain[len(plain)-min(lastFourLen, len(plain)):]
	return prefix, lastFour
}

func Expired(expiresAt *time.Time, now time.Time) bool {
	return expiresAt != nil && !now.Before(*expiresAt)
}
