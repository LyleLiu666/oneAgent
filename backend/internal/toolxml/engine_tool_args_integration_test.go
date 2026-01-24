package toolxml

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/shell"
	"github.com/liu_y/oneAgent/backend/internal/tool"
)

func mustToolDefinition(t *testing.T, name string) tool.Definition {
	t.Helper()

	for _, def := range tool.All() {
		if def.Spec.Function.Name == name {
			return def
		}
	}
	t.Fatalf("tool %q not found", name)
	return tool.Definition{}
}

func TestBuildToolArgs_Edit_RunsEditTool(t *testing.T) {
	root := t.TempDir()
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{BashRootDir: root}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	resolvedRoot, err := shell.ResolveBashRoot(root)
	if err != nil {
		t.Fatalf("resolve root: %v", err)
	}

	target := filepath.Join(resolvedRoot, "a.txt")
	if err := os.WriteFile(target, []byte("hello\nworld\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	args, _, err := buildToolArgs("edit", map[string]string{
		"filePath":    "a.txt",
		"oldcontent":  "hello",
		"newcontent":  "hi",
		"replaceAll":  "false",
		"unusedField": "ignored",
	})
	if err != nil {
		t.Fatalf("build args: %v", err)
	}

	editDef := mustToolDefinition(t, "edit")
	if _, err := editDef.Handler(context.Background(), args); err != nil {
		t.Fatalf("edit tool: %v", err)
	}

	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(content) != "hi\nworld\n" {
		t.Fatalf("unexpected content: %q", string(content))
	}
}

func TestBuildToolArgs_WriteFile_RunsWriteFileTool(t *testing.T) {
	root := t.TempDir()
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{BashRootDir: root}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	resolvedRoot, err := shell.ResolveBashRoot(root)
	if err != nil {
		t.Fatalf("resolve root: %v", err)
	}

	args, _, err := buildToolArgs("write_file", map[string]string{
		"filePath": "dir/new.txt",
		"content":  "abc\n",
	})
	if err != nil {
		t.Fatalf("build args: %v", err)
	}

	writeDef := mustToolDefinition(t, "write_file")
	if _, err := writeDef.Handler(context.Background(), args); err != nil {
		t.Fatalf("write_file tool: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(resolvedRoot, "dir", "new.txt"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(content) != "abc\n" {
		t.Fatalf("unexpected content: %q", string(content))
	}
}

func TestBuildToolArgs_WriteFile_WithAppend_Appends(t *testing.T) {
	root := t.TempDir()
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{BashRootDir: root}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	resolvedRoot, err := shell.ResolveBashRoot(root)
	if err != nil {
		t.Fatalf("resolve root: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(resolvedRoot, "dir"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(resolvedRoot, "dir", "new.txt"), []byte("a"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	args, _, err := buildToolArgs("write_file", map[string]string{
		"filePath": "dir/new.txt",
		"content":  "b",
		"append":   "true",
	})
	if err != nil {
		t.Fatalf("build args: %v", err)
	}

	writeDef := mustToolDefinition(t, "write_file")
	if _, err := writeDef.Handler(context.Background(), args); err != nil {
		t.Fatalf("write_file tool: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(resolvedRoot, "dir", "new.txt"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(content) != "ab" {
		t.Fatalf("unexpected content: %q", string(content))
	}
}

func TestBuildToolArgs_EditWithContent_IsRejected(t *testing.T) {
	_, _, err := buildToolArgs("edit", map[string]string{
		"filePath": "a.txt",
		"content":  "hello",
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestBuildToolArgs_EditWithCommand_IsRejected(t *testing.T) {
	_, _, err := buildToolArgs("edit", map[string]string{
		"command": "apply_edit <<'EOF'\nfile: a.txt\n<<<< SEARCH\nhello\n==== REPLACE\nhi\n>>>>\nEOF\n",
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "apply_edit") || !strings.Contains(err.Error(), "command") {
		t.Fatalf("unexpected error: %v", err)
	}
}
