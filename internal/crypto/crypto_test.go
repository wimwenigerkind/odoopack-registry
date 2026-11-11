package crypto

import (
	"bytes"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func testCipher(t *testing.T, fill byte) *Cipher {
	t.Helper()
	c, err := NewCipher(bytes.Repeat([]byte{fill}, KeySize))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

var aad = []byte("7d0c2a4e-acme:access_token")

func TestRoundTrip(t *testing.T) {
	c := testCipher(t, 1)
	for _, plain := range []string{"", "gho_acme", strings.Repeat("x", 4096)} {
		sealed, err := c.Seal([]byte(plain), aad)
		if err != nil {
			t.Fatal(err)
		}
		if sealed[0] != Version {
			t.Errorf("version byte %#x, want %#x", sealed[0], Version)
		}
		got, err := c.Open(sealed, aad)
		if err != nil {
			t.Fatalf("open %q: %v", plain, err)
		}
		if string(got) != plain {
			t.Errorf("got %q, want %q", got, plain)
		}
	}
}

func TestSealUsesFreshNonce(t *testing.T) {
	c := testCipher(t, 1)
	a, _ := c.Seal([]byte("gho_acme"), aad)
	b, _ := c.Seal([]byte("gho_acme"), aad)
	if bytes.Equal(a, b) {
		t.Error("sealing the same plaintext twice gave the same ciphertext")
	}
}

func TestOpenRejects(t *testing.T) {
	c := testCipher(t, 1)
	sealed, err := c.Seal([]byte("gho_acme"), aad)
	if err != nil {
		t.Fatal(err)
	}

	tampered := bytes.Clone(sealed)
	tampered[len(tampered)-1] ^= 0xff
	wrongVersion := bytes.Clone(sealed)
	wrongVersion[0] = 0x02

	cases := []struct {
		name   string
		cipher *Cipher
		sealed []byte
		aad    []byte
		want   error
	}{
		{"wrong key", testCipher(t, 2), sealed, aad, nil},
		{"tampered ciphertext", c, tampered, aad, nil},
		{"other id", c, sealed, []byte("9f1b3c5d-acme:access_token"), nil},
		{"other field", c, sealed, []byte("7d0c2a4e-acme:refresh_token"), nil},
		{"no aad", c, sealed, nil, nil},
		{"wrong version", c, wrongVersion, aad, ErrUnknownVersion},
		{"empty", c, nil, aad, ErrCiphertextShort},
		{"short", c, sealed[:1+nonceSize+tagSize-1], aad, ErrCiphertextShort},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.cipher.Open(tc.sealed, tc.aad)
			if err == nil {
				t.Fatalf("opened to %q", got)
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Errorf("err %v, want %v", err, tc.want)
			}
		})
	}
}

func TestStringRoundTrip(t *testing.T) {
	c := testCipher(t, 1)
	encoded, err := c.SealString("gho_acme", aad)
	if err != nil {
		t.Fatal(err)
	}
	if !LooksSealed(encoded) {
		t.Errorf("%q does not look sealed", encoded)
	}
	got, err := c.OpenString(encoded, aad)
	if err != nil || got != "gho_acme" {
		t.Errorf("got %q, %v", got, err)
	}
	if _, err := c.OpenString("not base64!", aad); err == nil {
		t.Error("opened invalid base64")
	}
}

func TestLooksSealed(t *testing.T) {
	envelope := func(version byte, n int) string {
		b := make([]byte, n)
		b[0] = version
		return base64.StdEncoding.EncodeToString(b)
	}
	cases := []struct {
		value string
		want  bool
	}{
		{envelope(Version, 1+nonceSize+tagSize), true},
		{envelope(Version, 1+nonceSize+tagSize+40), true},
		{envelope(Version, 1+nonceSize+tagSize-1), false},
		{envelope(0x02, 1+nonceSize+tagSize), false},
		{"", false},
		{"gho_16C7e42F292c6912E7710c838347Ae178B4a", false},
		{"8a1f0c2e9b7d4a6f3e5c1b0a9d8e7f6a5b4c3d2e", false},
		{"Zm9vYmFyYmF6cXV4", false},
	}
	for _, tc := range cases {
		if got := LooksSealed(tc.value); got != tc.want {
			t.Errorf("LooksSealed(%q) = %v, want %v", tc.value, got, tc.want)
		}
	}
}

func TestParseKey(t *testing.T) {
	cases := []struct {
		name    string
		encoded string
		wantErr error
		anyErr  bool
	}{
		{"32 bytes", base64.StdEncoding.EncodeToString(make([]byte, 32)), nil, false},
		{"16 bytes", base64.StdEncoding.EncodeToString(make([]byte, 16)), ErrInvalidKeySize, true},
		{"33 bytes", base64.StdEncoding.EncodeToString(make([]byte, 33)), ErrInvalidKeySize, true},
		{"empty", "", ErrInvalidKeySize, true},
		{"bad base64", "not base64!", nil, true},
		{"url alphabet", strings.Repeat("_", 43) + "=", nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key, err := ParseKey(tc.encoded)
			if !tc.anyErr {
				if err != nil || len(key) != KeySize {
					t.Fatalf("key of %d bytes, %v", len(key), err)
				}
				return
			}
			if err == nil {
				t.Fatal("accepted")
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Errorf("err %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestNewCipherKeySize(t *testing.T) {
	if _, err := NewCipher(make([]byte, 16)); !errors.Is(err, ErrInvalidKeySize) {
		t.Errorf("err %v, want %v", err, ErrInvalidKeySize)
	}
}
