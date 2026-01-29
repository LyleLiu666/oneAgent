package server

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateDiffArtifacts_GitWorkspace_WritesPatchAndChangedFiles(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}

	workspace := t.TempDir()
	runGit(t, workspace, "init")
	runGit(t, workspace, "config", "user.email", "test@example.com")
	runGit(t, workspace, "config", "user.name", "Test")
	writeFile(t, filepath.Join(workspace, "a.txt"), "hello\n")
	runGit(t, workspace, "add", ".")
	runGit(t, workspace, "commit", "-m", "init")

	writeFile(t, filepath.Join(workspace, "a.txt"), "hello world\n")

	outDir := filepath.Join(t.TempDir(), "artifacts")
	got, err := generateDiffArtifacts(context.Background(), workspace, "", outDir)
	if err != nil {
		t.Fatalf("generateDiffArtifacts: %v", err)
	}
	if !got.IsGitWorkspace {
		t.Fatalf("expected git workspace")
	}
	if strings.TrimSpace(got.ChangedFilesPath) == "" {
		t.Fatalf("expected changed_files_path")
	}
	if _, err := os.Stat(got.ChangedFilesPath); err != nil {
		t.Fatalf("changed_files_path missing: %v", err)
	}
	cf, _ := os.ReadFile(got.ChangedFilesPath)
	if !strings.Contains(string(cf), "a.txt") {
		t.Fatalf("changed_files missing a.txt: %s", string(cf))
	}
	if strings.TrimSpace(got.DiffPatchPath) == "" {
		t.Fatalf("expected diff_patch_path")
	}
	patch, _ := os.ReadFile(got.DiffPatchPath)
	if !strings.Contains(string(patch), "diff --git") {
		t.Fatalf("diff patch looks wrong: %s", string(patch))
	}
}

func TestGenerateDiffArtifacts_NonGitWorkspace_UsesFindingsChangedFiles(t *testing.T) {
	workspace := t.TempDir()
	findings := filepath.Join(t.TempDir(), "FINDINGS.md")
	writeFile(t, findings, "# FINDINGS\n\n## 变更文件\n- foo.go\n- bar.md\n\n## Findings\n- ok\n")

	outDir := filepath.Join(t.TempDir(), "artifacts")
	got, err := generateDiffArtifacts(context.Background(), workspace, findings, outDir)
	if err != nil {
		t.Fatalf("generateDiffArtifacts: %v", err)
	}
	if got.IsGitWorkspace {
		t.Fatalf("expected non-git workspace")
	}
	if strings.TrimSpace(got.DiffPatchPath) != "" {
		t.Fatalf("expected diff_patch_path omitted for non-git workspace")
	}
	cf, _ := os.ReadFile(got.ChangedFilesPath)
	if !strings.Contains(string(cf), "foo.go") || !strings.Contains(string(cf), "bar.md") {
		t.Fatalf("changed_files should include findings entries: %s", string(cf))
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, string(out))
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

