package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const csrfContextKey = "csrf_token"

// CSRFMiddleware issues and validates CSRF tokens for unsafe requests.
func CSRFMiddleware(store *SessionStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if shouldSkipCSRFMiddleware(c.Request.URL.Path) {
			c.Next()
			return
		}

		token, err := store.EnsureCSRFToken(c)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to initialize csrf token"})
			return
		}
		c.Set(csrfContextKey, token)

		if isSafeMethod(c.Request.Method) {
			c.Next()
			return
		}

		requestToken := strings.TrimSpace(c.GetHeader("X-CSRF-Token"))
		if requestToken == "" {
			requestToken = strings.TrimSpace(c.PostForm("_csrf"))
		}
		if requestToken == "" || requestToken != token {
			if strings.Contains(c.GetHeader("Accept"), "application/json") {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid csrf token"})
				return
			}
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		c.Next()
	}
}

// CSRFToken returns the request CSRF token.
func CSRFToken(c *gin.Context) string {
	if value, ok := c.Get(csrfContextKey); ok {
		if token, ok := value.(string); ok {
			return token
		}
	}
	return ""
}

func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

func shouldSkipCSRFMiddleware(path string) bool {
	if strings.HasPrefix(path, "/static/") {
		return true
	}
	switch path {
	case "/healthz", "/api/weather/current":
		return true
	default:
		return false
	}
}
