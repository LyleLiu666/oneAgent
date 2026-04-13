package formalmemory

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectScopeIDFromWorkspaceRoot_UsesStableGitIdentity(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoA := initGitRepoWithOrigin(t, "git@github.com:example/oneagent.git")
	repoB := initGitRepoWithOrigin(t, "https://github.com/example/oneagent.git")

	idA := projectScopeIDFromWorkspaceRoot(repoA)
	idB := projectScopeIDFromWorkspaceRoot(repoB)

	if !strings.HasPrefix(idA, "repo:") {
		t.Fatalf("expected repo-scoped project id, got %q", idA)
	}
	if idA != idB {
		t.Fatalf("expected same repo identity across host paths, got %q vs %q", idA, idB)
	}
}

func TestProjectScopeIDFromWorkspaceRoot_DistinguishesSubdirProjects(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repo := initGitRepoWithOrigin(t, "git@github.com:example/oneagent.git")
	subdir := filepath.Join(repo, "services", "agent")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}

	rootID := projectScopeIDFromWorkspaceRoot(repo)
	subdirID := projectScopeIDFromWorkspaceRoot(subdir)
	if rootID == subdirID {
		t.Fatalf("expected repo root and subdir to use different project ids, got %q", rootID)
	}
}

func initGitRepoWithOrigin(t *testing.T, remote string) string {
	t.Helper()

	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "remote", "add", "origin", remote)
	return dir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmdArgs := make([]string, 0, len(args)+2)
	cmdArgs = append(cmdArgs, "-C", dir)
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command("git", cmdArgs...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, strings.TrimSpace(string(out)))
	}
}
