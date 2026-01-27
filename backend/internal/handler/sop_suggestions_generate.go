package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/middleware"
	"github.com/liu_y/oneAgent/backend/internal/workledger"
)

type generateSuggestionsRequest struct {
	LookbackDays int `json:"lookback_days,omitempty"`
	Count        int `json:"count,omitempty"`
}

// GenerateSuggestions is a v1 best-effort generator that proposes a SOP suggestion
// from recent receipts. It is user-triggered and does not run automatically.
func GenerateSuggestions(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.WorkLedger == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "work ledger not initialized"})
		return
	}

	principal := strings.TrimSpace(middleware.GetUserID(c))
	if principal == "" {
		principal = "local"
	}

	var req generateSuggestionsRequest
	_ = c.ShouldBindJSON(&req)

	created, err := rt.WorkLedger.GenerateSuggestionsV1(c.Request.Context(), workledger.GenerateSuggestionsInput{
		PrincipalID:  principal,
		LookbackDays: req.LookbackDays,
		Count:        req.Count,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, created)
}
