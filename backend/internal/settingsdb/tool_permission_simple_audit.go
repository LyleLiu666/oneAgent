package settingsdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/permissions"
)

type ToolPermissionSimpleAuditRecord struct {
	PrincipalID           string
	Source                string
	OldPolicy             permissions.Policy
	NewPolicy             permissions.Policy
	SelectedMode          string
	CommandApprovalMode   string
	SkipPolicyPersistence bool
	ChangedAt             time.Time
}

func (d *DB) ApplySimpleToolPermissionChange(ctx context.Context, record ToolPermissionSimpleAuditRecord) error {
	if d == nil || d.db == nil {
		return errors.New("settings db is not initialized")
	}

	record.PrincipalID = strings.TrimSpace(record.PrincipalID)
	record.Source = strings.TrimSpace(record.Source)
	record.SelectedMode = strings.TrimSpace(record.SelectedMode)
	record.CommandApprovalMode = strings.ToLower(strings.TrimSpace(record.CommandApprovalMode))

	if record.PrincipalID == "" {
		return errors.New("principal_id is required")
	}
	if record.Source == "" {
		return errors.New("source is required")
	}
	if record.SelectedMode == "" {
		return errors.New("selected_mode is required")
	}
	if record.CommandApprovalMode == "" {
		record.CommandApprovalMode = "auto"
	}
	if record.CommandApprovalMode != "auto" && record.CommandApprovalMode != "manual" {
		return errors.New("command_approval_mode must be auto|manual")
	}
	if record.ChangedAt.IsZero() {
		record.ChangedAt = time.Now()
	}

	oldPolicyJSON, err := json.Marshal(record.OldPolicy)
	if err != nil {
		return err
	}
	newPolicyJSON, err := json.Marshal(record.NewPolicy)
	if err != nil {
		return err
	}

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	if !record.SkipPolicyPersistence {
		if err := setUserSettingTx(ctx, tx, record.PrincipalID, SettingKeyToolPolicy, string(newPolicyJSON)); err != nil {
			return err
		}
	}
	if err := setUserSettingTx(ctx, tx, record.PrincipalID, SettingKeyCommandApprovalMode, record.CommandApprovalMode); err != nil {
		return err
	}

	changedAtMS := record.ChangedAt.UTC().UnixMilli()
	if _, err := tx.ExecContext(ctx, `
INSERT INTO tool_permission_simple_audit (
  principal_id,
  source,
  old_policy_json,
  new_policy_json,
  old_policy_hash,
  new_policy_hash,
  selected_mode,
  command_approval_mode,
  changed_at_ms
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);
`, record.PrincipalID, record.Source, string(oldPolicyJSON), string(newPolicyJSON), permissions.HashPolicy(record.OldPolicy), permissions.HashPolicy(record.NewPolicy), record.SelectedMode, record.CommandApprovalMode, changedAtMS); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	tx = nil
	return nil
}

func (d *DB) ListSimpleToolPermissionAudit(ctx context.Context, principalID string) ([]ToolPermissionSimpleAuditRecord, error) {
	if d == nil || d.db == nil {
		return nil, errors.New("settings db is not initialized")
	}
	principalID = strings.TrimSpace(principalID)
	if principalID == "" {
		return nil, errors.New("principal_id is required")
	}

	rows, err := d.db.QueryContext(ctx, `
SELECT
  principal_id,
  source,
  old_policy_json,
  new_policy_json,
  selected_mode,
  command_approval_mode,
  changed_at_ms
FROM tool_permission_simple_audit
WHERE principal_id = ?
ORDER BY changed_at_ms DESC, id DESC;
`, principalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ToolPermissionSimpleAuditRecord, 0)
	for rows.Next() {
		var (
			record        ToolPermissionSimpleAuditRecord
			oldPolicyJSON string
			newPolicyJSON string
			changedAtMS   int64
		)
		if err := rows.Scan(&record.PrincipalID, &record.Source, &oldPolicyJSON, &newPolicyJSON, &record.SelectedMode, &record.CommandApprovalMode, &changedAtMS); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(oldPolicyJSON), &record.OldPolicy); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(newPolicyJSON), &record.NewPolicy); err != nil {
			return nil, err
		}
		record.ChangedAt = timeFromMillis(changedAtMS)
		out = append(out, record)
	}
	return out, rows.Err()
}

func setUserSettingTx(ctx context.Context, tx *sql.Tx, userID, key, value string) error {
	userID = strings.TrimSpace(userID)
	key = strings.TrimSpace(key)
	if tx == nil {
		return errors.New("transaction is required")
	}
	if userID == "" || key == "" {
		return errors.New("userID and key are required")
	}

	now := nowMillis()
	_, err := tx.ExecContext(ctx, `
INSERT INTO user_settings (user_id, key, value, created_at_ms, updated_at_ms)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(user_id, key) DO UPDATE SET
  value = excluded.value,
  updated_at_ms = excluded.updated_at_ms;
`, userID, key, value, now, now)
	return err
}
