package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/middleware"
	"github.com/liu_y/oneAgent/backend/internal/netutil"
)

// GetAuthConfig returns the Keycloak configuration
func GetAuthConfig(c *gin.Context) {
	c.JSON(http.StatusGone, gin.H{
		"error": "OAuth/Keycloak login is deprecated in local tool mode. Use local token auth (Authorization: Bearer <token>).",
	})
}

// GetRuntimeConfig returns runtime defaults for the UI (workspace-first onboarding, etc).
func GetRuntimeConfig(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Config == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "runtime not initialized"})
		return
	}
	warnings := make([]string, 0, 1)
	if !netutil.IsLoopbackBind(rt.Config.Bind) {
		warnings = append(warnings, fmt.Sprintf("Server is listening on a non-loopback interface (bind=%s). This may expose your local agent to the network; prefer bind=127.0.0.1.", rt.Config.Bind))
	}
	c.JSON(http.StatusOK, gin.H{
		"default_workspace": rt.Config.DefaultWorkspace,
		"base_url":          fmt.Sprintf("http://localhost:%s", rt.Config.Port),
		"warnings":          warnings,
	})
}
