package database

import (
	"fmt"
	"log/slog"

	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Connect(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return db, nil
}

func Migrate(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database connection not established")
	}

	err := db.AutoMigrate(
		&models.User{},
		&models.Identity{},
		&models.OAuthIntegration{},
		&models.Repo{},
		&models.Addon{},
		&models.AddonVersion{},
		&models.AddonVersionReadme{},
		&models.Group{},
		&models.GroupMembership{},
		&models.GroupAddonAccess{},
		&models.ApiToken{},
	)
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	slog.Info("database migrations completed")
	return nil
}
