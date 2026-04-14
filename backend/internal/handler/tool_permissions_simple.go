package handler

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/middleware"
	"github.com/liu_y/oneAgent/backend/internal/permissions"
	oneruntime "github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/settingsdb"
)

type simpleToolPermissionMode struct {
	Mode              string `json:"mode"`
	Label             string `json:"label"`
	Description       string `json:"description"`
	RiskLevel         string `json:"risk_level"`
	Available         bool   `json:"available"`
	UnavailableReason string `json:"unavailable_reason,omitempty"`
	Current           bool   `json:"current"`
	Recommended       bool   `json:"recommended"`
}

type simpleToolPermissionScope struct {
	Summary                  string   `json:"summary"`
	Affects                  []string `json:"affects"`
	DoesNotAffect            []string `json:"does_not_affect"`
	FutureExecutionsOnly     bool     `json:"future_executions_only"`
	RunningAttemptsUnchanged bool     `json:"running_attempts_unchanged"`
	SecretaryRemainsReadOnly bool     `json:"secretary_remains_read_only"`
}

type simpleToolPermissionResponse struct {
	PrincipalID               string                     `json:"principal_id"`
	CurrentMode               string                     `json:"current_mode"`
	CommandApprovalMode       string                     `json:"command_approval_mode"`
	AvailableModes            []simpleToolPermissionMode `json:"available_modes"`
	EffectiveScope            simpleToolPermissionScope  `json:"effective_scope"`
	Snapshot                  permissions.Snapshot       `json:"snapshot"`
	AdvancedSettingsAvailable bool                       `json:"advanced_settings_available"`
}

type updateSimpleToolPermissionRequest struct {
	PrincipalID         string `json:"principal_id,omitempty"`
	Mode                string `json:"mode,omitempty"`
	CommandApprovalMode string `json:"command_approval_mode,omitempty"`
	Source              string `json:"source,omitempty"`
}

const (
	simpleToolPermissionSourceChatHeader      = "chat_header"
	simpleToolPermissionSourceSecretaryPrompt = "secretary_prompt"
	simpleToolPermissionSourceToolPage        = "tool_permissions_page"
)

func GetSimpleToolPermissions(c *gin.Context) {
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

	principalID := strings.TrimSpace(middleware.GetUserID(c))
	if principalID == "" {
		RespondError(c, http.StatusUnauthorized, &PublicError{
			Status: http.StatusUnauthorized,
			Code:   "unauthorized",
			Public: "未授权",
			Hint:   "请登录或提供有效 token",
		})
		return
	}

	resp, err := buildSimpleToolPermissionResponse(c.Request.Context(), rt.Settings, rt, principalID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func UpdateSimpleToolPermissions(c *gin.Context) {
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

	principalID := strings.TrimSpace(middleware.GetUserID(c))
	if principalID == "" {
		RespondError(c, http.StatusUnauthorized, &PublicError{
			Status: http.StatusUnauthorized,
			Code:   "unauthorized",
			Public: "未授权",
			Hint:   "请登录或提供有效 token",
		})
		return
	}

	var req updateSimpleToolPermissionRequest
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

	if strings.TrimSpace(req.PrincipalID) != "" {
		RespondError(c, http.StatusBadRequest, &PublicError{
			Status: http.StatusBadRequest,
			Code:   "principal_scope_violation",
			Public: "简单权限接口只允许修改当前账户自己的权限",
			Hint:   "不要在 simple API 里传 principal_id；跨 principal 管理请走管理员接口",
		})
		return
	}

	source, ok := parseSimpleToolPermissionSource(req.Source)
	if !ok {
		RespondError(c, http.StatusBadRequest, &PublicError{
			Status: http.StatusBadRequest,
			Code:   "invalid_simple_change_source",
			Public: "source 只能是 chat_header、secretary_prompt 或 tool_permissions_page",
			Hint:   "不传时会默认记为 tool_permissions_page",
		})
		return
	}

	oldSnapshot, err := rt.ResolveToolPolicySnapshot(c.Request.Context(), principalID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}
	oldPolicy := oldSnapshot.Policy
	selectedMode := permissions.ClassifySimpleMode(oldPolicy)

	requestedMode := permissions.SimpleMode(strings.ToLower(strings.TrimSpace(req.Mode)))
	newPolicy := oldPolicy
	skipPolicyPersistence := requestedMode == ""
	if requestedMode != "" {
		if requestedMode == permissions.SimpleModeCustom {
			RespondError(c, http.StatusBadRequest, &PublicError{
				Status: http.StatusBadRequest,
				Code:   "simple_mode_not_supported",
				Public: "simple API 只能应用预设模式，不能直接保存自定义策略",
				Hint:   "如需自定义，请使用高级设置",
			})
			return
		}
		var buildErr error
		newPolicy, buildErr = permissions.BuildSimpleModePolicy(requestedMode)
		if buildErr != nil {
			var unavailable *permissions.SimpleModeUnavailableError
			if errors.As(buildErr, &unavailable) {
				RespondError(c, http.StatusConflict, &PublicError{
					Status: http.StatusConflict,
					Code:   "sandbox_mode_unavailable",
					Public: unavailable.Error(),
				})
				return
			}
			RespondError(c, http.StatusBadRequest, buildErr)
			return
		}
		selectedMode = requestedMode
	}

	var commandApprovalMode string
	if strings.TrimSpace(req.CommandApprovalMode) == "" {
		commandApprovalMode, err = resolveCommandApprovalMode(c.Request.Context(), rt.Settings, principalID)
		if err != nil {
			RespondError(c, http.StatusInternalServerError, err)
			return
		}
	} else {
		var ok bool
		commandApprovalMode, ok = parseCommandApprovalMode(req.CommandApprovalMode)
		if !ok {
			RespondError(c, http.StatusBadRequest, &PublicError{
				Status: http.StatusBadRequest,
				Code:   "invalid_command_approval_mode",
				Public: "高风险命令审批方式只能是 auto 或 manual",
				Hint:   "请传入 auto 或 manual",
			})
			return
		}
	}

	if requestedMode == "" && strings.TrimSpace(req.CommandApprovalMode) == "" {
		RespondError(c, http.StatusBadRequest, &PublicError{
			Status: http.StatusBadRequest,
			Code:   "missing_simple_change",
			Public: "至少要提供一个预设模式，或更新高风险命令审批方式",
		})
		return
	}

	changedAt := time.Now().UTC()
	if err := rt.Settings.ApplySimpleToolPermissionChange(c.Request.Context(), settingsdb.ToolPermissionSimpleAuditRecord{
		PrincipalID:           principalID,
		Source:                source,
		OldPolicy:             oldPolicy,
		NewPolicy:             newPolicy,
		SelectedMode:          string(selectedMode),
		CommandApprovalMode:   commandApprovalMode,
		SkipPolicyPersistence: skipPolicyPersistence,
		ChangedAt:             changedAt,
	}); err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}

	log.Printf("tool_permission_simple_change principal=%s source=%s old_policy_hash=%s new_policy_hash=%s selected_mode=%s command_approval_mode=%s changed_at=%s", principalID, source, oldSnapshot.PolicyHash, permissions.HashPolicy(newPolicy), selectedMode, commandApprovalMode, changedAt.Format(time.RFC3339))

	resp, err := buildSimpleToolPermissionResponse(c.Request.Context(), rt.Settings, rt, principalID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func buildSimpleToolPermissionResponse(ctx context.Context, db *settingsdb.DB, rt *oneruntime.Runtime, principalID string) (simpleToolPermissionResponse, error) {
	snapshot, err := rt.ResolveToolPolicySnapshot(ctx, principalID)
	if err != nil {
		return simpleToolPermissionResponse{}, err
	}
	commandApprovalMode, err := resolveCommandApprovalMode(ctx, db, principalID)
	if err != nil {
		return simpleToolPermissionResponse{}, err
	}

	currentMode := permissions.ClassifySimpleMode(snapshot.Policy)
	return simpleToolPermissionResponse{
		PrincipalID:               principalID,
		CurrentMode:               string(currentMode),
		CommandApprovalMode:       commandApprovalMode,
		AvailableModes:            buildSimpleToolPermissionModes(currentMode),
		EffectiveScope:            simpleToolPermissionEffectiveScope(),
		Snapshot:                  snapshot,
		AdvancedSettingsAvailable: principalID == "local",
	}, nil
}

func buildSimpleToolPermissionModes(current permissions.SimpleMode) []simpleToolPermissionMode {
	presets := permissions.PresetSimpleModes()
	out := make([]simpleToolPermissionMode, 0, len(presets))
	hardBoundaryMode, hardBoundaryAvailable, hardBoundaryReason := permissions.DetectPreferredHardBoundarySandbox()
	for _, mode := range presets {
		details := permissions.SimpleModeDetails(mode)
		item := simpleToolPermissionMode{
			Mode:        string(mode),
			Label:       details.Label,
			Description: details.Description,
			RiskLevel:   details.RiskLevel,
			Available:   true,
			Current:     mode == current,
			Recommended: mode == permissions.SimpleModeSandboxCoding,
		}
		if mode == permissions.SimpleModeSandboxCoding && !hardBoundaryAvailable {
			item.Available = false
			item.UnavailableReason = hardBoundaryReason
		}
		if mode == permissions.SimpleModeSandboxCoding && hardBoundaryAvailable {
			item.Description = details.Description + " 当前会使用 " + string(hardBoundaryMode) + " 作为隔离边界。"
		}
		out = append(out, item)
	}
	return out
}

func simpleToolPermissionEffectiveScope() simpleToolPermissionScope {
	return simpleToolPermissionScope{
		Summary: "切换后会影响后续 worker、普通聊天和新建或重试后的任务尝试；不会让秘书自己直接获得写权限，也不会改变正在运行中的任务。",
		Affects: []string{
			"完整聊天后续回合",
			"后续派发给 worker 的执行",
			"新建任务与后续重试",
		},
		DoesNotAffect: []string{
			"秘书 SU/SW 自身仍保持只读",
			"已经在运行中的 task attempt",
		},
		FutureExecutionsOnly:     true,
		RunningAttemptsUnchanged: true,
		SecretaryRemainsReadOnly: true,
	}
}

func parseSimpleToolPermissionSource(raw string) (string, bool) {
	source := strings.ToLower(strings.TrimSpace(raw))
	if source == "" {
		return simpleToolPermissionSourceToolPage, true
	}
	switch source {
	case simpleToolPermissionSourceChatHeader,
		simpleToolPermissionSourceSecretaryPrompt,
		simpleToolPermissionSourceToolPage:
		return source, true
	default:
		return "", false
	}
}
