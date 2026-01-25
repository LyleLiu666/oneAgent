package settingsdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type Provider struct {
	ID           string
	UserID       string
	Name         string
	ProviderType string
	BaseURL      string
	APIKey       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Model struct {
	ID            string
	ProviderID    string
	UserID        string
	Name          string
	Model         string
	IsDefault     bool
	EnableKVCache bool
	Options       map[string]any
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (d *DB) ListProviders(ctx context.Context, userID string) ([]Provider, error) {
	if d == nil || d.db == nil {
		return nil, errors.New("settings db is not initialized")
	}
	rows, err := d.db.QueryContext(ctx, `
SELECT id, user_id, name, provider_type, base_url, api_key, created_at_ms, updated_at_ms
FROM llm_providers
WHERE user_id = ?
ORDER BY updated_at_ms DESC;
`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Provider
	for rows.Next() {
		var p Provider
		var createdMs, updatedMs int64
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.ProviderType, &p.BaseURL, &p.APIKey, &createdMs, &updatedMs); err != nil {
			return nil, err
		}
		p.CreatedAt = timeFromMillis(createdMs)
		p.UpdatedAt = timeFromMillis(updatedMs)
		out = append(out, p)
	}
	return out, rows.Err()
}

func (d *DB) GetProvider(ctx context.Context, userID, providerID string) (Provider, error) {
	if d == nil || d.db == nil {
		return Provider{}, errors.New("settings db is not initialized")
	}
	var p Provider
	var createdMs, updatedMs int64
	err := d.db.QueryRowContext(ctx, `
SELECT id, user_id, name, provider_type, base_url, api_key, created_at_ms, updated_at_ms
FROM llm_providers
WHERE id = ? AND user_id = ?;
`, providerID, userID).Scan(&p.ID, &p.UserID, &p.Name, &p.ProviderType, &p.BaseURL, &p.APIKey, &createdMs, &updatedMs)
	if err != nil {
		return Provider{}, err
	}
	p.CreatedAt = timeFromMillis(createdMs)
	p.UpdatedAt = timeFromMillis(updatedMs)
	return p, nil
}

func (d *DB) CreateProvider(ctx context.Context, p Provider) (Provider, error) {
	if d == nil || d.db == nil {
		return Provider{}, errors.New("settings db is not initialized")
	}
	if p.ID == "" || p.UserID == "" {
		return Provider{}, errors.New("provider id/user_id is required")
	}
	now := nowMillis()
	_, err := d.db.ExecContext(ctx, `
INSERT INTO llm_providers (id, user_id, name, provider_type, base_url, api_key, created_at_ms, updated_at_ms)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);
`, p.ID, p.UserID, p.Name, p.ProviderType, p.BaseURL, p.APIKey, now, now)
	if err != nil {
		return Provider{}, err
	}
	p.CreatedAt = timeFromMillis(now)
	p.UpdatedAt = timeFromMillis(now)
	return p, nil
}

func (d *DB) UpdateProvider(ctx context.Context, userID, providerID string, updates map[string]any) (Provider, error) {
	if d == nil || d.db == nil {
		return Provider{}, errors.New("settings db is not initialized")
	}
	if len(updates) == 0 {
		return d.GetProvider(ctx, userID, providerID)
	}

	// Read existing then write full update (simple and safe).
	p, err := d.GetProvider(ctx, userID, providerID)
	if err != nil {
		return Provider{}, err
	}

	if v, ok := updates["name"].(string); ok {
		p.Name = v
	}
	if v, ok := updates["provider_type"].(string); ok {
		p.ProviderType = v
	}
	if v, ok := updates["base_url"].(string); ok {
		p.BaseURL = v
	}
	if v, ok := updates["api_key"].(string); ok {
		p.APIKey = v
	}

	now := nowMillis()
	_, err = d.db.ExecContext(ctx, `
UPDATE llm_providers
SET name = ?, provider_type = ?, base_url = ?, api_key = ?, updated_at_ms = ?
WHERE id = ? AND user_id = ?;
`, p.Name, p.ProviderType, p.BaseURL, p.APIKey, now, providerID, userID)
	if err != nil {
		return Provider{}, err
	}
	p.UpdatedAt = timeFromMillis(now)
	return p, nil
}

func (d *DB) DeleteProvider(ctx context.Context, userID, providerID string) (bool, error) {
	if d == nil || d.db == nil {
		return false, errors.New("settings db is not initialized")
	}
	res, err := d.db.ExecContext(ctx, `DELETE FROM llm_providers WHERE id = ? AND user_id = ?;`, providerID, userID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (d *DB) ListModels(ctx context.Context, userID, providerID string) ([]Model, error) {
	if d == nil || d.db == nil {
		return nil, errors.New("settings db is not initialized")
	}

	query := `
SELECT id, provider_id, user_id, name, model, is_default, enable_kv_cache, options_json, created_at_ms, updated_at_ms
FROM llm_models
WHERE user_id = ?
`
	args := []any{userID}
	if providerID != "" {
		query += " AND provider_id = ?"
		args = append(args, providerID)
	}
	query += " ORDER BY updated_at_ms DESC;"

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Model
	for rows.Next() {
		var m Model
		var isDefaultInt, enableKVInt int
		var optionsJSON string
		var createdMs, updatedMs int64
		if err := rows.Scan(&m.ID, &m.ProviderID, &m.UserID, &m.Name, &m.Model, &isDefaultInt, &enableKVInt, &optionsJSON, &createdMs, &updatedMs); err != nil {
			return nil, err
		}
		m.IsDefault = isDefaultInt != 0
		m.EnableKVCache = enableKVInt != 0
		if optionsJSON != "" {
			_ = json.Unmarshal([]byte(optionsJSON), &m.Options)
		}
		m.CreatedAt = timeFromMillis(createdMs)
		m.UpdatedAt = timeFromMillis(updatedMs)
		out = append(out, m)
	}
	return out, rows.Err()
}

func (d *DB) GetModel(ctx context.Context, userID, modelID string) (Model, error) {
	if d == nil || d.db == nil {
		return Model{}, errors.New("settings db is not initialized")
	}
	var m Model
	var isDefaultInt, enableKVInt int
	var optionsJSON string
	var createdMs, updatedMs int64
	err := d.db.QueryRowContext(ctx, `
SELECT id, provider_id, user_id, name, model, is_default, enable_kv_cache, options_json, created_at_ms, updated_at_ms
FROM llm_models
WHERE id = ? AND user_id = ?;
`, modelID, userID).Scan(&m.ID, &m.ProviderID, &m.UserID, &m.Name, &m.Model, &isDefaultInt, &enableKVInt, &optionsJSON, &createdMs, &updatedMs)
	if err != nil {
		return Model{}, err
	}
	m.IsDefault = isDefaultInt != 0
	m.EnableKVCache = enableKVInt != 0
	if optionsJSON != "" {
		_ = json.Unmarshal([]byte(optionsJSON), &m.Options)
	}
	m.CreatedAt = timeFromMillis(createdMs)
	m.UpdatedAt = timeFromMillis(updatedMs)
	return m, nil
}

func (d *DB) CreateModel(ctx context.Context, m Model) (Model, error) {
	if d == nil || d.db == nil {
		return Model{}, errors.New("settings db is not initialized")
	}
	if m.ID == "" || m.ProviderID == "" || m.UserID == "" {
		return Model{}, errors.New("model id/provider_id/user_id is required")
	}
	if m.Options == nil {
		m.Options = map[string]any{}
	}
	optionsJSON, _ := json.Marshal(m.Options)

	now := nowMillis()
	_, err := d.db.ExecContext(ctx, `
INSERT INTO llm_models (id, provider_id, user_id, name, model, is_default, enable_kv_cache, options_json, created_at_ms, updated_at_ms)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
`, m.ID, m.ProviderID, m.UserID, m.Name, m.Model, boolToInt(m.IsDefault), boolToInt(m.EnableKVCache), string(optionsJSON), now, now)
	if err != nil {
		return Model{}, err
	}

	if m.IsDefault {
		if err := d.clearOtherDefaults(ctx, m.UserID, m.ProviderID, m.ID); err != nil {
			return Model{}, err
		}
	}

	m.CreatedAt = timeFromMillis(now)
	m.UpdatedAt = timeFromMillis(now)
	return m, nil
}

func (d *DB) UpdateModel(ctx context.Context, userID, modelID string, updates map[string]any) (Model, error) {
	if d == nil || d.db == nil {
		return Model{}, errors.New("settings db is not initialized")
	}
	if len(updates) == 0 {
		return d.GetModel(ctx, userID, modelID)
	}

	m, err := d.GetModel(ctx, userID, modelID)
	if err != nil {
		return Model{}, err
	}

	if v, ok := updates["name"].(string); ok {
		m.Name = v
	}
	if v, ok := updates["model"].(string); ok {
		m.Model = v
	}
	if v, ok := updates["is_default"].(bool); ok {
		m.IsDefault = v
	}
	if v, ok := updates["enable_kv_cache"].(bool); ok {
		m.EnableKVCache = v
	}
	if v, ok := updates["options"].(map[string]any); ok {
		m.Options = v
	}

	if m.Options == nil {
		m.Options = map[string]any{}
	}
	optionsJSON, _ := json.Marshal(m.Options)

	now := nowMillis()
	res, err := d.db.ExecContext(ctx, `
UPDATE llm_models
SET name = ?, model = ?, is_default = ?, enable_kv_cache = ?, options_json = ?, updated_at_ms = ?
WHERE id = ? AND user_id = ?;
`, m.Name, m.Model, boolToInt(m.IsDefault), boolToInt(m.EnableKVCache), string(optionsJSON), now, modelID, userID)
	if err != nil {
		return Model{}, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return Model{}, sql.ErrNoRows
	}

	if m.IsDefault {
		if err := d.clearOtherDefaults(ctx, m.UserID, m.ProviderID, m.ID); err != nil {
			return Model{}, err
		}
	}

	m.UpdatedAt = timeFromMillis(now)
	return m, nil
}

func (d *DB) DeleteModel(ctx context.Context, userID, modelID string) (bool, error) {
	if d == nil || d.db == nil {
		return false, errors.New("settings db is not initialized")
	}
	res, err := d.db.ExecContext(ctx, `DELETE FROM llm_models WHERE id = ? AND user_id = ?;`, modelID, userID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (d *DB) clearOtherDefaults(ctx context.Context, userID, providerID, modelID string) error {
	_, err := d.db.ExecContext(ctx, `
UPDATE llm_models
SET is_default = 0, updated_at_ms = ?
WHERE user_id = ? AND provider_id = ? AND id <> ? AND is_default = 1;
`, nowMillis(), userID, providerID, modelID)
	return err
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
