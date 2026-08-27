package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/viper"
	"github.com/wimwenigerkind/odoopack-registry/internal/auth"
	"github.com/wimwenigerkind/odoopack-registry/internal/bootstrap"
	"github.com/wimwenigerkind/odoopack-registry/internal/config"
	"github.com/wimwenigerkind/odoopack-registry/internal/database"
	"github.com/wimwenigerkind/odoopack-registry/internal/logger"
	"github.com/wimwenigerkind/odoopack-registry/internal/repository"
	"github.com/wimwenigerkind/odoopack-registry/internal/worker"
	"gorm.io/gorm"
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

	if addr := viper.GetString("worker.health_address"); addr != "" {
		startHealthServer(ctx, addr, db)
	}

	consumer := worker.NewConsumer(db, versionRepo, integrationRepo, authRegistry, store, viper.GetDuration("worker.poll_interval"))
	workers := viper.GetInt("worker.count")
	slog.Info("starting worker", "workers", workers)
	consumer.Run(ctx, workers)

	<-ctx.Done()
	slog.Info("worker shutting down")
	consumer.Wait()
}

func startHealthServer(ctx context.Context, addr string, db *gorm.DB) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		pingCtx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		err := errors.New("database connection not established")
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			err = sqlDB.PingContext(pingCtx)
		}
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"unavailable"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	srv := &http.Server{Addr: addr, Handler: mux}
	go func() {
		slog.Info("starting worker health server", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("worker health server failed", "error", err)
		}
	}()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
}
