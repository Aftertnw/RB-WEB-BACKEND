package httpapi

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func RequireRole(allowed ...string) gin.HandlerFunc {
	allow := map[string]bool{}
	for _, r := range allowed {
		allow[strings.ToLower(strings.TrimSpace(r))] = true
	}

	return func(c *gin.Context) {
		role := strings.ToLower(strings.TrimSpace(c.GetString("userRole")))
		if role == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}
		if !allow[role] {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			c.Abort()
			return
		}
		c.Next()
	}
}
