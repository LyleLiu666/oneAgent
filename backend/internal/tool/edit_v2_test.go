package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/config"
)

func TestEditV2_AmbiguityFails(t *testing.T) {
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	root := t.TempDir()
	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: root})

	path := filepath.Join(root, "a.txt")
	if err := os.WriteFile(path, []byte("x oldcontent y\nx oldcontent z\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	raw, _ := json.Marshal(map[string]any{
		"filePath":  "a.txt",
		"oldString": "oldcontent",
		"newString": "NEW",
	})
	gotAny, err := runEditV2Tool(ctx, raw)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	got := gotAny.(editV2Result)
	if got.OK {
		t.Fatalf("expected ok=false")
	}
	if !strings.Contains(strings.ToLower(got.Error), "ambiguous") {
		t.Fatalf("expected ambiguous error, got: %q", got.Error)
	}
}

func TestEditV2_OccurrenceSelectsSecond(t *testing.T) {
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	root := t.TempDir()
	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: root})

	path := filepath.Join(root, "a.txt")
	if err := os.WriteFile(path, []byte("oldcontent\noldcontent\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	raw, _ := json.Marshal(map[string]any{
		"filePath":               "a.txt",
		"oldString":              "oldcontent",
		"newString":              "NEW",
		"occurrence":             2,
		"expected_replacements":  1,
	})
	gotAny, err := runEditV2Tool(ctx, raw)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	got := gotAny.(editV2Result)
	if !got.OK || got.Replacements != 1 {
		t.Fatalf("expected ok=true replacements=1, got %+v", got)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "oldcontent\nNEW\n" {
		t.Fatalf("unexpected content: %q", string(data))
	}
}

func TestEditV2_AnchorsDisambiguate(t *testing.T) {
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	root := t.TempDir()
	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: root})

	path := filepath.Join(root, "a.txt")
	content := "A\nbefore\noldcontent\nafter\nB\noldcontent\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	raw, _ := json.Marshal(map[string]any{
		"filePath":       "a.txt",
		"oldString":      "oldcontent",
		"newString":      "NEW",
		"before_anchor":  "before\n",
		"after_anchor":   "after\n",
		"expected_replacements": 1,
	})
	gotAny, err := runEditV2Tool(ctx, raw)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	got := gotAny.(editV2Result)
	if !got.OK {
		t.Fatalf("expected ok=true, got %+v", got)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "before\nNEW\nafter\n") {
		t.Fatalf("expected first occurrence replaced, got: %q", string(data))
	}
	if !strings.Contains(string(data), "B\noldcontent\n") {
		t.Fatalf("expected second occurrence untouched, got: %q", string(data))
	}
}

func TestEditV2_PreservesCRLF(t *testing.T) {
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	root := t.TempDir()
	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: root})

	path := filepath.Join(root, "a.txt")
	if err := os.WriteFile(path, []byte("a\r\nold\r\nb\r\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	raw, _ := json.Marshal(map[string]any{
		"filePath":              "a.txt",
		"oldString":             "old\n",
		"newString":             "NEW\n",
		"expected_replacements": 1,
	})
	gotAny, err := runEditV2Tool(ctx, raw)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	got := gotAny.(editV2Result)
	if !got.OK {
		t.Fatalf("expected ok=true, got %+v", got)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "a\r\nNEW\r\nb\r\n" {
		t.Fatalf("unexpected content: %q", string(data))
	}
}

func TestEditV2_DiffPreviewCapped(t *testing.T) {
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	root := t.TempDir()
	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: root})

	path := filepath.Join(root, "a.txt")
	if err := os.WriteFile(path, []byte("start\nold\nend\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	huge := strings.Repeat("X", 10000)
	raw, _ := json.Marshal(map[string]any{
		"filePath":              "a.txt",
		"oldString":             "old\n",
		"newString":             huge + "\n",
		"expected_replacements": 1,
	})
	gotAny, err := runEditV2Tool(ctx, raw)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	got := gotAny.(editV2Result)
	if !got.OK {
		t.Fatalf("expected ok=true, got %+v", got)
	}
	if !got.DiffCapped || !strings.Contains(got.DiffPreview, "truncated") {
		t.Fatalf("expected diff_preview capped, got capped=%v len=%d", got.DiffCapped, len(got.DiffPreview))
	}
}

func TestEditV2_PreconditionMismatchReturnsFlag(t *testing.T) {
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	root := t.TempDir()
	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: root})

	path := filepath.Join(root, "a.txt")
	if err := os.WriteFile(path, []byte("old\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	sha, err := fileSHA256Hex(path)
	if err != nil {
		t.Fatalf("sha: %v", err)
	}
	// Change file after "read".
	if err := os.WriteFile(path, []byte("changed\n"), 0o644); err != nil {
		t.Fatalf("write2: %v", err)
	}

	raw, _ := json.Marshal(map[string]any{
		"filePath":  "a.txt",
		"oldString": "old\n",
		"newString": "NEW\n",
		"preconditions": map[string]any{
			"expected_sha256": sha,
		},
	})
	gotAny, err := runEditV2Tool(ctx, raw)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	got := gotAny.(editV2Result)
	if got.OK || !got.PreconditionFailed {
		t.Fatalf("expected precondition_failed, got %+v", got)
	}
}
