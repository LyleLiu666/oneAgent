package bocha

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/middleware"
	"github.com/liu_y/oneAgent/backend/internal/settingsdb"
)

// updateSettingsRequest represents the request body for updating settings.
type updateSettingsRequest struct {
	BochaAPIKey *string `json:"bocha_api_key,omitempty"`
}

// settingsResponse represents the response for settings.
type settingsResponse struct {
	HasBochaAPIKey bool `json:"has_bocha_api_key"`
}

// GetSettingsHandler returns the current user's Bocha settings.
// GET /api/bocha/settings
func GetSettingsHandler(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Settings == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Settings not available"})
		return
	}

	settingsMap, err := rt.Settings.GetUserSettingsMap(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load settings"})
		return
	}

	// Build response
	resp := settingsResponse{
		HasBochaAPIKey: false,
	}
	if hasBochaKey, ok := settingsMap["has_bocha_api_key"].(bool); ok {
		resp.HasBochaAPIKey = hasBochaKey
	}

	c.JSON(http.StatusOK, resp)
}

// UpdateSettingsHandler updates the current user's Bocha settings.
// PUT /api/bocha/settings
func UpdateSettingsHandler(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req updateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Settings == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Settings not available"})
		return
	}

	// Update Bocha API key if provided
	if req.BochaAPIKey != nil {
		apiKey := strings.TrimSpace(*req.BochaAPIKey)
		if err := rt.Settings.SetUserSetting(c.Request.Context(), userID, settingsdb.SettingKeyBochaAPIKey, apiKey); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save settings"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Settings updated successfully"})
}
