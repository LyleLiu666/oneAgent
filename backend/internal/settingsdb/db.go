package settingsdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	path string
	db   *sql.DB
}

func Open(path string) (*DB, error) {
	if path == "" {
		return nil, errors.New("settings db path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create settings db dir: %w", err)
	}

	// modernc.org/sqlite uses driver name "sqlite".
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Basic pragmas for durability and concurrency.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := conn.ExecContext(ctx, "PRAGMA journal_mode=WAL;"); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("set journal_mode: %w", err)
	}
	if _, err := conn.ExecContext(ctx, "PRAGMA synchronous=NORMAL;"); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("set synchronous: %w", err)
	}
	if _, err := conn.ExecContext(ctx, "PRAGMA busy_timeout=5000;"); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("set busy_timeout: %w", err)
	}
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys=ON;"); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("set foreign_keys: %w", err)
	}

	db := &DB{path: path, db: conn}
	if err := db.Migrate(); err != nil {
		_ = conn.Close()
		return nil, err
	}

	return db, nil
}

func (d *DB) Close() error {
	if d == nil || d.db == nil {
		return nil
	}
	return d.db.Close()
}

func (d *DB) Path() string {
	if d == nil {
		return ""
	}
	return d.path
}

func (d *DB) HealthCheck(ctx context.Context) error {
	if d == nil || d.db == nil {
		return errors.New("settings db is not initialized")
	}
	return d.db.PingContext(ctx)
}

func (d *DB) Migrate() error {
	if d == nil || d.db == nil {
		return errors.New("settings db is not initialized")
	}

	stmts := []string{
		`
CREATE TABLE IF NOT EXISTS llm_providers (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  name TEXT NOT NULL,
  provider_type TEXT NOT NULL,
  base_url TEXT NOT NULL,
  api_key TEXT NOT NULL,
  created_at_ms INTEGER NOT NULL,
  updated_at_ms INTEGER NOT NULL
);
`,
		`CREATE INDEX IF NOT EXISTS idx_llm_providers_user_id ON llm_providers(user_id);`,
		`
CREATE TABLE IF NOT EXISTS llm_models (
  id TEXT PRIMARY KEY,
  provider_id TEXT NOT NULL,
  user_id TEXT NOT NULL,
  name TEXT NOT NULL,
  model TEXT NOT NULL,
  is_default INTEGER NOT NULL,
  enable_kv_cache INTEGER NOT NULL,
  options_json TEXT NOT NULL,
  created_at_ms INTEGER NOT NULL,
  updated_at_ms INTEGER NOT NULL,
  FOREIGN KEY(provider_id) REFERENCES llm_providers(id) ON DELETE CASCADE
);
`,
		`CREATE INDEX IF NOT EXISTS idx_llm_models_user_id ON llm_models(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_llm_models_provider_id ON llm_models(provider_id);`,
		`
CREATE TABLE IF NOT EXISTS user_settings (
  user_id TEXT NOT NULL,
  key TEXT NOT NULL,
  value TEXT NOT NULL,
  created_at_ms INTEGER NOT NULL,
  updated_at_ms INTEGER NOT NULL,
  PRIMARY KEY(user_id, key)
);
`,
		`
CREATE TABLE IF NOT EXISTS auth_tokens (
  token TEXT PRIMARY KEY,
  principal_id TEXT NOT NULL,
  created_at_ms INTEGER NOT NULL,
  revoked_at_ms INTEGER
);
`,
		`CREATE INDEX IF NOT EXISTS idx_auth_tokens_principal_id ON auth_tokens(principal_id);`,
		`
CREATE TABLE IF NOT EXISTS tool_approvals (
  id TEXT PRIMARY KEY,
  principal_id TEXT NOT NULL,
  scope_id TEXT NOT NULL,
  tool_id TEXT NOT NULL,
  args_hash TEXT NOT NULL,
  status TEXT NOT NULL,
  requested_at_ms INTEGER NOT NULL,
  approved_at_ms INTEGER,
  denied_at_ms INTEGER,
  consumed_at_ms INTEGER,
  decision_reason TEXT NOT NULL,
  created_at_ms INTEGER NOT NULL,
  updated_at_ms INTEGER NOT NULL
);
`,
		`CREATE INDEX IF NOT EXISTS idx_tool_approvals_lookup ON tool_approvals(principal_id, scope_id, tool_id, args_hash, status);`,
		`
CREATE TABLE IF NOT EXISTS skill_usage (
  user_id TEXT NOT NULL,
  skill_id TEXT NOT NULL,
  used_count INTEGER NOT NULL,
  last_used_at_ms INTEGER NOT NULL,
  last_used_reason TEXT NOT NULL,
  created_at_ms INTEGER NOT NULL,
  updated_at_ms INTEGER NOT NULL,
  PRIMARY KEY(user_id, skill_id)
);
`,
		`CREATE INDEX IF NOT EXISTS idx_skill_usage_user_id ON skill_usage(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_skill_usage_last_used_at_ms ON skill_usage(last_used_at_ms);`,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migrate settings db: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("migrate settings db commit: %w", err)
	}
	return nil
}

func nowMillis() int64 {
	return time.Now().UnixMilli()
}

func timeFromMillis(ms int64) time.Time {
	if ms <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms)
}
