package handler

import (
	"net/http"

	"panel/internal/i18n"
	"panel/internal/middleware"
	"panel/internal/service"
	"panel/internal/view"

	"github.com/gin-gonic/gin"
)

// DashboardHandler handles dashboard pages.
type DashboardHandler struct {
	renderer *view.Renderer
	service  *service.DashboardService
	auth     *service.AuthService
}

// NewDashboardHandler creates a handler.
func NewDashboardHandler(renderer *view.Renderer, service *service.DashboardService, auth *service.AuthService) *DashboardHandler {
	return &DashboardHandler{renderer: renderer, service: service, auth: auth}
}

// Index renders the dashboard home.
func (h *DashboardHandler) Index(c *gin.Context) {
	lang := i18n.FromContext(c)
	currentUserID, _ := c.Get("current_user_id")
	currentUsername := ""
	if userID, ok := currentUserID.(string); ok && userID != "" {
		user, err := h.auth.GetUserByID(c.Request.Context(), userID)
		if err != nil {
			_ = c.Error(err)
			return
		}
		currentUsername = user.Username
	}

	csrfToken := middleware.CSRFToken(c)
	data, err := h.service.GetDashboardData(c.Request.Context(), currentUsernameString(currentUsername), lang, csrfToken)
	if err != nil {
		_ = c.Error(err)
		return
	}

	h.renderer.HTML(c, http.StatusOK, "dashboard/index.html", gin.H{
		"Title":        "TileDock",
		"Data":         data,
		"Lang":         lang,
		"CSRFToken":    csrfToken,
		"SettingsOpen": c.Query("panel") == "settings",
		"Success":      c.Query("success"),
		"Error":        c.Query("error"),
	})
}

func currentUsernameString(value any) string {
	if username, ok := value.(string); ok {
		return username
	}
	return ""
}
