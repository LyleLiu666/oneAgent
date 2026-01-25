package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetAuthConfig returns the Keycloak configuration
func GetAuthConfig(c *gin.Context) {
	c.JSON(http.StatusGone, gin.H{
		"error": "OAuth/Keycloak login is deprecated in local tool mode. Use local token auth (Authorization: Bearer <token>).",
	})
}
