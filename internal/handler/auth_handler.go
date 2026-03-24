package handler

import (
	"log/slog"
	"net/http"

	"panel/internal/i18n"
	"panel/internal/middleware"
	"panel/internal/service"
	"panel/internal/view"

	"github.com/gin-gonic/gin"
)

// AuthHandler handles login/logout.
type AuthHandler struct {
	renderer *view.Renderer
	service  *service.AuthService
	limiter  *middleware.LoginRateLimiter
	log      *slog.Logger
}

// NewAuthHandler creates a handler.
func NewAuthHandler(renderer *view.Renderer, service *service.AuthService, limiter *middleware.LoginRateLimiter, log *slog.Logger) *AuthHandler {
	return &AuthHandler{renderer: renderer, service: service, limiter: limiter, log: log}
}

// ShowLogin renders the login page.
func (h *AuthHandler) ShowLogin(c *gin.Context) {
	lang := i18n.FromContext(c)
	h.renderer.HTML(c, http.StatusOK, "auth/login.html", gin.H{
		"Title":     i18n.T(lang, "title.login"),
		"Lang":      lang,
		"CSRFToken": middleware.CSRFToken(c),
	})
}

// Login handles login requests.
func (h *AuthHandler) Login(c *gin.Context) {
	lang := i18n.FromContext(c)
	username := c.PostForm("username")
	password := c.PostForm("password")
	clientIP := c.ClientIP()

	if h.limiter != nil {
		if allowed, _ := h.limiter.Allow(clientIP, username); !allowed {
			auditLog(h.log, c, "login.blocked", "username", username)
			h.renderer.HTML(c, http.StatusTooManyRequests, "auth/login.html", gin.H{
				"Title":     i18n.T(lang, "title.login"),
				"Error":     i18n.T(lang, "login.error.rate_limited"),
				"Lang":      lang,
				"CSRFToken": middleware.CSRFToken(c),
			})
			return
		}
		h.limiter.RegisterAttempt(clientIP, username)
	}

	user, err := h.service.Authenticate(c.Request.Context(), username, password)
	if err != nil {
		if h.limiter != nil {
			h.limiter.RegisterFailure(clientIP, username)
		}
		auditLog(h.log, c, "login.failed", "username", username)
		h.renderer.HTML(c, http.StatusUnauthorized, "auth/login.html", gin.H{
			"Title":     i18n.T(lang, "title.login"),
			"Error":     i18n.T(lang, "login.error.invalid"),
			"Lang":      lang,
			"CSRFToken": middleware.CSRFToken(c),
		})
		return
	}
	if h.limiter != nil {
		h.limiter.RegisterSuccess(clientIP, username)
	}

	c.Set("login_user_id", user.ID)
	auditLog(h.log, c, "login.success", "username", user.Username, "target_user_id", user.ID)
	c.Redirect(http.StatusFound, "/")
}

// Logout handles logout requests.
func (h *AuthHandler) Logout(c *gin.Context) {
	auditLog(h.log, c, "logout")
	c.Redirect(http.StatusFound, "/login")
}
