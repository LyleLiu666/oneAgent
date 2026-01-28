package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
)

func TestServer_DocumentExportAPI_Smoke(t *testing.T) {
	home := t.TempDir()
	t.Setenv("ONEAGENT_HOME", home)
	t.Setenv("HOME", home)
	cfg := &config.Config{
		Profile:          "local",
		Bind:             "127.0.0.1",
		Port:             "0",
		Home:             home,
		AuthMode:         "none",
		LogRetentionDays: 1,
	}
	rt, err := runtime.Init(cfg)
	if err != nil {
		t.Fatalf("init runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "report.md"), []byte("# hello\n"), 0o644); err != nil {
		t.Fatalf("write md: %v", err)
	}

	// Use test helper as pandoc binary.
	t.Setenv("ONEAGENT_PANDOC_CMD", os.Args[0])
	t.Setenv("ONEAGENT_PANDOC_ARGS", "-test.run=TestHelperPandoc --")
	t.Setenv("ONEAGENT_PANDOC_TEST", "1")

	body := map[string]any{
		"workspace":   workspace,
		"input_path":  "report.md",
		"format":      "docx",
		"output_path": "report.docx",
	}
	b, _ := json.Marshal(body)
	res, err := http.Post(srv.URL+"/api/documents/export", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST /api/documents/export: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", res.StatusCode)
	}

	var out struct {
		OK         bool   `json:"ok"`
		OutputPath string `json:"output_path"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !out.OK {
		t.Fatalf("expected ok=true")
	}
	if strings.TrimSpace(out.OutputPath) == "" {
		t.Fatalf("expected output_path")
	}
	if _, err := os.Stat(filepath.Join(workspace, out.OutputPath)); err != nil {
		t.Fatalf("expected output file exists: %v", err)
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

