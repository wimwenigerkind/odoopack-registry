package database

import (
	"fmt"
	"log"

	"github.com/spf13/viper"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Connect() (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s",
		viper.GetString("db.host"),
		viper.GetString("db.user"),
		viper.GetString("db.password"),
		viper.GetString("db.name"),
		viper.GetString("db.port"),
	)

	db, err := gorm.Open(postgres.Open(dsn))
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
		&models.Addon{},
		&models.AddonVersion{},
		&models.User{},
		&models.Identity{},
		&models.Group{},
		&models.GroupMembership{},
		&models.GroupAddonAccess{},
		&models.ApiToken{},
	)
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Println("database migrations completed successfully")
	return nil
}
