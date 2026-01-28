package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/config"
)

func TestDocumentExport_MDToDocx_WritesOutputWithinWorkspace(t *testing.T) {
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	root := t.TempDir()
	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: root})

	if err := os.WriteFile(filepath.Join(root, "report.md"), []byte("# hello\n"), 0o644); err != nil {
		t.Fatalf("write md: %v", err)
	}

	// Use test helper as pandoc binary.
	t.Setenv("ONEAGENT_PANDOC_CMD", os.Args[0])
	t.Setenv("ONEAGENT_PANDOC_ARGS", "-test.run=TestHelperPandoc --")
	t.Setenv("ONEAGENT_PANDOC_TEST", "1")

	raw, _ := json.Marshal(map[string]any{
		"input_path": "report.md",
		"format":     "docx",
	})
	gotAny, err := runDocumentExportTool(ctx, raw)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	got, ok := gotAny.(documentExportResult)
	if !ok || !got.OK {
		t.Fatalf("unexpected result: %#v (%T)", gotAny, gotAny)
	}
	if strings.TrimSpace(got.OutputPath) == "" {
		t.Fatalf("expected output_path")
	}
	if !strings.HasSuffix(got.OutputPath, ".docx") {
		t.Fatalf("expected docx output, got %q", got.OutputPath)
	}
	if _, err := os.Stat(filepath.Join(root, got.OutputPath)); err != nil {
		t.Fatalf("expected output file exists: %v", err)
	}
}

func TestDocumentExport_RejectsOutputOutsideWorkspace(t *testing.T) {
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	root := t.TempDir()
	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: root})

	if err := os.WriteFile(filepath.Join(root, "report.md"), []byte("# hello\n"), 0o644); err != nil {
		t.Fatalf("write md: %v", err)
	}

	raw, _ := json.Marshal(map[string]any{
		"input_path":  "report.md",
		"format":      "docx",
		"output_path": filepath.Join(os.TempDir(), "out.docx"),
	})
	if _, err := runDocumentExportTool(ctx, raw); err == nil {
		t.Fatalf("expected error")
	}
}

func TestDocumentExport_MissingPandocHasActionableMessage(t *testing.T) {
	prevCfg := config.AppConfig
	config.AppConfig = &config.Config{}
	t.Cleanup(func() { config.AppConfig = prevCfg })

	root := t.TempDir()
	ctx := ContextWithWorkspace(context.Background(), WorkspaceConfig{Enabled: true, Root: root})

	if err := os.WriteFile(filepath.Join(root, "report.md"), []byte("# hello\n"), 0o644); err != nil {
		t.Fatalf("write md: %v", err)
	}

	t.Setenv("ONEAGENT_PANDOC_CMD", "pandoc-does-not-exist")

	raw, _ := json.Marshal(map[string]any{
		"input_path": "report.md",
		"format":     "docx",
	})
	_, err := runDocumentExportTool(ctx, raw)
	if err == nil {
		t.Fatalf("expected error")
	}
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "pandoc") || !strings.Contains(msg, "doctor") {
		t.Fatalf("expected actionable message, got: %q", err.Error())
	}
}

// TestHelperPandoc runs as a subprocess invoked by the export tool in tests.
func TestHelperPandoc(t *testing.T) {
	if strings.TrimSpace(os.Getenv("ONEAGENT_PANDOC_TEST")) != "1" {
		return
	}
	// Find args after "--" terminator.
	args := os.Args
	idx := -1
	for i, a := range args {
		if a == "--" {
			idx = i + 1
			break
		}
	}
	if idx < 0 || idx >= len(args) {
		return
	}
	toolArgs := args[idx:]

	outPath := ""
	for i := 0; i < len(toolArgs); i++ {
		if toolArgs[i] == "-o" && i+1 < len(toolArgs) {
			outPath = toolArgs[i+1]
			break
		}
	}
	if strings.TrimSpace(outPath) == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(outPath), 0o755)
	_ = os.WriteFile(outPath, []byte("fake office file\n"), 0o644)
	fmt.Fprintln(os.Stdout, "written:", outPath)
}

