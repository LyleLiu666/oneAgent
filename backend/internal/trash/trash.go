package trash

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/fsutil"
)

const (
	DefaultRetention       = 7 * 24 * time.Hour
	DefaultCleanupInterval = 24 * time.Hour
)

type Manifest struct {
	TrashID      string `json:"trash_id"`
	CreatedAtUTC string `json:"created_at_utc"`

	OriginalPath string `json:"original_path"`
	TrashedPath  string `json:"trashed_path"`

	IsDir bool `json:"is_dir"`
}

func TrashRoot(workspaceRoot string) string {
	return filepath.Join(strings.TrimSpace(workspaceRoot), ".oneagent", "trash")
}

func EntryDir(workspaceRoot, trashID string) string {
	return filepath.Join(TrashRoot(workspaceRoot), strings.TrimSpace(trashID))
}

func ManifestPath(workspaceRoot, trashID string) string {
	return filepath.Join(EntryDir(workspaceRoot, trashID), "manifest.json")
}

func PayloadPath(workspaceRoot, trashID string) string {
	return filepath.Join(EntryDir(workspaceRoot, trashID), "payload")
}

type CleanupResult struct {
	OK bool `json:"ok"`

	TrashRoot string `json:"trash_root"`
	Retention string `json:"retention"`

	DeletedEntries int `json:"deleted_entries"`
	ScannedEntries int `json:"scanned_entries"`
}

func Cleanup(ctx context.Context, workspaceRoot string, retention time.Duration, now time.Time) (CleanupResult, error) {
	_ = ctx
	root := strings.TrimSpace(workspaceRoot)
	if root == "" {
		return CleanupResult{}, errors.New("workspace root is required")
	}
	if retention <= 0 {
		retention = DefaultRetention
	}
	if now.IsZero() {
		now = time.Now()
	}

	trashRoot := TrashRoot(root)
	entries, err := os.ReadDir(trashRoot)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return CleanupResult{
				OK:             true,
				TrashRoot:      trashRoot,
				Retention:      retention.String(),
				ScannedEntries: 0,
				DeletedEntries: 0,
			}, nil
		}
		return CleanupResult{}, err
	}

	cutoff := now.Add(-retention)
	scanned := 0
	deleted := 0

	for _, ent := range entries {
		if strings.HasPrefix(ent.Name(), ".") {
			continue
		}
		if !ent.IsDir() {
			continue
		}
		select {
		case <-ctx.Done():
			return CleanupResult{}, ctx.Err()
		default:
		}

		scanned++
		entryDir := filepath.Join(trashRoot, ent.Name())

		createdAt := time.Time{}
		manifestPath := filepath.Join(entryDir, "manifest.json")
		if raw, err := os.ReadFile(manifestPath); err == nil {
			var m Manifest
			if jsonErr := json.Unmarshal(raw, &m); jsonErr == nil {
				if ts := strings.TrimSpace(m.CreatedAtUTC); ts != "" {
					if parsed, parseErr := time.Parse(time.RFC3339Nano, ts); parseErr == nil {
						createdAt = parsed
					}
				}
			}
		}
		if createdAt.IsZero() {
			if st, err := os.Stat(entryDir); err == nil {
				createdAt = st.ModTime()
			}
		}

		if !createdAt.IsZero() && createdAt.After(cutoff) {
			continue
		}

		if err := os.RemoveAll(entryDir); err != nil {
			continue
		}
		deleted++
	}

	return CleanupResult{
		OK:             true,
		TrashRoot:      trashRoot,
		Retention:      retention.String(),
		ScannedEntries: scanned,
		DeletedEntries: deleted,
	}, nil
}

func RunCleanupLoop(ctx context.Context, workspaceRoot string, retention time.Duration, interval time.Duration) {
	root := strings.TrimSpace(workspaceRoot)
	if root == "" {
		return
	}
	if retention <= 0 {
		retention = DefaultRetention
	}
	if interval <= 0 {
		interval = DefaultCleanupInterval
	}

	// Best-effort: ensure the trash root exists so the loop work is bounded and predictable.
	_ = os.MkdirAll(TrashRoot(root), 0o700)

	_, _ = Cleanup(ctx, root, retention, time.Now())

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = Cleanup(ctx, root, retention, time.Now())
		}
	}
}

func WriteManifest(workspaceRoot string, trashID string, m Manifest) (string, error) {
	path := ManifestPath(workspaceRoot, trashID)
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", err
	}
	if err := fsutil.AtomicWriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}
