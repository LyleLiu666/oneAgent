package scope

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrWorkspaceNotSet      = errors.New("workspace is not set")
	ErrPathOutsideWorkspace = errors.New("path is outside workspace")
	ErrPathOutsideScope     = errors.New("path is outside scope")
)

func NormalizeWorkspaceRoot(root string) (string, error) {
	trimmed := strings.TrimSpace(root)
	if trimmed == "" {
		return "", nil
	}
	abs, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("resolve workspace path: %w", err)
	}
	abs = filepath.Clean(abs)

	real := abs
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		real = resolved
	}

	info, err := os.Stat(real)
	if err != nil {
		return "", fmt.Errorf("workspace not accessible: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("workspace is not a directory: %s", real)
	}

	return real, nil
}

// ResolveReadPath resolves input path for a read-only operation.
//
// - Absolute paths are allowed even if outside workspace.
// - Relative paths require workspaceRoot and must stay within it (no ".." escape).
func ResolveReadPath(workspaceRoot, input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", errors.New("path is required")
	}

	trimmed = normalizeWorkspaceAlias(trimmed)

	if filepath.IsAbs(trimmed) {
		return filepath.Clean(trimmed), nil
	}

	root := strings.TrimSpace(workspaceRoot)
	if root == "" {
		return "", ErrWorkspaceNotSet
	}

	abs, err := secureJoin(root, trimmed)
	if err != nil {
		if errors.Is(err, ErrPathOutsideWorkspace) {
			return "", ErrPathOutsideWorkspace
		}
		return "", err
	}
	if !isWithinRoot(root, abs) {
		return "", ErrPathOutsideWorkspace
	}
	return abs, nil
}

// ResolveWritePath resolves input path for a write operation and enforces:
// - workspaceRoot must be set
// - path must be within workspaceRoot (including symlink escape prevention)
// - optional writeScope patterns (glob) must match the path (relative to workspaceRoot)
func ResolveWritePath(workspaceRoot, input string, writeScope []string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", errors.New("path is required")
	}

	root := strings.TrimSpace(workspaceRoot)
	if root == "" {
		return "", ErrWorkspaceNotSet
	}

	trimmed = normalizeWorkspaceAlias(trimmed)

	var rel string
	if filepath.IsAbs(trimmed) {
		abs := filepath.Clean(trimmed)
		if isWithinRoot(root, abs) {
			got, err := filepath.Rel(root, abs)
			if err != nil {
				return "", fmt.Errorf("resolve path: %w", err)
			}
			rel = got
		} else {
			// Many agent systems assume the workspace root is `/` and emit paths like `/file.txt`.
			// Treat such paths as workspace-relative to improve real-world robustness; scope enforcement still applies.
			//
			// Example: "/long.txt" => "long.txt"
			slashed := filepath.ToSlash(trimmed)
			slashed = strings.TrimPrefix(slashed, "/")
			slashed = strings.TrimPrefix(slashed, "./")
			slashed = strings.TrimPrefix(slashed, "/")
			// Be conservative: only accept single-segment absolute paths ("/file.txt").
			// Real OS absolute paths outside the workspace MUST remain denied.
			if slashed == "" || slashed == "." || strings.Contains(slashed, "/") {
				return "", ErrPathOutsideWorkspace
			}
			rel = filepath.Clean(filepath.FromSlash(slashed))
			if rel == "." {
				return "", errors.New("path is required")
			}
			if strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
				return "", ErrPathOutsideWorkspace
			}
		}
	} else {
		rel = filepath.Clean(trimmed)
		if rel == "." {
			return "", errors.New("path is required")
		}
		if strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
			return "", ErrPathOutsideWorkspace
		}
	}

	abs, err := secureJoin(root, rel)
	if err != nil {
		if errors.Is(err, ErrPathOutsideWorkspace) {
			return "", ErrPathOutsideWorkspace
		}
		return "", err
	}

	relSlash := filepath.ToSlash(rel)
	relSlash = strings.TrimPrefix(relSlash, "./")
	relSlash = strings.TrimPrefix(relSlash, "/")

	if len(writeScope) > 0 {
		matched, err := MatchAny(writeScope, relSlash)
		if err != nil {
			return "", err
		}
		if !matched {
			return "", ErrPathOutsideScope
		}
	}

	return abs, nil
}

// normalizeWorkspaceAlias converts common “virtual workspace root” prefixes into a workspace-relative path.
//
// Many agent systems describe the workspace as `/workspace/...`. For our local runtime, the real workspace is an
// OS path, so we treat `/workspace/` and `workspace/` as aliases for “workspaceRoot/”.
//
// NOTE: This function intentionally does not accept arbitrary absolute paths; it only rewrites the `/workspace`
// alias so scope enforcement remains intact.
func normalizeWorkspaceAlias(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return path
	}

	// Be tolerant of Windows-style separators coming from models.
	canonical := strings.ReplaceAll(trimmed, "\\", "/")

	switch canonical {
	case "/workspace", "workspace":
		return "."
	}
	if strings.HasPrefix(canonical, "/workspace/") {
		return strings.TrimPrefix(canonical, "/workspace/")
	}
	if strings.HasPrefix(canonical, "workspace/") {
		return strings.TrimPrefix(canonical, "workspace/")
	}
	return trimmed
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

// secureJoin joins root and rel, preventing escaping root via ".." or symlinks.
// It does not create directories; it only resolves existing symlinks.
func secureJoin(root, rel string) (string, error) {
	cleanRel := filepath.Clean(rel)
	cleanRel = strings.TrimPrefix(cleanRel, string(filepath.Separator))
	if cleanRel == "." {
		return root, nil
	}
	if cleanRel == ".." || strings.HasPrefix(cleanRel, ".."+string(filepath.Separator)) {
		return "", ErrPathOutsideWorkspace
	}

	current := root
	parts := strings.Split(cleanRel, string(filepath.Separator))
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			return "", ErrPathOutsideWorkspace
		}

		next := filepath.Join(current, part)
		info, err := os.Lstat(next)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				current = next
				continue
			}
			return "", err
		}

		if info.Mode()&os.ModeSymlink != 0 {
			resolved, err := filepath.EvalSymlinks(next)
			if err != nil {
				return "", err
			}
			if !isWithinRoot(root, resolved) {
				return "", ErrPathOutsideWorkspace
			}
			current = resolved
			continue
		}

		current = next
	}

	if !isWithinRoot(root, current) {
		return "", ErrPathOutsideWorkspace
	}
	return current, nil
}
