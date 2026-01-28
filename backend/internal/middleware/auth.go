package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/runtime"
)

// GetUserID extracts user ID from Gin context
func GetUserID(c *gin.Context) string {
	if userID, exists := c.Get("user_id"); exists {
		return userID.(string)
	}
	return ""
}

// GetUsername extracts username from Gin context
func GetUsername(c *gin.Context) string {
	if username, exists := c.Get("username"); exists {
		return username.(string)
	}
	return ""
}

func normalizePath(path string) string {
	if path == "" {
		return "/"
	}
	if path == "/" {
		return "/"
	}
	path = strings.TrimRight(path, "/")
	if path == "" {
		return "/"
	}
	return path
}

func APIAuth(rt *runtime.Runtime, publicPaths ...string) gin.HandlerFunc {
	public := make(map[string]struct{}, len(publicPaths))
	for _, p := range publicPaths {
		public[normalizePath(p)] = struct{}{}
	}

	return func(c *gin.Context) {
		if c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}

		path := normalizePath(c.Request.URL.Path)
		if !strings.HasPrefix(path, "/api") {
			c.Next()
			return
		}

		if _, ok := public[path]; ok {
			c.Next()
			return
		}

		if rt == nil || rt.Config == nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "runtime not initialized"})
			return
		}

		switch strings.ToLower(strings.TrimSpace(rt.Config.AuthMode)) {
		case "none":
			injectUser(c, "local")
			c.Next()
			return
		case "token":
			userID, ok := authenticateLocalToken(c, rt)
			if !ok {
				return
			}
			injectUser(c, userID)
			c.Next()
			return
		default:
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "invalid auth mode"})
			return
		}
	}
}

func authenticateLocalToken(c *gin.Context, rt *runtime.Runtime) (string, bool) {
	// Extract token from Authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "Authorization header required",
		})
		return "", false
	}

	// Parse Bearer token
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid authorization header format",
		})
		return "", false
	}

	tokenString := strings.TrimSpace(parts[1])
	if tokenString == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return "", false
	}

	// Prefer settingsdb token mapping when available.
	if rt != nil && rt.Settings != nil {
		principalID, ok, err := rt.Settings.LookupAuthToken(c.Request.Context(), tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return "", false
		}
		if ok {
			return principalID, true
		}
	}

	// Backward-compatible fallback to the single local token file.
	if rt != nil && strings.TrimSpace(rt.AuthToken) != "" && tokenString == rt.AuthToken {
		return "local", true
	}

	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
	return "", false
}

func injectUser(c *gin.Context, userID string) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		userID = "local"
	}
	c.Set("user_id", userID)
	c.Set("username", userID)
}
