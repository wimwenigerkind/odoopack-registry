package bootstrap

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/viper"
	"github.com/wimwenigerkind/odoopack-registry/internal/auth"
	"github.com/wimwenigerkind/odoopack-registry/internal/storage"
)

func Fatal(msg string, err error) {
	slog.Error(msg, "err", err)
	os.Exit(1)
}

func Storage() (storage.Storage, error) {
	switch viper.GetString("storage.driver") {
	case "local":
		return storage.NewLocalStorage(viper.GetString("storage.local.root"))
	case "s3":
		return storage.NewS3Storage(storage.S3Config{
			Endpoint:        viper.GetString("storage.s3.endpoint"),
			Region:          viper.GetString("storage.s3.region"),
			Bucket:          viper.GetString("storage.s3.bucket"),
			AccessKeyID:     viper.GetString("storage.s3.access_key_id"),
			SecretAccessKey: viper.GetString("storage.s3.secret_access_key"),
			UseSSL:          viper.GetBool("storage.s3.use_ssl"),
			UsePathStyle:    viper.GetBool("storage.s3.use_path_style"),
			Prefix:          viper.GetString("storage.s3.prefix"),
			PresignTTL:      viper.GetDuration("storage.s3.presign_ttl"),
		})
	default:
		return nil, fmt.Errorf("unsupported storage driver %q", viper.GetString("storage.driver"))
	}
}

func SessionStore() (auth.SessionStore, error) {
	switch viper.GetString("session.store") {
	case "", "memory":
		return auth.NewMemorySessionStore(), nil
	case "redis":
		return auth.NewRedisSessionStore(viper.GetString("redis.url"))
	default:
		return nil, fmt.Errorf("unsupported session store %q", viper.GetString("session.store"))
	}
}

func StateStore() (auth.StateStore, error) {
	switch viper.GetString("session.store") {
	case "", "memory":
		return auth.NewMemoryStateStore(), nil
	case "redis":
		return auth.NewRedisStateStore(viper.GetString("redis.url"))
	default:
		return nil, fmt.Errorf("unsupported session store %q", viper.GetString("session.store"))
	}
}
