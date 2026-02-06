package gitutil

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type GitBase struct {
	BaseCommitSHA string
	BaseRef       string
}

func ResolveGitBase(ctx context.Context, workspaceRoot string) (GitBase, error) {
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	if workspaceRoot == "" {
		return GitBase{}, errors.New("workspace_root is required")
	}
	if !IsGitWorkspace(ctx, workspaceRoot) {
		return GitBase{}, errors.New("workspace is not a git repository")
	}

	sha, err := runCmd(ctx, workspaceRoot, "git", "rev-parse", "HEAD")
	if err != nil {
		return GitBase{}, fmt.Errorf("resolve base_commit_sha: %w", err)
	}
	ref, err := runCmd(ctx, workspaceRoot, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return GitBase{}, fmt.Errorf("resolve base_ref: %w", err)
	}

	return GitBase{
		BaseCommitSHA: strings.TrimSpace(string(sha)),
		BaseRef:       strings.TrimSpace(string(ref)),
	}, nil
}

func IsGitWorkspace(ctx context.Context, workspaceRoot string) bool {
	if _, err := exec.LookPath("git"); err != nil {
		return false
	}
	out, err := runCmd(ctx, workspaceRoot, "git", "rev-parse", "--is-inside-work-tree")
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}

func CreateWorktree(ctx context.Context, workspaceRoot, worktreeRoot, baseCommitSHA string) error {
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	worktreeRoot = strings.TrimSpace(worktreeRoot)
	baseCommitSHA = strings.TrimSpace(baseCommitSHA)
	if workspaceRoot == "" {
		return errors.New("workspace_root is required")
	}
	if worktreeRoot == "" {
		return errors.New("worktree_root is required")
	}
	if baseCommitSHA == "" {
		return errors.New("base_commit_sha is required")
	}
	if !IsGitWorkspace(ctx, workspaceRoot) {
		return errors.New("workspace is not a git repository")
	}

	// Best-effort prune before creating a new worktree (helps after unclean shutdowns).
	_, _ = runCmd(ctx, workspaceRoot, "git", "worktree", "prune")

	// If the path already exists, try to remove it first to keep the operation idempotent.
	if _, err := os.Stat(worktreeRoot); err == nil {
		_ = RemoveWorktree(ctx, workspaceRoot, worktreeRoot)
		_ = os.RemoveAll(worktreeRoot)
	}

	if err := os.MkdirAll(filepath.Dir(worktreeRoot), 0o700); err != nil {
		return fmt.Errorf("create worktree parent dir: %w", err)
	}
	if _, err := runCmd(ctx, workspaceRoot, "git", "worktree", "add", "--detach", worktreeRoot, baseCommitSHA); err != nil {
		return fmt.Errorf("git worktree add: %w", err)
	}
	return nil
}

func RemoveWorktree(ctx context.Context, workspaceRoot, worktreeRoot string) error {
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	worktreeRoot = strings.TrimSpace(worktreeRoot)
	if workspaceRoot == "" {
		return errors.New("workspace_root is required")
	}
	if worktreeRoot == "" {
		return errors.New("worktree_root is required")
	}
	if !IsGitWorkspace(ctx, workspaceRoot) {
		return errors.New("workspace is not a git repository")
	}

	// git worktree remove is best-effort; if it fails we still attempt to delete the dir.
	_, err := runCmd(ctx, workspaceRoot, "git", "worktree", "remove", "--force", worktreeRoot)
	_ = os.RemoveAll(worktreeRoot)
	_, _ = runCmd(ctx, workspaceRoot, "git", "worktree", "prune")
	return err
}

type DiffArtifacts struct {
	DiffPatchPath    string
	ChangedFilesPath string
	Reason           string
	IsGitWorkspace   bool
}

const maxDiffPatchBytes = 512 * 1024
const maxFileSnapshotBytes = maxDiffPatchBytes + 1
const maxChangedFilesReportEntries = 200
const maxFileSnapshots = 30

func GenerateDiffArtifacts(ctx context.Context, workspaceRoot, findingsPath, outDir string) (DiffArtifacts, error) {
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

	isGit := IsGitWorkspace(ctx, workspaceRoot)
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
			changedFiles = listWorkspaceFilesBestEffort(workspaceRoot, maxChangedFilesReportEntries)
			if len(changedFiles) == 0 {
				reason = "not a git workspace; no changed files found in findings; workspace scan found no files"
			} else {
				reason = "not a git workspace; changed files inferred from workspace listing (best-effort)"
			}
		} else {
			reason = "not a git workspace"
		}
	}

	reportFiles := changedFiles
	if len(reportFiles) > maxChangedFilesReportEntries {
		reportFiles = reportFiles[:maxChangedFilesReportEntries]
		reason = strings.TrimSpace(reason + fmt.Sprintf("; changed_files truncated: showing %d of %d", len(reportFiles), len(changedFiles)))
	}

	if err := writeChangedFilesReport(changedFilesPath, createdAt, workspaceRoot, isGit, reason, reportFiles); err != nil {
		return DiffArtifacts{}, err
	}

	// Best-effort: snapshot file contents for UI browsing (important for worktree cleanup).
	_ = snapshotChangedFilesBestEffort(workspaceRoot, outDir, reportFiles)

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

func listWorkspaceFilesBestEffort(workspaceRoot string, maxFiles int) []string {
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	if workspaceRoot == "" || maxFiles <= 0 {
		return nil
	}

	skipDirs := map[string]struct{}{
		".git":         {},
		".oneagent":    {},
		"node_modules": {},
		"vendor":       {},
		"dist":         {},
		"build":        {},
		".next":        {},
		"coverage":     {},
	}

	var stopErr = errors.New("limit reached")

	out := make([]string, 0, min(maxFiles, 64))
	err := filepath.WalkDir(workspaceRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		rel, err := filepath.Rel(workspaceRoot, path)
		if err != nil || rel == "." {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if _, ok := skipDirs[name]; ok {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		relSlash = strings.TrimPrefix(relSlash, "./")
		relSlash = strings.TrimPrefix(relSlash, "/")
		if relSlash == "" || relSlash == "." || relSlash == ".." || strings.HasPrefix(relSlash, "../") {
			return nil
		}
		out = append(out, relSlash)
		if len(out) >= maxFiles {
			return stopErr
		}
		return nil
	})
	// Best-effort: ignore walk errors (including early-exit stopErr).
	_ = err
	sort.Strings(out)
	return out
}

func snapshotChangedFilesBestEffort(workspaceRoot, outDir string, files []string) error {
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	outDir = strings.TrimSpace(outDir)
	if workspaceRoot == "" || outDir == "" || len(files) == 0 {
		return nil
	}

	workspaceRootReal := workspaceRoot
	if resolved, err := filepath.EvalSymlinks(workspaceRoot); err == nil && strings.TrimSpace(resolved) != "" {
		workspaceRootReal = resolved
	}

	filesDir := filepath.Join(outDir, "files")
	if err := os.MkdirAll(filesDir, 0o700); err != nil {
		return err
	}

	count := 0
	for _, raw := range files {
		if count >= maxFileSnapshots {
			break
		}
		rel := normalizeRelFilePath(raw)
		if rel == "" {
			continue
		}

		src := filepath.Join(workspaceRoot, filepath.FromSlash(rel))
		if !isWithinRoot(workspaceRoot, src) {
			continue
		}
		// Avoid following symlinks that escape the workspace.
		if st, err := os.Lstat(src); err != nil || st.Mode()&os.ModeSymlink != 0 {
			continue
		}
		resolved, err := filepath.EvalSymlinks(src)
		if err != nil || !isWithinRoot(workspaceRootReal, resolved) {
			continue
		}
		info, err := os.Stat(resolved)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}

		dst := filepath.Join(filesDir, filepath.FromSlash(rel))
		if !isWithinRoot(filesDir, dst) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
			continue
		}

		in, err := os.Open(resolved)
		if err != nil {
			continue
		}
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			_ = in.Close()
			continue
		}

		_, _ = io.Copy(out, io.LimitReader(in, maxFileSnapshotBytes))
		_ = out.Close()
		_ = in.Close()
		count++
	}

	return nil
}

func normalizeRelFilePath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	canonical := strings.ReplaceAll(raw, "\\", "/")
	if strings.HasPrefix(canonical, "/") {
		return ""
	}
	canonical = strings.TrimPrefix(canonical, "./")
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(canonical)))
	clean = strings.TrimPrefix(clean, "./")
	clean = strings.TrimPrefix(clean, "/")
	if clean == "" || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return ""
	}
	return clean
}

func isWithinRoot(root, target string) bool {
	root = filepath.Clean(root)
	target = filepath.Clean(target)
	if root == target {
		return true
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
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
