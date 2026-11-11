package database

import (
	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/apitoken"
	"gorm.io/gorm"
)

// hashLegacyApiTokens replaces the plain token column of api_tokens with its
// sha256, so tokens issued before hashing keep working. It runs before
// AutoMigrate, which can neither add a NOT NULL column to a table with rows nor
// drop a column. A database without the plain column (fresh, or already
// migrated) is left alone, and Postgres runs the DDL inside the transaction, so
// a failed run leaves the table as it was.
func hashLegacyApiTokens(db *gorm.DB) error {
	const table = "api_tokens"
	return db.Transaction(func(tx *gorm.DB) error {
		m := tx.Migrator()
		if !m.HasTable(table) || !m.HasColumn(table, "token") {
			return nil
		}
		if err := tx.Exec(`ALTER TABLE api_tokens
			ADD COLUMN IF NOT EXISTS token_hash text,
			ADD COLUMN IF NOT EXISTS prefix text,
			ADD COLUMN IF NOT EXISTS last_four text`).Error; err != nil {
			return err
		}

		var rows []struct {
			ID    uuid.UUID
			Token string
		}
		if err := tx.Table(table).Select("id", "token").Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			if err := tx.Table(table).Where("id = ?", row.ID).Updates(legacyTokenColumns(row.Token)).Error; err != nil {
				return err
			}
		}

		if err := tx.Exec(`ALTER TABLE api_tokens
			ALTER COLUMN token_hash SET NOT NULL,
			ALTER COLUMN prefix SET NOT NULL,
			ALTER COLUMN last_four SET NOT NULL`).Error; err != nil {
			return err
		}
		return m.DropColumn(table, "token")
	})
}

func legacyTokenColumns(plain string) map[string]any {
	prefix, lastFour := apitoken.DisplayParts(plain)
	return map[string]any{
		"token_hash": apitoken.Hash(plain),
		"prefix":     prefix,
		"last_four":  lastFour,
	}
}
