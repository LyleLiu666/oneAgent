package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/config"
)

func TestMultiEditTool_AppliesEdits(t *testing.T) {
	root := t.TempDir()
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{BashRootDir: root}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	resolvedRoot, err := resolveSmartEditRoot()
	if err != nil {
		t.Fatalf("resolve root: %v", err)
	}

	target := filepath.Join(resolvedRoot, "a.txt")
	if err := os.WriteFile(target, []byte("hello\nworld\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	defs, err := Mount([]string{ToolIDMultiEdit})
	if err != nil {
		t.Fatalf("mount: %v", err)
	}
	if len(defs) != 1 || defs[0].Spec.Function.Name != "multiedit" {
		t.Fatalf("expected multiedit tool, got %+v", defs)
	}

	raw, _ := json.Marshal(map[string]any{
		"edits": []map[string]any{
			{
				"filePath":  "a.txt",
				"oldString": "hello",
				"newString": "hi",
			},
		},
	})

	if _, err := defs[0].Handler(context.Background(), raw); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(content) != "hi\nworld\n" {
		t.Fatalf("unexpected content: %q", string(content))
	}
}
