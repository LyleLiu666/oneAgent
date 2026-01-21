package bocha

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/database"
	"github.com/liu_y/oneAgent/backend/internal/middleware"
	"github.com/liu_y/oneAgent/backend/internal/model"
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
	db := database.GetDB()
	apiKey, err := model.GetUserSetting(db, userID, model.SettingKeyBochaAPIKey)
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
