package settingsdb

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestSkillUsage_RecordAndGet(t *testing.T) {
	ctx := context.Background()
	db, err := Open(filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	userID := "alice"
	skillID := "demo-skill"

	if err := db.RecordSkillUse(ctx, userID, skillID, ""); err != nil {
		t.Fatalf("record: %v", err)
	}

	got1, ok, err := db.GetSkillUsage(ctx, userID, skillID)
	if err != nil {
		t.Fatalf("get1: %v", err)
	}
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if got1.UsedCount != 1 {
		t.Fatalf("expected used_count=1, got %d", got1.UsedCount)
	}
	if got1.LastUsedAt.IsZero() {
		t.Fatalf("expected last_used_at to be set")
	}

	time.Sleep(2 * time.Millisecond)

	if err := db.RecordSkillUse(ctx, userID, skillID, ""); err != nil {
		t.Fatalf("record2: %v", err)
	}

	got2, ok, err := db.GetSkillUsage(ctx, userID, skillID)
	if err != nil {
		t.Fatalf("get2: %v", err)
	}
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if got2.UsedCount != 2 {
		t.Fatalf("expected used_count=2, got %d", got2.UsedCount)
	}
	if !got2.LastUsedAt.After(got1.LastUsedAt) && !got2.LastUsedAt.Equal(got1.LastUsedAt) {
		t.Fatalf("expected last_used_at to not move backwards: %v -> %v", got1.LastUsedAt, got2.LastUsedAt)
	}
}

