package tool

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/permissions"
	"github.com/liu_y/oneAgent/backend/internal/settingsdb"
	"github.com/liu_y/oneAgent/backend/internal/shell"
)

func requireToolApprovalIfNeeded(ctx context.Context, toolID string, raw json.RawMessage) error {
	toolID = strings.TrimSpace(toolID)
	if toolID == "" {
		return errors.New("tool_id is required")
	}

	dec, err := RequirePolicy(ctx, toolID)
	if err != nil {
		return err
	}
	approval := strings.ToLower(strings.TrimSpace(dec.Constraints.Approval))
	if approval == "" {
		return nil
	}

	needsApproval := false
	autoApprove := false
	switch approval {
	case "required":
		needsApproval = true
	case "high_risk":
		if !isHighRiskToolCallV1(toolID, raw) {
			return nil
		}
		// Avoid creating approvals for tool calls that will be rejected by command constraints anyway.
		if shouldSkipHighRiskApprovalBecauseCommandWouldBeDenied(ctx, dec, toolID, raw) {
			return nil
		}
		needsApproval = true
		autoApprove = strings.EqualFold(resolveCommandApprovalMode(ctx), "auto")
	default:
		// Unknown approval mode: be conservative and require approval.
		needsApproval = true
	}
	if !needsApproval {
		return nil
	}

	db := SettingsDBFromContext(ctx)
	if db == nil {
		return errors.New("settings db is required for approval gating")
	}
	scopeID := strings.TrimSpace(AttemptIDFromContext(ctx))
	if scopeID == "" {
		scopeID = strings.TrimSpace(SessionIDFromContext(ctx))
	}
	if scopeID == "" {
		return errors.New("attempt_id or session_id is required for approval gating")
	}
	snap, _ := PolicySnapshotFromContext(ctx)
	principalID := strings.TrimSpace(snap.PrincipalID)
	if principalID == "" {
		principalID = "local"
	}

	argsHash := toolArgsHash(raw)
	consumed, err := db.ConsumeApprovedToolApproval(ctx, principalID, scopeID, toolID, argsHash)
	if err != nil {
		return err
	}
	if consumed {
		return nil
	}

	if decision, ok, err := db.LookupLatestToolApprovalDecision(ctx, principalID, scopeID, toolID, argsHash); err != nil {
		return err
	} else if ok && strings.EqualFold(strings.TrimSpace(decision.Status), "denied") {
		return &ApprovalDeniedError{
			ApprovalID: decision.ID,
			ToolID:     toolID,
			ScopeID:    scopeID,
			Reason:     strings.TrimSpace(decision.Reason),
		}
	}

	approvalID, err := db.EnsurePendingToolApproval(ctx, principalID, scopeID, toolID, argsHash)
	if err != nil {
		return err
	}

	if autoApprove {
		if err := db.ApproveToolApprovalForPrincipal(ctx, principalID, approvalID, "auto-approved"); err != nil {
			return err
		}
		consumed, err := db.ConsumeApprovedToolApproval(ctx, principalID, scopeID, toolID, argsHash)
		if err != nil {
			return err
		}
		if consumed {
			return nil
		}
	}

	return &ApprovalRequiredError{ApprovalID: approvalID, ToolID: toolID, ScopeID: scopeID}
}

func toolArgsHash(raw json.RawMessage) string {
	normalized := []byte(raw)
	if len(raw) > 0 {
		var v any
		if err := json.Unmarshal(raw, &v); err == nil {
			if b, err := json.Marshal(v); err == nil {
				normalized = b
			}
		}
	}
	sum := sha256.Sum256(normalized)
	return hex.EncodeToString(sum[:])
}

func resolveCommandApprovalMode(ctx context.Context) string {
	db := SettingsDBFromContext(ctx)
	if db == nil {
		return "auto"
	}
	snap, _ := PolicySnapshotFromContext(ctx)
	principalID := strings.TrimSpace(snap.PrincipalID)
	if principalID == "" {
		principalID = "local"
	}
	val, err := db.GetUserSetting(ctx, principalID, settingsdb.SettingKeyCommandApprovalMode)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "auto"
		}
		return "auto"
	}
	mode := strings.ToLower(strings.TrimSpace(val))
	if mode == "manual" {
		return "manual"
	}
	return "auto"
}

func isHighRiskToolCallV1(toolID string, raw json.RawMessage) bool {
	switch toolID {
	case ToolIDBash:
		var req bashToolRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			return false
		}
		return isHighRiskCommand(req.Command)
	case ToolIDRunCommand:
		var req runCommandToolRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			return false
		}
		action := strings.ToLower(strings.TrimSpace(req.Action))
		command := strings.TrimSpace(req.Command)
		jobID := strings.TrimSpace(req.JobID)
		if action == "" {
			if command != "" {
				action = "start"
			} else if jobID != "" {
				action = "poll"
			}
		}
		if action != "start" {
			return false
		}
		return isHighRiskCommand(command)
	default:
		// Unknown tool: treat as high-risk to avoid accidental bypass.
		return true
	}
}

func shouldSkipHighRiskApprovalBecauseCommandWouldBeDenied(ctx context.Context, dec permissions.Decision, toolID string, raw json.RawMessage) bool {
	switch toolID {
	case ToolIDBash:
		var req bashToolRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			return true
		}
		return commandCallWouldBeDenied(ctx, dec, strings.TrimSpace(req.Command))
	case ToolIDRunCommand:
		var req runCommandToolRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			return true
		}
		action := strings.ToLower(strings.TrimSpace(req.Action))
		command := strings.TrimSpace(req.Command)
		jobID := strings.TrimSpace(req.JobID)
		if action == "" {
			if command != "" {
				action = "start"
			} else if jobID != "" {
				action = "poll"
			}
		}
		if action != "start" {
			return true
		}
		return commandCallWouldBeDenied(ctx, dec, command)
	default:
		return false
	}
}

func commandCallWouldBeDenied(ctx context.Context, dec permissions.Decision, command string) bool {
	if strings.TrimSpace(command) == "" {
		return true
	}
	mode, err := commandToolSandboxMode(dec)
	if err != nil {
		return true
	}
	profile := CommandProfile(dec, "dev")
	requestedProfile := strings.ToLower(strings.TrimSpace(profile))
	if requestedProfile == "coding" && !mode.IsHardBoundary() {
		return true
	}
	allowlist := dec.Constraints.Allowlist
	if mode == shell.SandboxModeNone {
		// Keep behavior consistent with command tools: without a hard boundary, they degrade to readonly.
		profile = "readonly"
		allowlist = nil
	}
	if permissions.ValidateCommand(profile, command, allowlist) != nil {
		return true
	}
	root, err := resolveWorkspaceRoot(ctx)
	if err != nil {
		return true
	}
	if shell.GuardCommand(command, root) != nil {
		return true
	}
	return false
}
