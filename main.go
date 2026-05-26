package main

import (
	"context"
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/wimwenigerkind/odoopack-registry/internal/config"
	"github.com/wimwenigerkind/odoopack-registry/internal/database"
	"github.com/wimwenigerkind/odoopack-registry/internal/handler"
	"github.com/wimwenigerkind/odoopack-registry/internal/repository"
	"github.com/wimwenigerkind/odoopack-registry/internal/storage"
	"github.com/wimwenigerkind/odoopack-registry/internal/worker"
)

func main() {
	config.LoadConfig()

	db, err := database.Connect()
	if err != nil {
		log.Fatalf("database connect: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		log.Fatalf("database migrate: %v", err)
	}

	store, err := buildStorage()
	if err != nil {
		log.Fatalf("storage: %v", err)
	}

	addonRepo := repository.NewAddonRepository(db)
	versionRepo := repository.NewAddonVersionRepository(db)

	queue := worker.NewQueue(versionRepo, store, viper.GetInt("worker.queue_size"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	queue.Start(ctx, viper.GetInt("worker.count"))

	addonHandler := handler.NewAddonHandler(addonRepo)
	downloadHandler := handler.NewDownloadHandler(addonRepo, versionRepo, store)
	triggerHandler := handler.NewTriggerHandler(addonRepo, queue)
	registryHandler := handler.NewRegistryHandler(addonRepo, store)

	r := gin.Default()
	registerRoutes(r, addonHandler, downloadHandler, triggerHandler, registryHandler)

	if viper.GetString("storage.driver") == "local" {
		r.Static("/zipball", viper.GetString("storage.local.root"))
	}

	addr := viper.GetString("server_address")
	fmt.Printf("starting on: http://%s\n", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func buildStorage() (storage.Storage, error) {
	switch viper.GetString("storage.driver") {
	case "local":
		return storage.NewLocalStorage(
			viper.GetString("storage.local.root"),
			viper.GetString("storage.local.public_url"),
		)
	default:
		return nil, fmt.Errorf("unsupported storage driver %q", viper.GetString("storage.driver"))
	}
}

func registerRoutes(r *gin.Engine, addons *handler.AddonHandler, downloads *handler.DownloadHandler, triggers *handler.TriggerHandler, registry *handler.RegistryHandler) {
	api := r.Group("/api/v1")
	{
		api.GET("/addons", addons.List)
		api.POST("/addons", addons.Register)
		api.GET("/addons/:id", addons.Get)
		api.GET("/addons/:id/versions/:version/download", downloads.Zipball)
		api.POST("/addons/:id/sync", triggers.Sync)
	}

	reg := r.Group("/registry/v1")
	{
		reg.GET("/addons/*name", registry.Get)
	}
}
