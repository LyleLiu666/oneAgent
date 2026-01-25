package bocha

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/middleware"
	"github.com/liu_y/oneAgent/backend/internal/settingsdb"
)

// searchHandlerRequest represents the request body for Bocha search.
type searchHandlerRequest struct {
	Query     string `json:"query" binding:"required"`
	Summary   bool   `json:"summary,omitempty"`
	Freshness string `json:"freshness,omitempty"` // noLimit, oneDay, oneWeek, oneMonth, oneYear
	Count     int    `json:"count,omitempty"`     // 1-50, default 10
}

// SearchHandler performs a search using the Bocha API.
// POST /api/search/bocha
func SearchHandler(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req searchHandlerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	// Get user's Bocha API key
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Settings == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Settings not available"})
		return
	}

	apiKey, err := rt.Settings.GetUserSetting(c.Request.Context(), userID, settingsdb.SettingKeyBochaAPIKey)
	if err != nil || apiKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Bocha API key not configured. Please set it in Settings."})
		return
	}

	// Perform search
	searchReq := SearchRequest{
		Query:     req.Query,
		Summary:   req.Summary,
		Freshness: req.Freshness,
		Count:     req.Count,
	}

	resp, err := Search(apiKey, searchReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Search failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}
