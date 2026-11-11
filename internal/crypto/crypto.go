// Package crypto seals integration credentials with AES-GCM. The key never
// leaves the process and is read from the configuration, so it is neither in
// the source nor in the database.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

// KeySize is the key length in bytes. 32 bytes select AES-256.
const KeySize = 32

// Version leads every sealed value, so a later key rotation or change of
// scheme can tell the values it has to rewrite from the ones it already did.
const Version byte = 0x01

// The standard GCM sizes that cipher.NewGCM uses.
const (
	nonceSize = 12
	tagSize   = 16
)

var (
	ErrInvalidKeySize  = errors.New("encryption key must be 32 bytes")
	ErrCiphertextShort = errors.New("ciphertext is too short")
	ErrUnknownVersion  = errors.New("ciphertext has an unknown version")
)

type Cipher struct {
	aead cipher.AEAD
}

// ParseKey decodes a base64 encoded key, which is how it is passed in through
// the configuration.
func ParseKey(encoded string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode encryption key: %w", err)
	}
	if len(key) != KeySize {
		return nil, ErrInvalidKeySize
	}
	return key, nil
}

func NewCipher(key []byte) (*Cipher, error) {
	if len(key) != KeySize {
		return nil, ErrInvalidKeySize
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}

	return &Cipher{aead: aead}, nil
}

// Seal encrypts plaintext and returns the version byte, the nonce and the
// ciphertext. A fresh nonce per call is what keeps two identical secrets from
// producing identical ciphertext, so reusing one would leak that they are
// equal. aad is authenticated but not stored: Open needs the same aad, so a
// value copied to another place that passes a different aad does not open.
func (c *Cipher) Seal(plaintext, aad []byte) ([]byte, error) {
	out := make([]byte, 1+nonceSize, 1+nonceSize+len(plaintext)+tagSize)
	out[0] = Version
	nonce := out[1:]
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	return c.aead.Seal(out, nonce, plaintext, aad), nil
}

// Open reverses Seal. It fails if the ciphertext or aad was tampered with,
// because GCM authenticates as well as encrypts.
func (c *Cipher) Open(sealed, aad []byte) ([]byte, error) {
	if len(sealed) < 1+nonceSize+tagSize {
		return nil, ErrCiphertextShort
	}
	if sealed[0] != Version {
		return nil, ErrUnknownVersion
	}

	nonce := sealed[1 : 1+nonceSize]
	plaintext, err := c.aead.Open(nil, nonce, sealed[1+nonceSize:], aad)
	if err != nil {
		return nil, fmt.Errorf("open ciphertext: %w", err)
	}

	return plaintext, nil
}

// SealString seals plaintext and encodes the result as base64, for storing in
// a text column.
func (c *Cipher) SealString(plaintext string, aad []byte) (string, error) {
	sealed, err := c.Seal([]byte(plaintext), aad)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sealed), nil
}

func (c *Cipher) OpenString(encoded string, aad []byte) (string, error) {
	sealed, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}
	plaintext, err := c.Open(sealed, aad)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// LooksSealed reports whether encoded has the shape SealString produces:
// valid base64 of the version byte, a nonce and at least a GCM tag. It does
// not prove the value opens; it tells a sealed value that fails to open from
// one that was never sealed.
func LooksSealed(encoded string) bool {
	sealed, err := base64.StdEncoding.DecodeString(encoded)
	return err == nil && len(sealed) >= 1+nonceSize+tagSize && sealed[0] == Version
}
