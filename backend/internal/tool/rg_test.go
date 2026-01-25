package tool

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/config"
)

func TestRgTool_FindsMatches_WithDollarRegex(t *testing.T) {
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("rg not installed")
	}

	root := t.TempDir()
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: root})

	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("abc\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	raw, _ := json.Marshal(map[string]any{
		"pattern": "abc$",
		"path":    ".",
	})

	gotAny, err := runRgTool(ctx, raw)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	got, ok := gotAny.(RgToolResult)
	if !ok {
		t.Fatalf("expected RgToolResult, got %T", gotAny)
	}
	if !got.Available {
		t.Fatalf("expected available=true")
	}
	if len(got.Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(got.Matches))
	}
	if got.Matches[0].Path != "a.txt" {
		t.Fatalf("expected path=%q, got %q", "a.txt", got.Matches[0].Path)
	}
	if got.Matches[0].LineNumber != 1 {
		t.Fatalf("expected line_number=1, got %d", got.Matches[0].LineNumber)
	}
	if got.Matches[0].Lines != "abc" {
		t.Fatalf("expected lines=%q, got %q", "abc", got.Matches[0].Lines)
	}
}

func TestRgTool_RejectsOutsideRootPath(t *testing.T) {
	root := t.TempDir()
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = prevCfg })
	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: root})

	raw, _ := json.Marshal(map[string]any{
		"pattern": "abc",
		"path":    "..",
	})

	if _, err := runRgTool(ctx, raw); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRgTool_TruncatesWhenMaxResultsReached(t *testing.T) {
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("rg not installed")
	}

	root := t.TempDir()
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: root})

	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("x\nx\nx\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	raw, _ := json.Marshal(map[string]any{
		"pattern":     "x",
		"path":        ".",
		"max_results": 2,
	})

	gotAny, err := runRgTool(ctx, raw)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	got, ok := gotAny.(RgToolResult)
	if !ok {
		t.Fatalf("expected RgToolResult, got %T", gotAny)
	}
	if !got.Available {
		t.Fatalf("expected available=true")
	}
	if len(got.Matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(got.Matches))
	}
	if !got.Truncated {
		t.Fatalf("expected truncated=true")
	}
	if got.TruncatedReason != "max_results" {
		t.Fatalf("expected truncated_reason=%q, got %q", "max_results", got.TruncatedReason)
	}
}

func TestRgTool_FallsBackToGrepWhenRgMissing(t *testing.T) {
	root := t.TempDir()
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = prevCfg })
	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: root})

	grepPath, err := exec.LookPath("grep")
	if err != nil {
		t.Skip("grep not installed")
	}
	toolDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink(grepPath, filepath.Join(toolDir, "grep")); err != nil {
		t.Fatalf("symlink grep: %v", err)
	}
	t.Setenv("PATH", toolDir)

	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("abc\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	raw, _ := json.Marshal(map[string]any{
		"pattern":       "abc",
		"path":          ".",
		"fixed_strings": true,
	})

	gotAny, err := runRgTool(ctx, raw)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	got, ok := gotAny.(RgToolResult)
	if !ok {
		t.Fatalf("expected RgToolResult, got %T", gotAny)
	}
	if !got.Available {
		t.Fatalf("expected available=true")
	}
	if got.Backend != "grep" {
		t.Fatalf("expected backend=%q, got %q", "grep", got.Backend)
	}
	if got.NotAvailableReason == "" {
		t.Fatalf("expected not_available_reason (rg missing)")
	}
	if len(got.Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(got.Matches))
	}
}
