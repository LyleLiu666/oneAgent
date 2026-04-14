package handler

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/middleware"
	"github.com/liu_y/oneAgent/backend/internal/settingsdb"
)

type commandApprovalSettingsResponse struct {
	CommandApprovalMode string `json:"command_approval_mode"`
}

type updateCommandApprovalSettingsRequest struct {
	CommandApprovalMode string `json:"command_approval_mode"`
}

func GetCommandApprovalSettings(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Settings == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Settings not available"})
		return
	}

	userID := middleware.GetUserID(c)
	if strings.TrimSpace(userID) == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	mode, err := resolveCommandApprovalMode(c.Request.Context(), rt.Settings, userID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, commandApprovalSettingsResponse{CommandApprovalMode: mode})
}

func UpdateCommandApprovalSettings(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Settings == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Settings not available"})
		return
	}

	userID := middleware.GetUserID(c)
	if strings.TrimSpace(userID) == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req updateCommandApprovalSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	mode, ok := parseCommandApprovalMode(req.CommandApprovalMode)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "command_approval_mode must be auto|manual"})
		return
	}

	if err := rt.Settings.SetUserSetting(c.Request.Context(), userID, settingsdb.SettingKeyCommandApprovalMode, mode); err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, commandApprovalSettingsResponse{CommandApprovalMode: mode})
}

func resolveCommandApprovalMode(ctx context.Context, db *settingsdb.DB, userID string) (string, error) {
	if db == nil {
		return "", errors.New("settings db is not initialized")
	}

	mode := "auto"
	val, err := db.GetUserSetting(ctx, userID, settingsdb.SettingKeyCommandApprovalMode)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return mode, nil
		}
		return "", err
	}

	mode = normalizeCommandApprovalMode(val)
	if mode == "manual" {
		return mode, nil
	}
	return "auto", nil
}

func normalizeCommandApprovalMode(raw string) string {
	mode := strings.ToLower(strings.TrimSpace(raw))
	if mode == "manual" {
		return "manual"
	}
	return "auto"
}

func parseCommandApprovalMode(raw string) (string, bool) {
	mode := strings.ToLower(strings.TrimSpace(raw))
	switch mode {
	case "auto", "manual":
		return mode, true
	default:
		return "", false
	}
}
