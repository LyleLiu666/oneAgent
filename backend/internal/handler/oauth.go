package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// OAuthCallback is deprecated in local tool mode.
func OAuthCallback(c *gin.Context) {
	c.JSON(http.StatusGone, gin.H{
		"error": "OAuth/Keycloak login is deprecated in local tool mode. Use local token auth (Authorization: Bearer <token>).",
	})
}

