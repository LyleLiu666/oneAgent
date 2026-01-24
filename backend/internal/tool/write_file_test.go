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
	config.AppConfig = &config.Config{BashRootDir: root}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	resolvedRoot, err := resolveSmartEditRoot()
	if err != nil {
		t.Fatalf("resolve root: %v", err)
	}

	raw, _ := json.Marshal(map[string]any{
		"filePath": "dir/file.txt",
		"content":  "hello\nworld\n",
	})

	gotAny, err := runWriteFileTool(context.Background(), raw)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	got, ok := gotAny.(writeFileResult)
	if !ok {
		t.Fatalf("expected writeFileResult, got %T", gotAny)
	}
	if got.WrittenBytes != len("hello\nworld\n") {
		t.Fatalf("expected written_bytes=%d, got %d", len("hello\nworld\n"), got.WrittenBytes)
	}
	if got.FilePath != filepath.Join(resolvedRoot, "dir", "file.txt") {
		t.Fatalf("expected file_path=%q, got %q", filepath.Join(resolvedRoot, "dir", "file.txt"), got.FilePath)
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
	config.AppConfig = &config.Config{BashRootDir: root}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	resolvedRoot, err := resolveSmartEditRoot()
	if err != nil {
		t.Fatalf("resolve root: %v", err)
	}

	initialRaw, _ := json.Marshal(map[string]any{
		"filePath": "a.txt",
		"content":  "hello",
	})
	if _, err := runWriteFileTool(context.Background(), initialRaw); err != nil {
		t.Fatalf("write file: %v", err)
	}

	appendRaw, _ := json.Marshal(map[string]any{
		"filePath": "a.txt",
		"content":  "\nworld\n",
		"append":   true,
	})
	if _, err := runWriteFileTool(context.Background(), appendRaw); err != nil {
		t.Fatalf("append file: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(resolvedRoot, "a.txt"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(content) != "hello\nworld\n" {
		t.Fatalf("unexpected content: %q", string(content))
	}
}

func TestWriteFileTool_RejectsLargeContent(t *testing.T) {
	root := t.TempDir()
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{BashRootDir: root}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	resolvedRoot, err := resolveSmartEditRoot()
	if err != nil {
		t.Fatalf("resolve root: %v", err)
	}

	raw, _ := json.Marshal(map[string]any{
		"filePath": "a.txt",
		"content":  strings.Repeat("a", maxWriteFileRunesPerCall+10),
	})
	gotAny, err := runWriteFileTool(context.Background(), raw)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	got, ok := gotAny.(writeFileResult)
	if !ok {
		t.Fatalf("expected writeFileResult, got %T", gotAny)
	}
	if !got.Truncated || !got.ContinueAppend {
		t.Fatalf("expected truncated + continue_append, got %+v", got)
	}

	content, err := os.ReadFile(filepath.Join(resolvedRoot, "a.txt"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if runeCount(string(content)) > maxWriteFileRunesPerCall {
		t.Fatalf("expected content <= %d runes, got %d", maxWriteFileRunesPerCall, runeCount(string(content)))
	}
}
