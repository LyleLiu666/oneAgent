package settingsdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type ToolApprovalDecision struct {
	ID     string
	Status string
	Reason string
}

func (d *DB) EnsurePendingToolApproval(ctx context.Context, principalID string, scopeID string, toolID string, argsHash string) (string, error) {
	if d == nil || d.db == nil {
		return "", errors.New("settings db is not initialized")
	}
	principalID = strings.TrimSpace(principalID)
	if principalID == "" {
		return "", errors.New("principal_id is required")
	}
	scopeID = strings.TrimSpace(scopeID)
	if scopeID == "" {
		return "", errors.New("scope_id is required")
	}
	toolID = strings.TrimSpace(toolID)
	if toolID == "" {
		return "", errors.New("tool_id is required")
	}
	argsHash = strings.TrimSpace(argsHash)
	if argsHash == "" {
		return "", errors.New("args_hash is required")
	}

	var existingID string
	err := d.db.QueryRowContext(ctx, `
SELECT id FROM tool_approvals
WHERE principal_id = ? AND scope_id = ? AND tool_id = ? AND args_hash = ? AND status = 'pending'
ORDER BY created_at_ms DESC
LIMIT 1;
`, principalID, scopeID, toolID, argsHash).Scan(&existingID)
	if err == nil {
		return existingID, nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}

	id, err := generateToken(16)
	if err != nil {
		return "", err
	}
	now := nowMillis()
	_, err = d.db.ExecContext(ctx, `
INSERT INTO tool_approvals (
  id,
  principal_id,
  scope_id,
  tool_id,
  args_hash,
  status,
  requested_at_ms,
  approved_at_ms,
  denied_at_ms,
  consumed_at_ms,
  decision_reason,
  created_at_ms,
  updated_at_ms
)
VALUES (?, ?, ?, ?, ?, 'pending', ?, NULL, NULL, NULL, '', ?, ?);
`, id, principalID, scopeID, toolID, argsHash, now, now, now)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (d *DB) LookupLatestToolApprovalDecision(ctx context.Context, principalID string, scopeID string, toolID string, argsHash string) (ToolApprovalDecision, bool, error) {
	if d == nil || d.db == nil {
		return ToolApprovalDecision{}, false, errors.New("settings db is not initialized")
	}
	principalID = strings.TrimSpace(principalID)
	if principalID == "" {
		return ToolApprovalDecision{}, false, errors.New("principal_id is required")
	}
	scopeID = strings.TrimSpace(scopeID)
	if scopeID == "" {
		return ToolApprovalDecision{}, false, errors.New("scope_id is required")
	}
	toolID = strings.TrimSpace(toolID)
	if toolID == "" {
		return ToolApprovalDecision{}, false, errors.New("tool_id is required")
	}
	argsHash = strings.TrimSpace(argsHash)
	if argsHash == "" {
		return ToolApprovalDecision{}, false, errors.New("args_hash is required")
	}

	var id, status, reason string
	err := d.db.QueryRowContext(ctx, `
SELECT id, status, decision_reason FROM tool_approvals
WHERE principal_id = ? AND scope_id = ? AND tool_id = ? AND args_hash = ?
ORDER BY created_at_ms DESC
LIMIT 1;
`, principalID, scopeID, toolID, argsHash).Scan(&id, &status, &reason)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ToolApprovalDecision{}, false, nil
		}
		return ToolApprovalDecision{}, false, err
	}
	return ToolApprovalDecision{ID: id, Status: status, Reason: reason}, true, nil
}

func (d *DB) ApproveToolApproval(ctx context.Context, approvalID string, reason string) error {
	if d == nil || d.db == nil {
		return errors.New("settings db is not initialized")
	}
	approvalID = strings.TrimSpace(approvalID)
	if approvalID == "" {
		return errors.New("approval_id is required")
	}
	reason = strings.TrimSpace(reason)
	now := nowMillis()
	res, err := d.db.ExecContext(ctx, `
UPDATE tool_approvals
SET status = 'approved', approved_at_ms = COALESCE(approved_at_ms, ?), decision_reason = ?, updated_at_ms = ?
WHERE id = ? AND status IN ('pending', 'approved');
`, now, reason, now, approvalID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		var status string
		scanErr := d.db.QueryRowContext(ctx, `SELECT status FROM tool_approvals WHERE id = ?;`, approvalID).Scan(&status)
		if scanErr != nil {
			return scanErr
		}
		return fmt.Errorf("cannot approve tool approval in status %s", status)
	}
	return nil
}

func (d *DB) ApproveToolApprovalForPrincipal(ctx context.Context, principalID string, approvalID string, reason string) error {
	if d == nil || d.db == nil {
		return errors.New("settings db is not initialized")
	}
	principalID = strings.TrimSpace(principalID)
	if principalID == "" {
		return errors.New("principal_id is required")
	}
	approvalID = strings.TrimSpace(approvalID)
	if approvalID == "" {
		return errors.New("approval_id is required")
	}
	reason = strings.TrimSpace(reason)
	now := nowMillis()
	res, err := d.db.ExecContext(ctx, `
UPDATE tool_approvals
SET status = 'approved', approved_at_ms = COALESCE(approved_at_ms, ?), decision_reason = ?, updated_at_ms = ?
WHERE id = ? AND principal_id = ? AND status IN ('pending', 'approved');
`, now, reason, now, approvalID, principalID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (d *DB) DenyToolApprovalForPrincipal(ctx context.Context, principalID string, approvalID string, reason string) error {
	if d == nil || d.db == nil {
		return errors.New("settings db is not initialized")
	}
	principalID = strings.TrimSpace(principalID)
	if principalID == "" {
		return errors.New("principal_id is required")
	}
	approvalID = strings.TrimSpace(approvalID)
	if approvalID == "" {
		return errors.New("approval_id is required")
	}
	reason = strings.TrimSpace(reason)
	now := nowMillis()
	res, err := d.db.ExecContext(ctx, `
UPDATE tool_approvals
SET status = 'denied', denied_at_ms = COALESCE(denied_at_ms, ?), decision_reason = ?, updated_at_ms = ?
WHERE id = ? AND principal_id = ? AND status IN ('pending', 'denied');
`, now, reason, now, approvalID, principalID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (d *DB) ConsumeApprovedToolApproval(ctx context.Context, principalID string, scopeID string, toolID string, argsHash string) (bool, error) {
	if d == nil || d.db == nil {
		return false, errors.New("settings db is not initialized")
	}
	principalID = strings.TrimSpace(principalID)
	if principalID == "" {
		return false, errors.New("principal_id is required")
	}
	scopeID = strings.TrimSpace(scopeID)
	if scopeID == "" {
		return false, errors.New("scope_id is required")
	}
	toolID = strings.TrimSpace(toolID)
	if toolID == "" {
		return false, errors.New("tool_id is required")
	}
	argsHash = strings.TrimSpace(argsHash)
	if argsHash == "" {
		return false, errors.New("args_hash is required")
	}

	now := nowMillis()
	res, err := d.db.ExecContext(ctx, `
UPDATE tool_approvals
SET status = 'consumed', consumed_at_ms = ?, updated_at_ms = ?
WHERE principal_id = ? AND scope_id = ? AND tool_id = ? AND args_hash = ? AND status = 'approved';
`, now, now, principalID, scopeID, toolID, argsHash)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}
