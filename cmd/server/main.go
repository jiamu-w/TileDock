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

	panelassets "panel"
	"panel/internal/config"
	"panel/internal/handler"
	"panel/internal/middleware"
	"panel/internal/repository"
	"panel/internal/router"
	"panel/internal/service"
	"panel/internal/view"
	"panel/pkg/db"
	"panel/pkg/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		_, _ = os.Stderr.WriteString("load config failed: " + err.Error() + "\n")
		os.Exit(1)
	}

	log := logger.New(cfg.App.Env, cfg.Log.Level)
	slog.SetDefault(log)

	database, err := db.New(cfg.Database.Path)
	if err != nil {
		log.Error("open database failed", "error", err)
		os.Exit(1)
	}

	if err := db.AutoMigrate(database); err != nil {
		log.Error("auto migrate failed", "error", err)
		os.Exit(1)
	}

	if err := db.SeedAdmin(database, cfg.Auth.DefaultAdminUser, cfg.Auth.DefaultAdminPassword); err != nil {
		log.Error("seed admin failed", "error", err)
		os.Exit(1)
	}

	renderer, err := view.NewFromFS(panelassets.Files, "templates/*/*.html", "templates/*.html")
	if err != nil {
		log.Error("load templates failed", "error", err)
		os.Exit(1)
	}

	userRepo := repository.NewUserRepository(database)
	groupRepo := repository.NewNavGroupRepository(database)
	linkRepo := repository.NewNavLinkRepository(database)
	settingRepo := repository.NewSettingRepository(database)

	authService := service.NewAuthService(userRepo)
	dashboardService := service.NewDashboardService(groupRepo, linkRepo, settingRepo)
	navService := service.NewNavigationService(groupRepo, linkRepo)
	bookmarkImportService := service.NewBookmarkImportService(navService, cfg.Storage.UploadDir)
	settingService := service.NewSettingService(settingRepo)
	weatherService := service.NewWeatherService(settingRepo)
	backupService := service.NewBackupService(
		database,
		groupRepo,
		settingRepo,
		cfg.Database.Path,
		cfg.Storage.BackupDir,
		cfg.Storage.UploadDir,
	)
	loginRateLimiter := middleware.NewLoginRateLimiter()

	authHandler := handler.NewAuthHandler(renderer, authService, loginRateLimiter, log)
	dashboardHandler := handler.NewDashboardHandler(renderer, dashboardService, authService)
	navHandler := handler.NewNavigationHandler(renderer, navService, log, cfg.Storage.UploadDir)
	settingHandler := handler.NewSettingHandler(renderer, settingService, authService, backupService, bookmarkImportService, cfg.Storage.UploadDir, log)
	systemHandler := handler.NewSystemHandler(log, weatherService)

	engine := router.New(cfg, log, renderer, authHandler, dashboardHandler, navHandler, settingHandler, systemHandler)

	server := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           engine,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Info("server started", "addr", cfg.Server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Error("server shutdown failed", "error", err)
		os.Exit(1)
	}

	log.Info("server exited")
}
