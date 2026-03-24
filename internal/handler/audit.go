package handler

import (
	"log/slog"

	"github.com/gin-gonic/gin"
)

func auditLog(log *slog.Logger, c *gin.Context, action string, attrs ...any) {
	if log == nil {
		return
	}

	base := []any{
		"action", action,
		"ip", c.ClientIP(),
		"method", c.Request.Method,
		"path", c.Request.URL.Path,
	}

	if userID := currentUserID(c); userID != "" {
		base = append(base, "user_id", userID)
	}

	log.Info("audit", append(base, attrs...)...)
}
