package settingsdb

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

const (
	SettingKeyBochaAPIKey = "bocha_api_key"
	SettingKeyCommandApprovalMode = "command_approval_mode" // auto|manual
)

func (d *DB) GetUserSetting(ctx context.Context, userID, key string) (string, error) {
	if d == nil || d.db == nil {
		return "", errors.New("settings db is not initialized")
	}
	var value string
	err := d.db.QueryRowContext(ctx, `SELECT value FROM user_settings WHERE user_id = ? AND key = ?;`, userID, key).Scan(&value)
	if err != nil {
		return "", err
	}
	return value, nil
}

func (d *DB) SetUserSetting(ctx context.Context, userID, key, value string) error {
	if d == nil || d.db == nil {
		return errors.New("settings db is not initialized")
	}
	userID = strings.TrimSpace(userID)
	key = strings.TrimSpace(key)
	if userID == "" || key == "" {
		return errors.New("userID and key are required")
	}

	now := nowMillis()

	// Upsert.
	_, err := d.db.ExecContext(ctx, `
INSERT INTO user_settings (user_id, key, value, created_at_ms, updated_at_ms)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(user_id, key) DO UPDATE SET
  value = excluded.value,
  updated_at_ms = excluded.updated_at_ms;
`, userID, key, value, now, now)
	return err
}

// GetUserSettingsMap returns a map of settings for the given user.
// Sensitive values are masked as has_* flags.
func (d *DB) GetUserSettingsMap(ctx context.Context, userID string) (map[string]any, error) {
	if d == nil || d.db == nil {
		return nil, errors.New("settings db is not initialized")
	}
	rows, err := d.db.QueryContext(ctx, `SELECT key, value FROM user_settings WHERE user_id = ?;`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]any)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		if key == SettingKeyBochaAPIKey {
			out["has_bocha_api_key"] = strings.TrimSpace(value) != ""
		} else {
			out[key] = value
		}
	}
	return out, rows.Err()
}

func (d *DB) TouchHealthCheck(ctx context.Context) error {
	if d == nil || d.db == nil {
		return errors.New("settings db is not initialized")
	}
	// Ensure we can write.
	_, err := d.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS _healthcheck (t INTEGER NOT NULL);`)
	if err != nil {
		return err
	}
	_, err = d.db.ExecContext(ctx, `INSERT INTO _healthcheck (t) VALUES (?);`, time.Now().UnixMilli())
	return err
}

var _ = sql.ErrNoRows
