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
	viper.SetDefault("instance.mode", "public") // "public" or "private"

	viper.SetDefault("cors.allowed_origins", []string{})

	viper.SetDefault("database.dsn", "host=postgres port=5432 user=odoopack password=odoopack dbname=odoopack sslmode=disable")

	viper.SetDefault("storage.driver", "local")
	viper.SetDefault("storage.local.root", "./data/storage")

	viper.SetDefault("storage.s3.endpoint", "")
	viper.SetDefault("storage.s3.region", "us-east-1")
	viper.SetDefault("storage.s3.bucket", "")
	viper.SetDefault("storage.s3.access_key_id", "")
	viper.SetDefault("storage.s3.secret_access_key", "")
	viper.SetDefault("storage.s3.use_ssl", true)
	viper.SetDefault("storage.s3.use_path_style", false)
	viper.SetDefault("storage.s3.prefix", "")
	viper.SetDefault("storage.s3.presign_ttl", "5m")

	viper.SetDefault("session.store", "memory") // "memory" or "redis"
	viper.SetDefault("redis.url", "redis://localhost:6379/0")

	viper.SetDefault("worker.count", 2)
	viper.SetDefault("worker.queue_size", 64)

	viper.SetDefault("log.level", "info")

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		fmt.Println("no config file found, using defaults")
	}
}
