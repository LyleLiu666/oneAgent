package projectcfg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeProjectJSON(t *testing.T, workspaceRoot, content string) string {
	t.Helper()
	dir := filepath.Join(workspaceRoot, ".oneagent")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir .oneagent: %v", err)
	}
	path := filepath.Join(dir, "project.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write project.json: %v", err)
	}
	return path
}

func TestLoad_MissingFileReturnsFoundFalse(t *testing.T) {
	workspaceRoot := t.TempDir()
	_, found, err := Load(workspaceRoot)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if found {
		t.Fatalf("expected found=false")
	}
}

func TestLoad_ValidConfigParses(t *testing.T) {
	workspaceRoot := t.TempDir()
	writeProjectJSON(t, workspaceRoot, `{
			"setup_script":"echo setup",
			"test_script":"echo test",
			"cleanup_script":"echo cleanup",
			"dev_server_script":"echo dev",
			"copy_files":[".env","config/.env.local"],
			"attempt_execution_mode":"worktree"
		}`)

	cfg, found, err := Load(workspaceRoot)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !found {
		t.Fatalf("expected found=true")
	}
	if cfg.SetupScript != "echo setup" || cfg.TestScript != "echo test" || cfg.CleanupScript != "echo cleanup" || cfg.DevServerScript != "echo dev" {
		t.Fatalf("unexpected scripts: %+v", cfg)
	}
	if cfg.AttemptExecutionMode != "worktree" {
		t.Fatalf("unexpected attempt_execution_mode: %+v", cfg)
	}
	if len(cfg.CopyFiles) != 2 || cfg.CopyFiles[0] != ".env" || cfg.CopyFiles[1] != "config/.env.local" {
		t.Fatalf("unexpected copy_files: %+v", cfg.CopyFiles)
	}
}

func TestLoad_UnknownFieldReturnsActionableError(t *testing.T) {
	workspaceRoot := t.TempDir()
	path := writeProjectJSON(t, workspaceRoot, `{"unknown_field": 1}`)

	_, found, err := Load(workspaceRoot)
	if !found {
		t.Fatalf("expected found=true")
	}
	if err == nil {
		t.Fatalf("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, path) || !strings.Contains(strings.ToLower(msg), "unknown") {
		t.Fatalf("expected actionable error mentioning path and unknown field, got: %q", msg)
	}
}

func TestLoad_CopyFilesEscapeIsRejected(t *testing.T) {
	workspaceRoot := t.TempDir()
	path := writeProjectJSON(t, workspaceRoot, `{"copy_files":["../secrets.env"]}`)

	_, found, err := Load(workspaceRoot)
	if !found {
		t.Fatalf("expected found=true")
	}
	if err == nil {
		t.Fatalf("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, path) || !strings.Contains(msg, "copy_files") {
		t.Fatalf("expected actionable error mentioning path and copy_files, got: %q", msg)
	}
}

func TestLoad_CopyFilesAbsolutePathIsRejected(t *testing.T) {
	workspaceRoot := t.TempDir()
	path := writeProjectJSON(t, workspaceRoot, `{"copy_files":["/etc/passwd"]}`)

	_, found, err := Load(workspaceRoot)
	if !found {
		t.Fatalf("expected found=true")
	}
	if err == nil {
		t.Fatalf("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, path) || !strings.Contains(msg, "copy_files") {
		t.Fatalf("expected actionable error mentioning path and copy_files, got: %q", msg)
	}
}
