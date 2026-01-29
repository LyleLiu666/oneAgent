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
		RespondError(c, http.StatusForbidden, &PublicError{
			Status: http.StatusForbidden,
			Code:   "admin_access_required",
			Public: "需要本地管理员权限",
			Hint:   "请使用 local 账户或在本地环境访问该页面",
		})
		return false
	}
	return true
}

func CreateAuthToken(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Settings == nil {
		RespondError(c, http.StatusServiceUnavailable, &PublicError{
			Status: http.StatusServiceUnavailable,
			Code:   "runtime_not_initialized",
			Public: "服务未就绪",
			Hint:   "请稍后重试",
		})
		return
	}
	if !requireLocalAdmin(c) {
		return
	}

	var req createAuthTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, &PublicError{
			Status: http.StatusBadRequest,
			Code:   "invalid_json",
			Public: "请求 JSON 格式不正确",
			Hint:   "请检查 JSON 字段与语法",
			Err:    err,
		})
		return
	}
	principalID := strings.TrimSpace(req.PrincipalID)
	if principalID == "" {
		principalID = middleware.GetUserID(c)
	}

	token, err := rt.Settings.CreateAuthToken(c.Request.Context(), principalID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err)
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
		RespondError(c, http.StatusServiceUnavailable, &PublicError{
			Status: http.StatusServiceUnavailable,
			Code:   "runtime_not_initialized",
			Public: "服务未就绪",
			Hint:   "请稍后重试",
		})
		return
	}
	if !requireLocalAdmin(c) {
		return
	}

	list, err := rt.Settings.ListAuthTokens(c.Request.Context())
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err)
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
		RespondError(c, http.StatusServiceUnavailable, &PublicError{
			Status: http.StatusServiceUnavailable,
			Code:   "runtime_not_initialized",
			Public: "服务未就绪",
			Hint:   "请稍后重试",
		})
		return
	}
	if !requireLocalAdmin(c) {
		return
	}

	var req revokeAuthTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, &PublicError{
			Status: http.StatusBadRequest,
			Code:   "invalid_json",
			Public: "请求 JSON 格式不正确",
			Hint:   "请检查 JSON 字段与语法",
			Err:    err,
		})
		return
	}
	token := strings.TrimSpace(req.Token)
	if token == "" {
		RespondError(c, http.StatusBadRequest, &PublicError{
			Status: http.StatusBadRequest,
			Code:   "missing_token",
			Public: "token 是必填项",
		})
		return
	}
	if err := rt.Settings.RevokeAuthToken(c.Request.Context(), token); err != nil {
		RespondError(c, http.StatusInternalServerError, err)
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
		RespondError(c, http.StatusServiceUnavailable, &PublicError{
			Status: http.StatusServiceUnavailable,
			Code:   "runtime_not_initialized",
			Public: "服务未就绪",
			Hint:   "请稍后重试",
		})
		return
	}
	if !requireLocalAdmin(c) {
		return
	}

	principalID := strings.TrimSpace(c.Param("principal_id"))
	if principalID == "" {
		RespondError(c, http.StatusBadRequest, &PublicError{
			Status: http.StatusBadRequest,
			Code:   "missing_principal_id",
			Public: "principal_id 是必填项",
		})
		return
	}
	policy, exists, err := rt.Settings.GetToolPolicy(c.Request.Context(), principalID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}
	snap, err := rt.ResolveToolPolicySnapshot(c.Request.Context(), principalID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err)
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
		RespondError(c, http.StatusServiceUnavailable, &PublicError{
			Status: http.StatusServiceUnavailable,
			Code:   "runtime_not_initialized",
			Public: "服务未就绪",
			Hint:   "请稍后重试",
		})
		return
	}
	if !requireLocalAdmin(c) {
		return
	}

	principalID := strings.TrimSpace(c.Param("principal_id"))
	if principalID == "" {
		RespondError(c, http.StatusBadRequest, &PublicError{
			Status: http.StatusBadRequest,
			Code:   "missing_principal_id",
			Public: "principal_id 是必填项",
		})
		return
	}

	var policy permissions.Policy
	if err := c.ShouldBindJSON(&policy); err != nil {
		RespondError(c, http.StatusBadRequest, &PublicError{
			Status: http.StatusBadRequest,
			Code:   "invalid_json",
			Public: "请求 JSON 格式不正确",
			Hint:   "请检查 JSON 字段与语法",
			Err:    err,
		})
		return
	}
	if strings.TrimSpace(policy.ID) == "" {
		policy.ID = "custom"
	}
	for _, rule := range policy.Rules {
		if strings.TrimSpace(rule.ID) == "" {
			RespondError(c, http.StatusBadRequest, &PublicError{
				Status: http.StatusBadRequest,
				Code:   "missing_rule_id",
				Public: "policy rule id 是必填项",
			})
			return
		}
		if strings.TrimSpace(rule.ToolID) == "" {
			RespondError(c, http.StatusBadRequest, &PublicError{
				Status: http.StatusBadRequest,
				Code:   "missing_rule_tool_id",
				Public: "policy rule tool_id 是必填项",
			})
			return
		}
		if rule.Effect != permissions.EffectAllow && rule.Effect != permissions.EffectDeny {
			RespondError(c, http.StatusBadRequest, &PublicError{
				Status: http.StatusBadRequest,
				Code:   "invalid_rule_effect",
				Public: "policy rule effect 必须为 allow|deny",
			})
			return
		}
	}

	if err := rt.Settings.SetToolPolicy(c.Request.Context(), principalID, policy); err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}
	snap, err := rt.ResolveToolPolicySnapshot(c.Request.Context(), principalID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, toolPolicyResponse{
		PrincipalID: principalID,
		Exists:      true,
		Policy:      policy,
		Snapshot:    snap,
	})
}
