package config

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/viper"
)

func LoadConfig() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	viper.SetDefault("server_address", "0.0.0.0:6969")
	viper.SetDefault("base_url", "http://localhost:6969")
	viper.SetDefault("instance.mode", "public")

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
	// dev_mode allows what only a single local process gets away with, such
	// as the memory session store.
	viper.SetDefault("dev_mode", false)
	viper.SetDefault("redis.url", "redis://localhost:6379/0")

	// Deployments run the migrate binary before the web pods start;
	// auto_migrate is for local development.
	viper.SetDefault("auto_migrate", false)

	viper.SetDefault("worker.count", 2)
	viper.SetDefault("worker.poll_interval", "2s")
	viper.SetDefault("worker.health_address", "")

	viper.SetDefault("log.level", "info")

	viper.SetDefault("integrations.encryption_key", "")

	setLimitDefaults(viper.GetViper())

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		fmt.Println("no config file found, using defaults")
	}
}

// ValidateInstanceMode accepts exactly "public" and "private". Anything else,
// a typo or "Private" included, used to make the instance silently public.
func ValidateInstanceMode(mode string) error {
	switch mode {
	case "public", "private":
		return nil
	}
	return fmt.Errorf("instance.mode must be %q or %q, got %q", "public", "private", mode)
}

// CookieSecure decides the Secure flag of the session and state cookies.
// Unset, it follows the scheme of baseURL, so an https deployment cannot ship
// cookies a plain http hop would leak. insecure reports an explicit false on an
// https baseURL, which the caller should warn about.
func CookieSecure(baseURL string, explicit *bool) (secure, insecure bool) {
	https := false
	if u, err := url.Parse(baseURL); err == nil {
		https = u.Scheme == "https"
	}
	if explicit == nil {
		return https, false
	}
	return *explicit, https && !*explicit
}
