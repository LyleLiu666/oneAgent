package tool

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/scope"
)

func TestReadFileTool_ReadsSmallFile(t *testing.T) {
	root := t.TempDir()
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: root})
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("hello\nworld\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	raw, _ := json.Marshal(map[string]any{
		"filePath": "a.txt",
	})
	gotAny, err := runReadFileTool(ctx, raw)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	got, ok := gotAny.(readFileResult)
	if !ok {
		t.Fatalf("expected readFileResult, got %T", gotAny)
	}
	if !got.OK {
		t.Fatalf("expected ok=true")
	}
	if got.StartLine != 1 || got.EndLine != 2 {
		t.Fatalf("expected lines 1..2, got %d..%d", got.StartLine, got.EndLine)
	}
	if got.Truncated {
		t.Fatalf("expected truncated=false")
	}
	if got.Content != "hello\nworld\n" {
		t.Fatalf("unexpected content: %q", got.Content)
	}
}

func TestReadFileTool_Paging(t *testing.T) {
	root := t.TempDir()
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: root})
	var b strings.Builder
	for i := 1; i <= 500; i++ {
		b.WriteString("line ")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("\n")
	}
	if err := os.WriteFile(filepath.Join(root, "big.txt"), []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	raw, _ := json.Marshal(map[string]any{
		"filePath":     "big.txt",
		"offset_lines": 0,
		"limit_lines":  200,
		"max_bytes":    1024 * 1024,
	})
	gotAny, err := runReadFileTool(ctx, raw)
	if err != nil {
		t.Fatalf("read page1: %v", err)
	}
	page1 := gotAny.(readFileResult)
	if page1.StartLine != 1 || page1.EndLine != 200 {
		t.Fatalf("expected 1..200, got %d..%d", page1.StartLine, page1.EndLine)
	}
	if !strings.Contains(page1.Content, "line 200\n") {
		t.Fatalf("expected line 200 present")
	}

	raw2, _ := json.Marshal(map[string]any{
		"filePath":     "big.txt",
		"offset_lines": 200,
		"limit_lines":  200,
		"max_bytes":    1024 * 1024,
	})
	gotAny, err = runReadFileTool(ctx, raw2)
	if err != nil {
		t.Fatalf("read page2: %v", err)
	}
	page2 := gotAny.(readFileResult)
	if page2.StartLine != 201 || page2.EndLine != 400 {
		t.Fatalf("expected 201..400, got %d..%d", page2.StartLine, page2.EndLine)
	}
	if !strings.Contains(page2.Content, "line 201\n") {
		t.Fatalf("expected line 201 present")
	}
}

func TestReadFileTool_MaxBytesTruncation(t *testing.T) {
	root := t.TempDir()
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: root})
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("你好你好你好你好你好\nworld\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	raw, _ := json.Marshal(map[string]any{
		"filePath":  "a.txt",
		"max_bytes": 10,
	})
	gotAny, err := runReadFileTool(ctx, raw)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	got := gotAny.(readFileResult)
	if !got.Truncated {
		t.Fatalf("expected truncated=true")
	}
	if !utf8.ValidString(got.Content) {
		t.Fatalf("expected valid utf8")
	}
	if len([]byte(got.Content)) > 10 {
		t.Fatalf("expected <=10 bytes, got %d", len([]byte(got.Content)))
	}
}

func TestReadFileTool_NoWorkspaceRelativeDenied(t *testing.T) {
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: false, Root: ""})
	raw, _ := json.Marshal(map[string]any{"filePath": "a.txt"})
	_, err := runReadFileTool(ctx, raw)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestReadFileTool_NoWorkspaceAbsoluteAllowed(t *testing.T) {
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	tmp := filepath.Join(t.TempDir(), "abs.txt")
	if err := os.WriteFile(tmp, []byte("ok\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: false, Root: ""})
	raw, _ := json.Marshal(map[string]any{"filePath": tmp})
	gotAny, err := runReadFileTool(ctx, raw)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	got := gotAny.(readFileResult)
	if got.Content != "ok\n" {
		t.Fatalf("unexpected content: %q", got.Content)
	}
}

func TestReadFileTool_SymlinkEscapeDenied(t *testing.T) {
	root := t.TempDir()
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	outsideDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(outsideDir, "secret.txt"), []byte("secret"), 0o600); err != nil {
		t.Fatalf("write outside: %v", err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(outsideDir, link); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: root})
	raw, _ := json.Marshal(map[string]any{"filePath": filepath.Join("link", "secret.txt")})
	_, err := runReadFileTool(ctx, raw)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !errors.Is(err, scope.ErrPathOutsideWorkspace) && !strings.Contains(err.Error(), "outside workspace") {
		t.Fatalf("expected outside workspace error, got %v", err)
	}
}
