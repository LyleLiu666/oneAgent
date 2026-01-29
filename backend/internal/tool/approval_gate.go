package tool

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
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
	approval := strings.TrimSpace(dec.Constraints.Approval)
	if !strings.EqualFold(approval, "required") {
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
	return &ApprovalRequiredError{
		ApprovalID: approvalID,
		ToolID:     toolID,
		ScopeID:    scopeID,
	}
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
