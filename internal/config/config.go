package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

func LoadConfig() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	viper.SetDefault("server_address", "0.0.0.0:6969")
	viper.SetDefault("base_url", "http://localhost:6969")

	viper.SetDefault("db.host", "localhost")
	viper.SetDefault("db.port", "5432")
	viper.SetDefault("db.user", "postgres")
	viper.SetDefault("db.password", "postgres")
	viper.SetDefault("db.name", "odoopack-registry")

	viper.SetDefault("storage.driver", "local")
	viper.SetDefault("storage.local.root", "./data/storage")
	viper.SetDefault("storage.local.public_url", "http://localhost:6969/zipball")

	viper.SetDefault("worker.count", 2)
	viper.SetDefault("worker.queue_size", 64)

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		fmt.Println("no config file found, using defaults")
	}
}
