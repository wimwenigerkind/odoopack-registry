package database

import (
	"bytes"
	"testing"

	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/crypto"
	"github.com/wimwenigerkind/odoopack-registry/internal/repository"
)

func testCipher(t *testing.T, fill byte) *crypto.Cipher {
	t.Helper()
	c, err := crypto.NewCipher(bytes.Repeat([]byte{fill}, crypto.KeySize))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestClassifyToken(t *testing.T) {
	c := testCipher(t, 1)
	id := uuid.New()
	aad := repository.IntegrationTokenAAD(id, "access_token")
	seal := func(c *crypto.Cipher, aad []byte) string {
		s, err := c.SealString("gho_acme", aad)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}

	cases := []struct {
		name   string
		stored string
		want   tokenState
	}{
		{"empty", "", tokenSealed},
		{"github token", "gho_16C7e42F292c6912E7710c838347Ae178B4a", tokenPlain},
		{"forgejo token", "8a1f0c2e9b7d4a6f3e5c1b0a9d8e7f6a5b4c3d2e", tokenPlain},
		{"bitbucket token", "Xk2bJ9q-LmN4pQrS_tUvWxYz0123456789abcdefGH==", tokenPlain},
		{"plain base64", "Zm9vYmFyYmF6cXV4", tokenPlain},
		{"sealed", seal(c, aad), tokenSealed},
		{"sealed with another key", seal(testCipher(t, 2), aad), tokenUnreadable},
		{"sealed for another row", seal(c, repository.IntegrationTokenAAD(uuid.New(), "access_token")), tokenUnreadable},
		{"sealed for the other column", seal(c, repository.IntegrationTokenAAD(id, "refresh_token")), tokenUnreadable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyToken(c, tc.stored, aad); got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
}
