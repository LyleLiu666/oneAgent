package memorydb

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
		return nil, errors.New("memory db path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create memory db dir: %w", err)
	}

	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

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
		return errors.New("memory db is not initialized")
	}
	return d.db.PingContext(ctx)
}

func (d *DB) Migrate() error {
	if d == nil || d.db == nil {
		return errors.New("memory db is not initialized")
	}

	stmts := []string{
		`
CREATE TABLE IF NOT EXISTS memory_entries (
  seq INTEGER PRIMARY KEY AUTOINCREMENT,
  id TEXT NOT NULL UNIQUE,
  principal_id TEXT NOT NULL,
  created_at_ms INTEGER NOT NULL,
  writer TEXT NOT NULL,
  type TEXT NOT NULL,
  workspace TEXT,
  title TEXT,
  content TEXT,
  links_json TEXT,
  tags_json TEXT
);
`,
		`CREATE INDEX IF NOT EXISTS idx_memory_entries_principal_created_at ON memory_entries(principal_id, created_at_ms);`,
		`CREATE INDEX IF NOT EXISTS idx_memory_entries_principal_writer_seq ON memory_entries(principal_id, writer, seq);`,
		`
CREATE TABLE IF NOT EXISTS memory_cursors (
  principal_id TEXT NOT NULL,
  channel TEXT NOT NULL,
  peer_writer TEXT NOT NULL,
  last_synced_seq INTEGER NOT NULL,
  PRIMARY KEY(principal_id, channel, peer_writer)
);
`,
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
			return fmt.Errorf("migrate memory db: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("migrate memory db commit: %w", err)
	}
	return nil
}

