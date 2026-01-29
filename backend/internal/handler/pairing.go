package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/middleware"
)

type createPairingCodeRequest struct {
	PrincipalID string `json:"principal_id"`
	TTLSeconds  int    `json:"ttl_seconds,omitempty"`
}

type pairingCodeResponse struct {
	Code        string `json:"code"`
	PrincipalID string `json:"principal_id"`
	ExpiresAt   string `json:"expires_at"`
}

func CreatePairingCode(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Pairing == nil || rt.Settings == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "runtime not initialized"})
		return
	}
	if !requireLocalAdmin(c) {
		return
	}

	var req createPairingCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err)
		return
	}

	principalID := strings.TrimSpace(req.PrincipalID)
	if principalID == "" {
		principalID = middleware.GetUserID(c)
	}

	ttl := time.Duration(req.TTLSeconds) * time.Second
	if req.TTLSeconds == 0 {
		ttl = 60 * time.Second
	}

	pc, err := rt.Pairing.Create(principalID, ttl)
	if err != nil {
		RespondError(c, http.StatusBadRequest, err)
		return
	}

	c.JSON(http.StatusOK, pairingCodeResponse{
		Code:        pc.Code,
		PrincipalID: pc.PrincipalID,
		ExpiresAt:   pc.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

type exchangePairingCodeRequest struct {
	Code string `json:"code"`
}

type exchangePairingCodeResponse struct {
	Token       string `json:"token"`
	PrincipalID string `json:"principal_id"`
	CreatedAt   string `json:"created_at"`
}

func ExchangePairingCode(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Pairing == nil || rt.Settings == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "runtime not initialized"})
		return
	}

	var req exchangePairingCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	code := strings.TrimSpace(req.Code)
	pc, err := rt.Pairing.Exchange(code)
	if err != nil {
		RespondError(c, http.StatusBadRequest, err)
		return
	}

	token, err := rt.Settings.CreateAuthToken(c.Request.Context(), pc.PrincipalID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, exchangePairingCodeResponse{
		Token:       token.Token,
		PrincipalID: token.PrincipalID,
		CreatedAt:   token.CreatedAt.UTC().Format(time.RFC3339),
	})
}
