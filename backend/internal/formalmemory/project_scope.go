package formalmemory

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

var projectScopeIDCache sync.Map

func projectScopeIDFromWorkspaceRoot(workspaceRoot string) string {
	normalized := strings.TrimSpace(workspaceRoot)
	if normalized == "" {
		return ""
	}
	normalized = filepath.Clean(normalized)
	if resolved, err := filepath.EvalSymlinks(normalized); err == nil && strings.TrimSpace(resolved) != "" {
		normalized = filepath.Clean(resolved)
	}
	if cached, ok := projectScopeIDCache.Load(normalized); ok {
		if id, ok := cached.(string); ok {
			return id
		}
	}

	id := resolveProjectScopeID(normalized)
	projectScopeIDCache.Store(normalized, id)
	return id
}

func resolveProjectScopeID(workspaceRoot string) string {
	if repoID, ok := repoScopedProjectID(workspaceRoot); ok {
		return repoID
	}
	return workspaceScopedProjectID(workspaceRoot)
}

func repoScopedProjectID(workspaceRoot string) (string, bool) {
	gitRoot, err := runGitOutput(workspaceRoot, "rev-parse", "--show-toplevel")
	if err != nil || strings.TrimSpace(gitRoot) == "" {
		return "", false
	}
	gitRoot = filepath.Clean(strings.TrimSpace(gitRoot))

	rel := "."
	if relPath, err := filepath.Rel(gitRoot, workspaceRoot); err == nil {
		relPath = filepath.Clean(relPath)
		if relPath != "" && relPath != "." {
			rel = filepath.ToSlash(relPath)
		}
	}

	seedParts := make([]string, 0, 2)
	if remote, err := runGitOutput(workspaceRoot, "remote", "get-url", "origin"); err == nil {
		if canonical := canonicalGitRemote(remote); canonical != "" {
			seedParts = append(seedParts, "remote="+canonical)
		}
	}
	if len(seedParts) == 0 {
		if rootCommit, err := runGitOutput(workspaceRoot, "rev-list", "--max-parents=0", "HEAD"); err == nil {
			rootCommit = firstNonEmptyLine(rootCommit)
			if rootCommit != "" {
				seedParts = append(seedParts, "root_commit="+rootCommit)
			}
		}
	}
	if len(seedParts) == 0 {
		return "", false
	}

	seedParts = append(seedParts, "subdir="+rel)
	sum := sha256.Sum256([]byte(strings.Join(seedParts, "\n")))
	return "repo:" + hex.EncodeToString(sum[:]), true
}

func workspaceScopedProjectID(workspaceRoot string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(workspaceRoot)))
	return "workspace:" + hex.EncodeToString(sum[:])
}

func runGitOutput(workspaceRoot string, args ...string) (string, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", err
	}
	cmdArgs := make([]string, 0, len(args)+2)
	cmdArgs = append(cmdArgs, "-C", workspaceRoot)
	cmdArgs = append(cmdArgs, args...)
	out, err := exec.Command("git", cmdArgs...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func canonicalGitRemote(raw string) string {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimSuffix(strings.TrimSuffix(trimmed, "/"), ".git")
	if trimmed == "" {
		return ""
	}

	if scpLike := canonicalSCPLikeGitRemote(trimmed); scpLike != "" {
		return scpLike
	}

	if parsed, err := url.Parse(trimmed); err == nil && strings.TrimSpace(parsed.Host) != "" {
		host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
		path := strings.Trim(strings.TrimSpace(parsed.Path), "/")
		path = strings.TrimSuffix(path, ".git")
		if host != "" && path != "" {
			return host + "/" + path
		}
	}

	if strings.Contains(trimmed, "@") && strings.Contains(trimmed, ":") {
		parts := strings.SplitN(trimmed, "@", 2)
		if len(parts) == 2 {
			hostPath := strings.SplitN(parts[1], ":", 2)
			if len(hostPath) == 2 {
				host := strings.ToLower(strings.TrimSpace(hostPath[0]))
				path := strings.Trim(strings.TrimSpace(hostPath[1]), "/")
				path = strings.TrimSuffix(path, ".git")
				if host != "" && path != "" {
					return host + "/" + path
				}
			}
		}
	}

	return ""
}

func canonicalSCPLikeGitRemote(raw string) string {
	if strings.Contains(raw, "://") {
		return ""
	}
	at := strings.LastIndex(raw, "@")
	colon := strings.LastIndex(raw, ":")
	if colon <= at {
		return ""
	}

	host := strings.TrimSpace(raw[:colon])
	if at >= 0 {
		host = host[at+1:]
	}
	path := strings.Trim(strings.TrimSpace(raw[colon+1:]), "/")
	path = strings.TrimSuffix(path, ".git")
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" || path == "" {
		return ""
	}
	return host + "/" + path
}

func firstNonEmptyLine(raw string) string {
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}
