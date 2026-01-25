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

func TestWriteFileTool_WritesFile(t *testing.T) {
	root := t.TempDir()
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: root})
	resolvedRoot, err := resolveWorkspaceRoot(ctx)
	if err != nil {
		t.Fatalf("resolve workspace root: %v", err)
	}

	raw, _ := json.Marshal(map[string]any{
		"filePath": "dir/file.txt",
		"content":  "hello\nworld\n",
	})

	gotAny, err := runWriteFileTool(ctx, raw)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	got, ok := gotAny.(writeFileResult)
	if !ok {
		t.Fatalf("expected writeFileResult, got %T", gotAny)
	}
	if !got.OK {
		t.Fatalf("expected ok=true, got %+v", got)
	}
	if got.Mode != "overwrite" {
		t.Fatalf("expected mode=%q, got %q", "overwrite", got.Mode)
	}
	if got.WrittenBytes != len("hello\nworld\n") {
		t.Fatalf("expected written_bytes=%d, got %d", len("hello\nworld\n"), got.WrittenBytes)
	}
	if got.WrittenLines != 2 {
		t.Fatalf("expected written_lines=%d, got %d", 2, got.WrittenLines)
	}
	if got.FilePath != filepath.Join(resolvedRoot, "dir", "file.txt") {
		t.Fatalf("expected file_path=%q, got %q", filepath.Join(resolvedRoot, "dir", "file.txt"), got.FilePath)
	}
	if got.TotalBytes != int64(len("hello\nworld\n")) {
		t.Fatalf("expected total_bytes=%d, got %d", len("hello\nworld\n"), got.TotalBytes)
	}
	if got.TotalLines != 2 {
		t.Fatalf("expected total_lines=%d, got %d", 2, got.TotalLines)
	}

	content, err := os.ReadFile(filepath.Join(resolvedRoot, "dir", "file.txt"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(content) != "hello\nworld\n" {
		t.Fatalf("unexpected content: %q", string(content))
	}
}

func TestWriteFileTool_AppendsFile(t *testing.T) {
	root := t.TempDir()
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: root})
	resolvedRoot, err := resolveWorkspaceRoot(ctx)
	if err != nil {
		t.Fatalf("resolve workspace root: %v", err)
	}

	initialRaw, _ := json.Marshal(map[string]any{
		"filePath": "a.txt",
		"content":  "hello",
	})
	if _, err := runWriteFileTool(ctx, initialRaw); err != nil {
		t.Fatalf("write file: %v", err)
	}

	appendRaw, _ := json.Marshal(map[string]any{
		"filePath": "a.txt",
		"content":  "\nworld\n",
		"append":   true,
	})
	gotAny, err := runWriteFileTool(ctx, appendRaw)
	if err != nil {
		t.Fatalf("append file: %v", err)
	}
	got, ok := gotAny.(writeFileResult)
	if !ok {
		t.Fatalf("expected writeFileResult, got %T", gotAny)
	}
	if !got.OK {
		t.Fatalf("expected ok=true, got %+v", got)
	}
	if got.Mode != "append" {
		t.Fatalf("expected mode=%q, got %q", "append", got.Mode)
	}
	if got.WrittenLines != 2 {
		t.Fatalf("expected written_lines=%d, got %d", 2, got.WrittenLines)
	}
	if got.TotalBytes != int64(len("hello\nworld\n")) {
		t.Fatalf("expected total_bytes=%d, got %d", len("hello\nworld\n"), got.TotalBytes)
	}
	if got.TotalLines != 2 {
		t.Fatalf("expected total_lines=%d, got %d", 2, got.TotalLines)
	}

	content, err := os.ReadFile(filepath.Join(resolvedRoot, "a.txt"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(content) != "hello\nworld\n" {
		t.Fatalf("unexpected content: %q", string(content))
	}
}

func TestWriteFileTool_TruncatesLargeContent(t *testing.T) {
	root := t.TempDir()
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: root})
	resolvedRoot, err := resolveWorkspaceRoot(ctx)
	if err != nil {
		t.Fatalf("resolve workspace root: %v", err)
	}

	raw, _ := json.Marshal(map[string]any{
		"filePath": "a.txt",
		"content":  strings.Repeat("a", maxWriteFileRunesPerCall+10),
	})
	gotAny, err := runWriteFileTool(ctx, raw)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	got, ok := gotAny.(writeFileResult)
	if !ok {
		t.Fatalf("expected writeFileResult, got %T", gotAny)
	}
	if !got.OK {
		t.Fatalf("expected ok=true, got %+v", got)
	}
	if got.Mode != "overwrite" {
		t.Fatalf("expected mode=%q, got %q", "overwrite", got.Mode)
	}
	if !got.Truncated || !got.ContinueAppend {
		t.Fatalf("expected truncated + continue_append, got %+v", got)
	}
	if got.TotalBytes > int64(maxWriteFileRunesPerCall) {
		t.Fatalf("expected total_bytes <= %d, got %d", maxWriteFileRunesPerCall, got.TotalBytes)
	}
	if got.TotalLines != 1 {
		t.Fatalf("expected total_lines=%d, got %d", 1, got.TotalLines)
	}

	content, err := os.ReadFile(filepath.Join(resolvedRoot, "a.txt"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if runeCount(string(content)) > maxWriteFileRunesPerCall {
		t.Fatalf("expected content <= %d runes, got %d", maxWriteFileRunesPerCall, runeCount(string(content)))
	}
}
