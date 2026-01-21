package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/liu_y/oneAgent/backend/internal/config"
)

// GetAuthConfig returns the Keycloak configuration
func GetAuthConfig(c *gin.Context) {
	cfg := config.GetConfig()
	c.JSON(http.StatusOK, gin.H{
		"url":      cfg.KeycloakURL,
		"realm":    cfg.KeycloakRealm,
		"clientId": cfg.KeycloakClientID,
	})
}
