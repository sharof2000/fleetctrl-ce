package api

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware validates JWT tokens for protected routes
func (r *Router) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var token string

		// Check Authorization header first
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && parts[0] == "Bearer" {
				token = parts[1]
			}
		}

		// Fall back to query param (for SSE connections which can't set headers)
		if token == "" {
			token = c.Query("token")
		}

		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization required"})
			c.Abort()
			return
		}

		claims, err := r.authService.ValidateToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		c.Set("username", claims.Username)
		c.Next()
	}
}

// HostAuthMiddleware validates host token for peer communication
func (r *Router) HostAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("X-Host-Token")
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Host token required"})
			c.Abort()
			return
		}

		// Check token against configured hosts
		validToken := false
		for _, host := range r.config.Hosts {
			if host.Token != "" && subtle.ConstantTimeCompare([]byte(token), []byte(host.Token)) == 1 {
				validToken = true
				break
			}
		}

		if !validToken {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid host token"})
			c.Abort()
			return
		}

		c.Next()
	}
}
