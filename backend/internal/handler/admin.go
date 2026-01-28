package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/middleware"
	"github.com/liu_y/oneAgent/backend/internal/permissions"
)

type createAuthTokenRequest struct {
	PrincipalID string `json:"principal_id"`
}

type revokeAuthTokenRequest struct {
	Token string `json:"token"`
}

type authTokenResponse struct {
	Token       string  `json:"token"`
	PrincipalID string  `json:"principal_id"`
	CreatedAt   string  `json:"created_at"`
	RevokedAt   *string `json:"revoked_at,omitempty"`
}

func requireLocalAdmin(c *gin.Context) bool {
	if middleware.GetUserID(c) != "local" {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
		return false
	}
	return true
}

func CreateAuthToken(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Settings == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "runtime not initialized"})
		return
	}
	if !requireLocalAdmin(c) {
		return
	}

	var req createAuthTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	principalID := strings.TrimSpace(req.PrincipalID)
	if principalID == "" {
		principalID = middleware.GetUserID(c)
	}

	token, err := rt.Settings.CreateAuthToken(c.Request.Context(), principalID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, authTokenResponse{
		Token:       token.Token,
		PrincipalID: token.PrincipalID,
		CreatedAt:   token.CreatedAt.UTC().Format(time.RFC3339),
	})
}

func ListAuthTokens(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Settings == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "runtime not initialized"})
		return
	}
	if !requireLocalAdmin(c) {
		return
	}

	list, err := rt.Settings.ListAuthTokens(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]authTokenResponse, 0, len(list))
	for _, t := range list {
		var revokedAt *string
		if t.RevokedAt != nil {
			s := t.RevokedAt.UTC().Format(time.RFC3339)
			revokedAt = &s
		}
		out = append(out, authTokenResponse{
			Token:       t.Token,
			PrincipalID: t.PrincipalID,
			CreatedAt:   t.CreatedAt.UTC().Format(time.RFC3339),
			RevokedAt:   revokedAt,
		})
	}
	c.JSON(http.StatusOK, out)
}

func RevokeAuthToken(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Settings == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "runtime not initialized"})
		return
	}
	if !requireLocalAdmin(c) {
		return
	}

	var req revokeAuthTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	token := strings.TrimSpace(req.Token)
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token is required"})
		return
	}
	if err := rt.Settings.RevokeAuthToken(c.Request.Context(), token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type toolPolicyResponse struct {
	PrincipalID string               `json:"principal_id"`
	Exists      bool                 `json:"exists"`
	Policy      permissions.Policy   `json:"policy"`
	Snapshot    permissions.Snapshot `json:"snapshot"`
}

func GetToolPolicy(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Settings == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "runtime not initialized"})
		return
	}
	if !requireLocalAdmin(c) {
		return
	}

	principalID := strings.TrimSpace(c.Param("principal_id"))
	if principalID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "principal_id is required"})
		return
	}
	policy, exists, err := rt.Settings.GetToolPolicy(c.Request.Context(), principalID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	snap, err := rt.ResolveToolPolicySnapshot(c.Request.Context(), principalID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, toolPolicyResponse{
		PrincipalID: principalID,
		Exists:      exists,
		Policy:      policy,
		Snapshot:    snap,
	})
}

func SetToolPolicy(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Settings == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "runtime not initialized"})
		return
	}
	if !requireLocalAdmin(c) {
		return
	}

	principalID := strings.TrimSpace(c.Param("principal_id"))
	if principalID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "principal_id is required"})
		return
	}

	var policy permissions.Policy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(policy.ID) == "" {
		policy.ID = "custom"
	}
	for _, rule := range policy.Rules {
		if strings.TrimSpace(rule.ID) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "policy rule id is required"})
			return
		}
		if strings.TrimSpace(rule.ToolID) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "policy rule tool_id is required"})
			return
		}
		if rule.Effect != permissions.EffectAllow && rule.Effect != permissions.EffectDeny {
			c.JSON(http.StatusBadRequest, gin.H{"error": "policy rule effect must be allow|deny"})
			return
		}
	}

	if err := rt.Settings.SetToolPolicy(c.Request.Context(), principalID, policy); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	snap, err := rt.ResolveToolPolicySnapshot(c.Request.Context(), principalID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, toolPolicyResponse{
		PrincipalID: principalID,
		Exists:      true,
		Policy:      policy,
		Snapshot:    snap,
	})
}
