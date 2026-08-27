package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/wimwenigerkind/odoopack-registry/internal/auth"
	"github.com/wimwenigerkind/odoopack-registry/internal/bootstrap"
	"github.com/wimwenigerkind/odoopack-registry/internal/config"
	"github.com/wimwenigerkind/odoopack-registry/internal/database"
	"github.com/wimwenigerkind/odoopack-registry/internal/handler"
	"github.com/wimwenigerkind/odoopack-registry/internal/logger"
	"github.com/wimwenigerkind/odoopack-registry/internal/middleware"
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
	if viper.GetBool("auto_migrate") {
		if err := database.Migrate(db); err != nil {
			bootstrap.Fatal("database migrate", err)
		}
	}

	store, err := bootstrap.Storage()
	if err != nil {
		bootstrap.Fatal("storage", err)
	}

	addonRepo := repository.NewAddonRepository(db)
	repoRepo := repository.NewRepoRepository(db)
	versionRepo := repository.NewAddonVersionRepository(db)
	userRepo := repository.NewUserRepository(db)
	groupRepo := repository.NewGroupRepository(db)
	tokenRepo := repository.NewApiTokenRepository(db)
	integrationRepo := repository.NewIntegrationRepository(db)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	authProviders, err := auth.LoadProviders(ctx, viper.GetString("base_url"))
	if err != nil {
		bootstrap.Fatal("auth providers", err)
	}
	authRegistry := auth.NewRegistry(authProviders)

	queue := worker.NewQueue(db)

	stateStore, err := bootstrap.StateStore()
	if err != nil {
		bootstrap.Fatal("state store", err)
	}
	defer stateStore.Stop()
	sessionStore, err := bootstrap.SessionStore()
	if err != nil {
		bootstrap.Fatal("session store", err)
	}
	defer sessionStore.Stop()

	mode := viper.GetString("instance.mode")
	baseURL := strings.TrimSpace(viper.GetString("base_url"))
	if baseURL == "" {
		bootstrap.Fatal("config", fmt.Errorf("base_url must be set"))
	}
	addonHandler := handler.NewAddonHandler(addonRepo, repoRepo, versionRepo, groupRepo, userRepo, integrationRepo, store, mode)
	repoHandler := handler.NewRepoHandler(repoRepo, addonRepo, groupRepo, userRepo, integrationRepo, mode)
	downloadHandler := handler.NewDownloadHandler(addonRepo, versionRepo, groupRepo, userRepo, store, mode)
	triggerHandler := handler.NewTriggerHandler(addonRepo, userRepo, queue)
	registryHandler := handler.NewRegistryHandler(addonRepo, groupRepo, userRepo, integrationRepo, authRegistry, store, mode, baseURL)
	authHandler := handler.NewAuthHandler(authRegistry, stateStore, sessionStore, userRepo, integrationRepo, viper.GetBool("auth.cookie_secure"))
	groupHandler := handler.NewGroupHandler(groupRepo)
	userHandler := handler.NewUserHandler(userRepo)
	tokenHandler := handler.NewTokenHandler(tokenRepo)
	integrationHandler := handler.NewIntegrationHandler(authRegistry, stateStore, integrationRepo, viper.GetBool("auth.cookie_secure"), baseURL)
	webhookHandler := handler.NewWebhookHandler(integrationRepo, addonRepo, queue)

	r := gin.Default()

	if origins := viper.GetStringSlice("cors.allowed_origins"); len(origins) > 0 {
		r.Use(cors.New(cors.Config{
			AllowOrigins:     origins,
			AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowHeaders:     []string{"Authorization", "Content-Type"},
			AllowCredentials: true,
			MaxAge:           12 * time.Hour,
		}))
	}

	if err := r.SetTrustedProxies(nil); err != nil {
		bootstrap.Fatal("trusted proxies", err)
	}
	requireAuth := middleware.RequireAuth(sessionStore)
	requireAdmin := middleware.RequireAdmin(sessionStore, userRepo)
	optionalAuth := middleware.OptionalAuth(sessionStore)
	apiKeyOptional := middleware.ApiKeyOptional(tokenRepo)
	registerRoutes(r, mode, addonHandler, repoHandler, downloadHandler, triggerHandler, registryHandler, authHandler, groupHandler, userHandler, tokenHandler, integrationHandler, webhookHandler, requireAuth, requireAdmin, optionalAuth, apiKeyOptional)

	healthChecks := []handler.HealthCheck{
		{Name: "database", Fn: func(ctx context.Context) error {
			sqlDB, err := db.DB()
			if err != nil {
				return err
			}
			return sqlDB.PingContext(ctx)
		}},
	}
	if pinger, ok := sessionStore.(interface{ Ping(context.Context) error }); ok {
		healthChecks = append(healthChecks, handler.HealthCheck{Name: "redis", Fn: pinger.Ping})
	}
	healthHandler := handler.NewHealthHandler(healthChecks...)
	r.GET("/healthz", healthHandler.Live)
	r.GET("/readyz", healthHandler.Ready)

	addr := viper.GetString("server_address")
	srv := &http.Server{Addr: addr, Handler: r}

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("starting web server", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			bootstrap.Fatal("server", err)
		}
	}()

	<-sigCtx.Done()
	stop()
	slog.Info("web shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
	}
}

func registerRoutes(r *gin.Engine, mode string, addons *handler.AddonHandler, repos *handler.RepoHandler, downloads *handler.DownloadHandler, triggers *handler.TriggerHandler, registry *handler.RegistryHandler, authH *handler.AuthHandler, groups *handler.GroupHandler, users *handler.UserHandler, tokens *handler.TokenHandler, integrations *handler.IntegrationHandler, webhooks *handler.WebhookHandler, requireAuth, requireAdmin, optionalAuth, apiKeyOptional gin.HandlerFunc) {
	api := r.Group("/api/v1")
	{
		api.GET("/me", requireAuth, authH.Me)
		api.GET("/addons", optionalAuth, addons.List)
		api.POST("/addons", requireAuth, addons.Register)
		api.GET("/addons/:id", optionalAuth, addons.Get)
		api.PUT("/addons/:id", requireAuth, addons.Update)
		api.DELETE("/addons/:id", requireAuth, addons.Delete)
		api.GET("/addons/:id/versions/:version/download", optionalAuth, downloads.Zipball)
		api.GET("/addons/:id/versions/:version/readme", optionalAuth, addons.Readme)
		api.DELETE("/addons/:id/versions/:version", requireAuth, addons.DeleteVersion)
		api.POST("/addons/:id/sync", requireAuth, triggers.Sync)

		api.GET("/repos/:id", optionalAuth, repos.Get)
		api.PUT("/repos/:id", requireAuth, repos.Update)
		api.DELETE("/repos/:id", requireAuth, repos.Delete)
		api.GET("/me/repos", requireAuth, repos.ListMine)

		api.POST("/me/tokens", requireAuth, tokens.Create)
		api.GET("/me/tokens", requireAuth, tokens.List)
		api.DELETE("/me/tokens/:id", requireAuth, tokens.Delete)

		api.GET("/me/integrations", requireAuth, integrations.List)
		api.DELETE("/me/integrations/:id", requireAuth, integrations.Delete)
		api.POST("/me/integrations/:id/workspace-hooks", requireAuth, integrations.CreateWorkspaceHook)
	}

	if mode == "private" {
		api.GET("/users", requireAdmin, users.List)

		api.POST("/groups", requireAdmin, groups.Create)
		api.GET("/groups", requireAdmin, groups.List)
		api.GET("/groups/:id", requireAdmin, groups.Get)
		api.DELETE("/groups/:id", requireAdmin, groups.Delete)
		api.POST("/groups/:id/members", requireAdmin, groups.AddMember)
		api.DELETE("/groups/:id/members/:user_id", requireAdmin, groups.RemoveMember)
		api.GET("/groups/:id/members", requireAdmin, groups.ListMembers)
		api.POST("/groups/:id/addons", requireAdmin, groups.GrantAddon)
		api.DELETE("/groups/:id/addons/:addon_id", requireAdmin, groups.RevokeAddon)
		api.GET("/groups/:id/addons", requireAdmin, groups.ListAddons)
	}

	reg := r.Group("/registry/v1")
	{
		reg.GET("/addons/*name", apiKeyOptional, registry.Get)
		reg.GET("/zipball/:addon_id/:reference", apiKeyOptional, registry.Zipball)
	}

	r.GET("/auth/providers", authH.ListProviders)
	a := r.Group("/auth/:provider")
	{
		a.GET("/login", authH.Login)
		a.GET("/link", requireAuth, authH.Link)
		a.GET("/callback", authH.Callback)
	}
	r.POST("/auth/logout", requireAuth, authH.Logout)
	r.DELETE("/api/v1/me/identities/:id", requireAuth, authH.UnlinkIdentity)

	r.GET("/integrations/providers", requireAuth, integrations.ListProviders)
	r.GET("/integrations/:provider/connect", requireAuth, integrations.Connect)

	r.POST("/webhooks/bitbucket/:integration_id", webhooks.Bitbucket)
}
