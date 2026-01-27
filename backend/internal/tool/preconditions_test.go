package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/shell"
)

func TestWriteFile_Preconditions_ExpectedSHA256(t *testing.T) {
	root := t.TempDir()
	resolvedRoot, err := shell.ResolveBashRoot(root)
	if err != nil {
		t.Fatalf("ResolveBashRoot: %v", err)
	}
	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: resolvedRoot})

	target := filepath.Join(resolvedRoot, "a.txt")
	if err := os.WriteFile(target, []byte("A\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	sha, err := fileSHA256Hex(target)
	if err != nil {
		t.Fatalf("sha: %v", err)
	}

	def := writeFileDefinition()

	// Mismatch should fail.
	{
		req := map[string]any{
			"filePath": "a.txt",
			"content":  "B\n",
			"preconditions": map[string]any{
				"expected_sha256": strings.Repeat("0", len(sha)),
			},
		}
		raw, _ := json.Marshal(req)
		_, err := def.Handler(ctx, raw)
		if err == nil || !strings.Contains(err.Error(), "precondition failed") {
			t.Fatalf("expected precondition error, got: %v", err)
		}
	}

	// Match should succeed.
	{
		req := map[string]any{
			"filePath": "a.txt",
			"content":  "B\n",
			"preconditions": map[string]any{
				"expected_sha256": sha,
			},
		}
		raw, _ := json.Marshal(req)
		_, err := def.Handler(ctx, raw)
		if err != nil {
			t.Fatalf("expected success, got: %v", err)
		}
	}
}

func TestEdit_Preconditions_ExpectedSHA256(t *testing.T) {
	root := t.TempDir()
	resolvedRoot, err := shell.ResolveBashRoot(root)
	if err != nil {
		t.Fatalf("ResolveBashRoot: %v", err)
	}
	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: resolvedRoot})

	target := filepath.Join(resolvedRoot, "a.txt")
	if err := os.WriteFile(target, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	sha, err := fileSHA256Hex(target)
	if err != nil {
		t.Fatalf("sha: %v", err)
	}

	def := smartEditDefinition()

	// Mismatch should fail.
	{
		req := map[string]any{
			"edits": []map[string]any{
				{
					"filePath":   "a.txt",
					"oldString":  "hello",
					"newString":  "hi",
					"preconditions": map[string]any{"expected_sha256": strings.Repeat("0", len(sha))},
				},
			},
		}
		raw, _ := json.Marshal(req)
		_, err := def.Handler(ctx, raw)
		if err == nil || !strings.Contains(err.Error(), "precondition failed") {
			t.Fatalf("expected precondition error, got: %v", err)
		}
	}

	// Match should succeed.
	{
		req := map[string]any{
			"edits": []map[string]any{
				{
					"filePath":   "a.txt",
					"oldString":  "hello",
					"newString":  "hi",
					"preconditions": map[string]any{"expected_sha256": sha},
				},
			},
		}
		raw, _ := json.Marshal(req)
		_, err := def.Handler(ctx, raw)
		if err != nil {
			t.Fatalf("expected success, got: %v", err)
		}
	}
}

