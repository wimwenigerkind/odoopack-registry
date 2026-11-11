package database

import (
	"log/slog"

	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/crypto"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"github.com/wimwenigerkind/odoopack-registry/internal/repository"
	"gorm.io/gorm"
)

type tokenState int

const (
	tokenPlain tokenState = iota
	tokenSealed
	tokenUnreadable
)

// sealIntegrationTokens encrypts the tokens of integrations written before
// tokens were sealed. It is idempotent: a value that opens with the key and
// its own aad is left alone, so a second run changes nothing.
func sealIntegrationTokens(db *gorm.DB, c *crypto.Cipher) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var rows []struct {
			ID           uuid.UUID
			AccessToken  string
			RefreshToken string
		}
		if err := tx.Model(&models.OAuthIntegration{}).Select("id", "access_token", "refresh_token").Find(&rows).Error; err != nil {
			return err
		}
		sealed := 0
		for _, row := range rows {
			cols := map[string]any{}
			for column, stored := range map[string]string{"access_token": row.AccessToken, "refresh_token": row.RefreshToken} {
				aad := repository.IntegrationTokenAAD(row.ID, column)
				switch classifyToken(c, stored, aad) {
				case tokenPlain:
					var err error
					if cols[column], err = c.SealString(stored, aad); err != nil {
						return err
					}
				case tokenUnreadable:
					slog.Warn("integration token is sealed with another key, the owner has to relink it", "integration", row.ID, "column", column)
				}
			}
			if len(cols) == 0 {
				continue
			}
			if err := tx.Model(&models.OAuthIntegration{}).Where("id = ?", row.ID).Updates(cols).Error; err != nil {
				return err
			}
			sealed++
		}
		if sealed > 0 {
			slog.Info("sealed integration tokens", "integrations", sealed)
		}
		return nil
	})
}

// classifyToken tells a plaintext token from a sealed one. A sealed value is
// standard base64 of the version byte, a nonce and a GCM tag, and opens with
// the key and its aad. Plaintext tokens do not have that shape: GitHub's carry
// an underscore, Forgejo's are lower case hex, which never decodes to a
// leading 0x01. A value with the shape that does not open was sealed with
// another key, a lost or a wrong one. It is left alone rather than sealed
// again: with the right key back it still opens, and otherwise relinking the
// integration overwrites it. An empty value counts as sealed, there is nothing
// to protect.
func classifyToken(c *crypto.Cipher, stored string, aad []byte) tokenState {
	switch {
	case stored == "":
		return tokenSealed
	case !crypto.LooksSealed(stored):
		return tokenPlain
	}
	if _, err := c.OpenString(stored, aad); err != nil {
		return tokenUnreadable
	}
	return tokenSealed
}
