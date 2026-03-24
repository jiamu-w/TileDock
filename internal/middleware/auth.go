package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// AuthRequired ensures users are logged in.
func AuthRequired(store *SessionStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := store.GetUserID(c)
		if err != nil || userID == "" {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		c.Set(sessionUserIDKey, userID)
		c.Next()
	}
}
