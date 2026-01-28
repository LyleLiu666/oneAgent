package settingsdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/permissions"
)

const (
	SettingKeyToolPolicy = "tool_policy_json"
)

func (d *DB) GetToolPolicy(ctx context.Context, principalID string) (permissions.Policy, bool, error) {
	if d == nil || d.db == nil {
		return permissions.Policy{}, false, errors.New("settings db is not initialized")
	}
	principalID = strings.TrimSpace(principalID)
	if principalID == "" {
		return permissions.Policy{}, false, errors.New("principal_id is required")
	}
	val, err := d.GetUserSetting(ctx, principalID, SettingKeyToolPolicy)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return permissions.Policy{}, false, nil
		}
		return permissions.Policy{}, false, err
	}
	var policy permissions.Policy
	if err := json.Unmarshal([]byte(val), &policy); err != nil {
		return permissions.Policy{}, false, err
	}
	return policy, true, nil
}

func (d *DB) SetToolPolicy(ctx context.Context, principalID string, policy permissions.Policy) error {
	if d == nil || d.db == nil {
		return errors.New("settings db is not initialized")
	}
	principalID = strings.TrimSpace(principalID)
	if principalID == "" {
		return errors.New("principal_id is required")
	}
	data, err := json.Marshal(policy)
	if err != nil {
		return err
	}
	return d.SetUserSetting(ctx, principalID, SettingKeyToolPolicy, string(data))
}
