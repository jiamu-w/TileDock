package middleware

import (
	"log/slog"
	"net/http"
	"panel/internal/i18n"
	"strings"

	"github.com/gin-gonic/gin"
)

// ErrorHandler converts handler errors into consistent responses.
func ErrorHandler(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) == 0 {
			return
		}

		err := c.Errors.Last().Err
		log.Error("request failed", "path", c.Request.URL.Path, "method", c.Request.Method, "error", err)

		if c.Writer.Written() {
			return
		}

		if strings.Contains(err.Error(), "bind") {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		accept := c.GetHeader("Accept")
		lang := i18n.FromContext(c)
		if strings.Contains(accept, "application/json") || c.FullPath() == "/healthz" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": i18n.T(lang, "error.message")})
			return
		}

		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"Title":   i18n.T(lang, "title.error"),
			"Message": i18n.T(lang, "error.message"),
			"Lang":    lang,
		})
	}
}
