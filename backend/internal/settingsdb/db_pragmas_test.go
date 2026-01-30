package settingsdb

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpen_ConfiguresPragmasForConcurrency(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	var journalMode string
	if err := db.db.QueryRowContext(ctx, "PRAGMA journal_mode;").Scan(&journalMode); err != nil {
		t.Fatalf("journal_mode: %v", err)
	}
	if strings.ToLower(strings.TrimSpace(journalMode)) != "wal" {
		t.Fatalf("expected journal_mode=wal, got %q", journalMode)
	}

	var busyTimeout int
	if err := db.db.QueryRowContext(ctx, "PRAGMA busy_timeout;").Scan(&busyTimeout); err != nil {
		t.Fatalf("busy_timeout: %v", err)
	}
	if busyTimeout < 1000 {
		t.Fatalf("expected busy_timeout to be set (>=1000ms), got %d", busyTimeout)
	}

	var foreignKeys int
	if err := db.db.QueryRowContext(ctx, "PRAGMA foreign_keys;").Scan(&foreignKeys); err != nil {
		t.Fatalf("foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Fatalf("expected foreign_keys=1, got %d", foreignKeys)
	}
}
