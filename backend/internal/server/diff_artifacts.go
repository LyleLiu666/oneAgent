package server

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type DiffArtifacts struct {
	DiffPatchPath    string
	ChangedFilesPath string
	Reason           string
	IsGitWorkspace   bool
}

const maxDiffPatchBytes = 512 * 1024

func generateDiffArtifacts(ctx context.Context, workspaceRoot, findingsPath, outDir string) (DiffArtifacts, error) {
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	outDir = strings.TrimSpace(outDir)
	findingsPath = strings.TrimSpace(findingsPath)
	if workspaceRoot == "" {
		return DiffArtifacts{}, errors.New("workspaceRoot is required")
	}
	if outDir == "" {
		return DiffArtifacts{}, errors.New("outDir is required")
	}
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return DiffArtifacts{}, fmt.Errorf("create diff artifacts dir: %w", err)
	}

	changedFilesPath := filepath.Join(outDir, "changed_files.txt")
	diffPatchPath := filepath.Join(outDir, "diff.patch")

	createdAt := time.Now().UTC().Format(time.RFC3339)

	isGit := isGitWorkspace(ctx, workspaceRoot)
	changedFiles := []string{}
	reason := ""

	if isGit {
		changedFiles = listGitChangedFiles(ctx, workspaceRoot)
		if len(changedFiles) == 0 {
			reason = "git workspace: no changes detected"
		} else {
			patch, patchReason := gitDiffHead(ctx, workspaceRoot)
			if patchReason != "" {
				reason = patchReason
			} else if len(patch) > maxDiffPatchBytes {
				reason = fmt.Sprintf("git diff omitted: patch too large (%d bytes > %d bytes)", len(patch), maxDiffPatchBytes)
			} else {
				if err := os.WriteFile(diffPatchPath, patch, 0o600); err != nil {
					reason = "git diff omitted: " + err.Error()
				}
			}
		}
	} else {
		changedFiles = extractChangedFilesFromFindings(findingsPath)
		if len(changedFiles) == 0 {
			reason = "not a git workspace; no changed files found in findings"
		} else {
			reason = "not a git workspace"
		}
	}

	if err := writeChangedFilesReport(changedFilesPath, createdAt, workspaceRoot, isGit, reason, changedFiles); err != nil {
		return DiffArtifacts{}, err
	}

	// Only return diff_patch_path when the file is actually created.
	if _, err := os.Stat(diffPatchPath); err != nil {
		diffPatchPath = ""
	}

	return DiffArtifacts{
		DiffPatchPath:    diffPatchPath,
		ChangedFilesPath: changedFilesPath,
		Reason:           strings.TrimSpace(reason),
		IsGitWorkspace:   isGit,
	}, nil
}

func isGitWorkspace(ctx context.Context, workspaceRoot string) bool {
	if _, err := exec.LookPath("git"); err != nil {
		return false
	}
	out, err := runCmd(ctx, workspaceRoot, "git", "rev-parse", "--is-inside-work-tree")
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}

func listGitChangedFiles(ctx context.Context, workspaceRoot string) []string {
	seen := make(map[string]struct{})
	addLines := func(lines []string) {
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			seen[line] = struct{}{}
		}
	}

	if out, err := runCmd(ctx, workspaceRoot, "git", "diff", "--name-only", "HEAD"); err == nil {
		addLines(strings.Split(string(out), "\n"))
	}
	if out, err := runCmd(ctx, workspaceRoot, "git", "ls-files", "--others", "--exclude-standard"); err == nil {
		addLines(strings.Split(string(out), "\n"))
	}

	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func gitDiffHead(ctx context.Context, workspaceRoot string) ([]byte, string) {
	if _, err := exec.LookPath("git"); err != nil {
		return nil, "git diff omitted: git not found"
	}
	patch, err := runCmd(ctx, workspaceRoot, "git", "diff", "--patch", "HEAD")
	if err != nil {
		return nil, "git diff omitted: " + err.Error()
	}
	if len(patch) == 0 {
		return nil, "git diff omitted: empty patch"
	}
	return patch, ""
}

func writeChangedFilesReport(path, createdAt, workspaceRoot string, isGit bool, reason string, files []string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("prepare changed_files dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open changed_files: %w", err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	defer w.Flush()

	fmt.Fprintln(w, "# changed_files")
	fmt.Fprintf(w, "# generated_at: %s\n", strings.TrimSpace(createdAt))
	fmt.Fprintf(w, "# workspace_root: %s\n", strings.TrimSpace(workspaceRoot))
	fmt.Fprintf(w, "# git_workspace: %t\n", isGit)
	if strings.TrimSpace(reason) != "" {
		fmt.Fprintf(w, "# note: %s\n", strings.TrimSpace(reason))
	}
	fmt.Fprintln(w)

	if len(files) == 0 {
		fmt.Fprintln(w, "- (none)")
		return nil
	}
	for _, p := range files {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		fmt.Fprintln(w, p)
	}
	return nil
}

func extractChangedFilesFromFindings(findingsPath string) []string {
	findingsPath = strings.TrimSpace(findingsPath)
	if findingsPath == "" {
		return nil
	}
	data, err := os.ReadFile(findingsPath)
	if err != nil {
		return nil
	}
	text := string(data)
	idx := strings.Index(text, "## 变更文件")
	if idx == -1 {
		return nil
	}
	section := text[idx:]
	// Stop at next heading.
	if next := strings.Index(section[len("## 变更文件"):], "\n## "); next != -1 {
		section = section[:len("## 变更文件")+next+1]
	}

	lines := strings.Split(section, "\n")
	out := make([]string, 0, 16)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- ") {
			continue
		}
		item := strings.TrimSpace(strings.TrimPrefix(line, "- "))
		if item == "" || strings.Contains(item, "（无）") || strings.Contains(item, "(none)") {
			continue
		}
		out = append(out, item)
	}

	uniq := make(map[string]struct{}, len(out))
	for _, p := range out {
		uniq[p] = struct{}{}
	}
	out = out[:0]
	for p := range uniq {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func runCmd(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err == nil {
		return out, nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		msg := strings.TrimSpace(string(ee.Stderr))
		if msg != "" {
			return nil, fmt.Errorf("%s: %s", name, msg)
		}
		return nil, fmt.Errorf("%s exit_code=%d", name, ee.ExitCode())
	}
	return nil, err
}

