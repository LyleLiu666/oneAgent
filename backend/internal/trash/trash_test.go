package trash

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCleanup_DeletesEntriesOlderThanRetention(t *testing.T) {
	root := t.TempDir()

	oldID := "old"
	newID := "new"

	if err := os.MkdirAll(PayloadPath(root, oldID), 0o700); err != nil {
		t.Fatalf("mkdir old payload dir: %v", err)
	}
	if err := os.MkdirAll(PayloadPath(root, newID), 0o700); err != nil {
		t.Fatalf("mkdir new payload dir: %v", err)
	}

	oldManifest := Manifest{
		TrashID:      oldID,
		CreatedAtUTC: time.Now().Add(-8 * 24 * time.Hour).UTC().Format(time.RFC3339Nano),
		OriginalPath: filepath.Join(root, "a.txt"),
		TrashedPath:  PayloadPath(root, oldID),
	}
	newManifest := Manifest{
		TrashID:      newID,
		CreatedAtUTC: time.Now().Add(-1 * 24 * time.Hour).UTC().Format(time.RFC3339Nano),
		OriginalPath: filepath.Join(root, "b.txt"),
		TrashedPath:  PayloadPath(root, newID),
	}

	{
		data, _ := json.Marshal(oldManifest)
		if err := os.WriteFile(ManifestPath(root, oldID), data, 0o644); err != nil {
			t.Fatalf("write old manifest: %v", err)
		}
	}
	{
		data, _ := json.Marshal(newManifest)
		if err := os.WriteFile(ManifestPath(root, newID), data, 0o644); err != nil {
			t.Fatalf("write new manifest: %v", err)
		}
	}

	now := time.Now()
	res, err := Cleanup(context.Background(), root, 7*24*time.Hour, now)
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if !res.OK {
		t.Fatalf("expected ok=true, got %+v", res)
	}
	if _, err := os.Stat(EntryDir(root, oldID)); !os.IsNotExist(err) {
		t.Fatalf("expected old entry to be deleted, stat err=%v", err)
	}
	if _, err := os.Stat(EntryDir(root, newID)); err != nil {
		t.Fatalf("expected new entry to remain, stat err=%v", err)
	}
}
