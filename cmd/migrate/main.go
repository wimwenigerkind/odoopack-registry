package main

import (
	"github.com/spf13/viper"
	"github.com/wimwenigerkind/odoopack-registry/internal/bootstrap"
	"github.com/wimwenigerkind/odoopack-registry/internal/config"
	"github.com/wimwenigerkind/odoopack-registry/internal/database"
	"github.com/wimwenigerkind/odoopack-registry/internal/logger"
)

func main() {
	config.LoadConfig()
	logger.Setup(viper.GetString("log.level"))

	db, err := database.Connect(viper.GetString("database.dsn"))
	if err != nil {
		bootstrap.Fatal("database connect", err)
	}
	if err := database.Migrate(db); err != nil {
		bootstrap.Fatal("database migrate", err)
	}
}
