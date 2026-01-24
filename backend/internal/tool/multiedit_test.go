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

	fileA := filepath.Join(root, "a.txt")
	fileB := filepath.Join(root, "b.txt")
	if err := os.WriteFile(fileA, []byte("hello\nworld\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := os.WriteFile(fileB, []byte("target\nx\ntarget\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	raw, _ := json.Marshal(map[string]any{
		"edits": []map[string]any{
			{
				"filePath":  "a.txt",
				"oldString": "hello",
				"newString": "hi",
			},
			{
				"filePath":  "b.txt",
				"oldString": "target",
				"newString": "done",
			},
		},
		"replaceAll": true,
	})

	gotAny, err := runMultiEditTool(context.Background(), raw)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	got, ok := gotAny.(SmartEditResult)
	if !ok {
		t.Fatalf("expected SmartEditResult, got %T", gotAny)
	}
	if got.Replacements != 3 {
		t.Fatalf("expected replacements=3, got %d", got.Replacements)
	}
	if len(got.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(got.Files))
	}

	contentA, err := os.ReadFile(fileA)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(contentA) != "hi\nworld\n" {
		t.Fatalf("unexpected content for a.txt: %q", string(contentA))
	}
	contentB, err := os.ReadFile(fileB)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(contentB) != "done\nx\ndone\n" {
		t.Fatalf("unexpected content for b.txt: %q", string(contentB))
	}
}
