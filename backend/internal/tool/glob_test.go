package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/config"
)

func TestGlobTool_MatchesPatterns(t *testing.T) {
	root := t.TempDir()
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{BashRootDir: root}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	resolvedRoot, err := resolveSmartEditRoot()
	if err != nil {
		t.Fatalf("resolve root: %v", err)
	}

	writeFile := func(path, content string) {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}

	writeFile(filepath.Join(resolvedRoot, "dir", "b.go"), "package main\n")
	writeFile(filepath.Join(resolvedRoot, "dir", "sub", "c.go"), "package main\n")
	writeFile(filepath.Join(resolvedRoot, "dir", "sub", "d.txt"), "text\n")

	raw, _ := json.Marshal(map[string]any{
		"pattern": "dir/*.go",
	})

	gotAny, err := runGlobTool(context.Background(), raw)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	got, ok := gotAny.(GlobToolResult)
	if !ok {
		t.Fatalf("expected GlobToolResult, got %T", gotAny)
	}
	want := []string{filepath.Join(resolvedRoot, "dir", "b.go")}
	if !reflect.DeepEqual(got.Matches, want) {
		t.Fatalf("expected matches %v, got %v", want, got.Matches)
	}

	raw, _ = json.Marshal(map[string]any{
		"pattern": "dir/**/*.go",
	})

	gotAny, err = runGlobTool(context.Background(), raw)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	got, ok = gotAny.(GlobToolResult)
	if !ok {
		t.Fatalf("expected GlobToolResult, got %T", gotAny)
	}
	want = []string{
		filepath.Join(resolvedRoot, "dir", "b.go"),
		filepath.Join(resolvedRoot, "dir", "sub", "c.go"),
	}
	if !reflect.DeepEqual(got.Matches, want) {
		t.Fatalf("expected matches %v, got %v", want, got.Matches)
	}
}
