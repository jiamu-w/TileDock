package router

import (
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	panelassets "panel"
	"panel/internal/config"
	"panel/internal/handler"
	"panel/internal/i18n"
	"panel/internal/middleware"
	"panel/internal/view"

	"github.com/gin-gonic/gin"
)

// New builds the gin engine.
func New(
	cfg *config.Config,
	log *slog.Logger,
	renderer *view.Renderer,
	authHandler *handler.AuthHandler,
	dashboardHandler *handler.DashboardHandler,
	navHandler *handler.NavigationHandler,
	settingHandler *handler.SettingHandler,
	systemHandler *handler.SystemHandler,
) *gin.Engine {
	if cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(requestLogger(log))
	engine.Use(middleware.SecurityHeaders())
	engine.Use(middleware.ErrorHandler(log))
	engine.Use(i18n.LanguageMiddleware())
	engine.HTMLRender = renderer
	registerStaticRoutes(engine, cfg, log)

	sessionStore := middleware.NewSessionStore(cfg.Session)
	engine.Use(func(c *gin.Context) {
		if userID, err := sessionStore.GetUserID(c); err == nil && userID != "" {
			c.Set("current_user_id", userID)
		}

		c.Next()

		if userID, ok := c.Get("login_user_id"); ok {
			_ = sessionStore.SetUserID(c, userID.(string))
		}
		if cleared, ok := c.Get("logout"); ok && cleared.(bool) {
			_ = sessionStore.Clear(c)
		}
	})
	engine.Use(middleware.CSRFMiddleware(sessionStore))

	engine.GET("/healthz", systemHandler.Healthz)
	engine.GET("/api/weather/current", systemHandler.CurrentWeather)
	engine.GET("/login", authHandler.ShowLogin)
	engine.POST("/login", authHandler.Login)
	engine.POST("/logout", func(c *gin.Context) {
		c.Set("logout", true)
		authHandler.Logout(c)
	})
	engine.GET("/", dashboardHandler.Index)

	authenticated := engine.Group("/")
	authenticated.Use(middleware.AuthRequired(sessionStore))
	authenticated.POST("/navigation/groups", navHandler.CreateGroup)
	authenticated.POST("/navigation/groups/:id", navHandler.UpdateGroup)
	authenticated.POST("/navigation/groups/:id/resize", navHandler.ResizeGroup)
	authenticated.DELETE("/navigation/groups/:id", navHandler.DeleteGroup)
	authenticated.POST("/navigation/links", navHandler.CreateLink)
	authenticated.POST("/navigation/links/:id", navHandler.UpdateLink)
	authenticated.DELETE("/navigation/links/:id", navHandler.DeleteLink)
	authenticated.POST("/navigation/reorder", navHandler.Reorder)
	authenticated.GET("/settings", settingHandler.Index)
	authenticated.POST("/settings", settingHandler.Save)
	authenticated.POST("/settings/profile", settingHandler.UpdateProfile)
	authenticated.POST("/settings/password", settingHandler.UpdatePassword)
	authenticated.POST("/settings/backup", settingHandler.DownloadBackup)
	authenticated.POST("/settings/restore", settingHandler.RestoreBackup)
	authenticated.POST("/settings/bookmarks/import", settingHandler.ImportBookmarks)

	return engine
}

func registerStaticRoutes(engine *gin.Engine, cfg *config.Config, log *slog.Logger) {
	embeddedStatic, err := fs.Sub(panelassets.Files, "static")
	if err != nil {
		panic(err)
	}
	embeddedServer := http.FileServer(http.FS(embeddedStatic))
	uploadsServer := http.FileServer(http.Dir(cfg.Storage.UploadDir))

	if err := os.MkdirAll(cfg.Storage.UploadDir, 0o755); err != nil {
		log.Error("create upload dir failed", "error", err, "dir", cfg.Storage.UploadDir)
	}

	engine.GET("/static/*filepath", func(c *gin.Context) {
		requestPath := c.Param("filepath")
		if strings.HasPrefix(requestPath, "/uploads/") {
			c.Request.URL.Path = strings.TrimPrefix(requestPath, "/uploads")
			uploadsServer.ServeHTTP(c.Writer, c.Request)
			return
		}

		c.Request.URL.Path = filepath.ToSlash(strings.TrimPrefix(requestPath, "/"))
		if !strings.HasPrefix(c.Request.URL.Path, "/") {
			c.Request.URL.Path = "/" + c.Request.URL.Path
		}
		embeddedServer.ServeHTTP(c.Writer, c.Request)
	})
}
