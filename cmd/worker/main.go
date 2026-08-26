package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/viper"
	"github.com/wimwenigerkind/odoopack-registry/internal/auth"
	"github.com/wimwenigerkind/odoopack-registry/internal/bootstrap"
	"github.com/wimwenigerkind/odoopack-registry/internal/config"
	"github.com/wimwenigerkind/odoopack-registry/internal/database"
	"github.com/wimwenigerkind/odoopack-registry/internal/logger"
	"github.com/wimwenigerkind/odoopack-registry/internal/repository"
	"github.com/wimwenigerkind/odoopack-registry/internal/worker"
)

func main() {
	config.LoadConfig()
	logger.Setup(viper.GetString("log.level"))

	db, err := database.Connect(viper.GetString("database.dsn"))
	if err != nil {
		bootstrap.Fatal("database connect", err)
	}

	store, err := bootstrap.Storage()
	if err != nil {
		bootstrap.Fatal("storage", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	authProviders, err := auth.LoadProviders(ctx, viper.GetString("base_url"))
	if err != nil {
		bootstrap.Fatal("auth providers", err)
	}
	authRegistry := auth.NewRegistry(authProviders)

	versionRepo := repository.NewAddonVersionRepository(db)
	integrationRepo := repository.NewIntegrationRepository(db)

	consumer := worker.NewConsumer(db, versionRepo, integrationRepo, authRegistry, store, viper.GetDuration("worker.poll_interval"))
	workers := viper.GetInt("worker.count")
	slog.Info("starting worker", "workers", workers)
	consumer.Run(ctx, workers)

	<-ctx.Done()
	slog.Info("worker shutting down")
	consumer.Wait()
}
