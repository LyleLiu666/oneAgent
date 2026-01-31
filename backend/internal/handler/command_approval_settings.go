package handler

import (
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

	mode := "auto"
	val, err := rt.Settings.GetUserSetting(c.Request.Context(), userID, settingsdb.SettingKeyCommandApprovalMode)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			RespondError(c, http.StatusInternalServerError, err)
			return
		}
	} else {
		v := strings.ToLower(strings.TrimSpace(val))
		if v == "manual" {
			mode = "manual"
		}
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

	mode := strings.ToLower(strings.TrimSpace(req.CommandApprovalMode))
	if mode != "auto" && mode != "manual" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "command_approval_mode must be auto|manual"})
		return
	}

	if err := rt.Settings.SetUserSetting(c.Request.Context(), userID, settingsdb.SettingKeyCommandApprovalMode, mode); err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, commandApprovalSettingsResponse{CommandApprovalMode: mode})
}

