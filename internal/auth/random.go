package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

func randString(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Errorf("crypto/rand unavailable: %w", err))
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func NewState() string { return randString(32) }

func NewVerifier() string { return randString(64) }
