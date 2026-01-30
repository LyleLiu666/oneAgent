package settingsdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type SkillUsage struct {
	UserID         string
	SkillID        string
	UsedCount      int64
	LastUsedAt     time.Time
	LastUsedReason string
}

func (d *DB) RecordSkillUse(ctx context.Context, userID string, skillID string, reason string) error {
	if d == nil || d.db == nil {
		return errors.New("settings db is not initialized")
	}
	userID = strings.TrimSpace(userID)
	skillID = strings.TrimSpace(skillID)
	reason = strings.TrimSpace(reason)
	if userID == "" {
		return errors.New("user_id is required")
	}
	if skillID == "" {
		return errors.New("skill_id is required")
	}

	now := nowMillis()
	if reason == "" {
		reason = "unknown"
	}

	_, err := d.db.ExecContext(ctx, `
INSERT INTO skill_usage (
  user_id,
  skill_id,
  used_count,
  last_used_at_ms,
  last_used_reason,
  created_at_ms,
  updated_at_ms
) VALUES (?, ?, 1, ?, ?, ?, ?)
ON CONFLICT(user_id, skill_id) DO UPDATE SET
  used_count = used_count + 1,
  last_used_at_ms = excluded.last_used_at_ms,
  last_used_reason = excluded.last_used_reason,
  updated_at_ms = excluded.updated_at_ms;
`, userID, skillID, now, reason, now, now)
	if err != nil {
		return fmt.Errorf("record skill use: %w", err)
	}
	return nil
}

func (d *DB) GetSkillUsage(ctx context.Context, userID string, skillID string) (SkillUsage, bool, error) {
	if d == nil || d.db == nil {
		return SkillUsage{}, false, errors.New("settings db is not initialized")
	}
	userID = strings.TrimSpace(userID)
	skillID = strings.TrimSpace(skillID)
	if userID == "" {
		return SkillUsage{}, false, errors.New("user_id is required")
	}
	if skillID == "" {
		return SkillUsage{}, false, errors.New("skill_id is required")
	}

	var usedCount int64
	var lastUsedAtMs int64
	var lastUsedReason string
	err := d.db.QueryRowContext(ctx, `
SELECT used_count, last_used_at_ms, last_used_reason
FROM skill_usage
WHERE user_id = ? AND skill_id = ?;
`, userID, skillID).Scan(&usedCount, &lastUsedAtMs, &lastUsedReason)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SkillUsage{}, false, nil
		}
		return SkillUsage{}, false, fmt.Errorf("get skill usage: %w", err)
	}

	return SkillUsage{
		UserID:         userID,
		SkillID:        skillID,
		UsedCount:      usedCount,
		LastUsedAt:     timeFromMillis(lastUsedAtMs),
		LastUsedReason: lastUsedReason,
	}, true, nil
}
