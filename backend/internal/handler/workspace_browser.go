package handler

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/middleware"
	"github.com/liu_y/oneAgent/backend/internal/scope"
)

var ErrWorkspaceBrowserNotSupported = errors.New("workspace browser not supported")

type workspaceBrowserCapability struct {
	Supported bool
	Reason    string
}

type workspaceBrowseRoot struct {
	Path           string
	CanSelectExact bool
}

type workspaceBrowsePolicy struct {
	Roots          []workspaceBrowseRoot
	HiddenPaths    map[string]struct{}
	ProtectedPaths map[string]struct{}
}

type workspaceBrowseEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type workspaceBrowseResponse struct {
	CurrentPath      string                 `json:"current_path,omitempty"`
	RootPath         string                 `json:"root_path,omitempty"`
	ParentPath       string                 `json:"parent_path,omitempty"`
	CanSelectCurrent bool                   `json:"can_select_current"`
	Entries          []workspaceBrowseEntry `json:"entries"`
}

func BrowseWorkspace(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Config == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "runtime not initialized"})
		return
	}

	policy := workspaceBrowsePolicyFromConfig(rt.Config)
	if len(policy.Roots) == 0 {
		RespondError(c, http.StatusNotImplemented, &PublicError{
			Status: http.StatusNotImplemented,
			Code:   "workspace_browser_unsupported",
			Public: "当前服务端环境没有可浏览的工作区根目录",
			Hint:   workspaceBrowserUnsupportedReason,
			Err:    ErrWorkspaceBrowserNotSupported,
		})
		return
	}

	requestedPath := strings.TrimSpace(c.Query("path"))
	if requestedPath == "" {
		c.JSON(http.StatusOK, workspaceBrowseResponse{
			Entries: browseRootEntries(policy.Roots),
		})
		return
	}

	currentPath, root, err := resolveWorkspaceBrowsePath(requestedPath, policy)
	if err != nil {
		var publicErr *PublicError
		if errors.As(err, &publicErr) {
			RespondError(c, publicErr.Status, publicErr)
			return
		}
		RespondError(c, http.StatusBadRequest, err)
		return
	}

	entries, err := listWorkspaceBrowseEntries(currentPath, root.Path, policy)
	if err != nil {
		RespondError(c, http.StatusBadRequest, &PublicError{
			Status: http.StatusBadRequest,
			Code:   "workspace_browser_invalid_path",
			Public: "所选目录不可访问，或无法读取其子目录",
			Hint:   "请重新选择一个服务端可访问的目录",
			Err:    err,
		})
		return
	}

	c.JSON(http.StatusOK, workspaceBrowseResponse{
		CurrentPath:      currentPath,
		RootPath:         root.Path,
		ParentPath:       workspaceBrowseParentPath(currentPath, root.Path),
		CanSelectCurrent: workspaceBrowseCanSelectCurrent(policy, currentPath, root),
		Entries:          entries,
	})
}

const workspaceBrowserUnsupportedReason = "请手动填写服务端工作区路径，或为当前服务端配置一个可浏览的目录根"

func defaultWorkspaceBrowserCapability(cfg *config.Config) workspaceBrowserCapability {
	if len(workspaceBrowsePolicyFromConfig(cfg).Roots) == 0 {
		return workspaceBrowserCapability{
			Supported: false,
			Reason:    workspaceBrowserUnsupportedReason,
		}
	}
	return workspaceBrowserCapability{Supported: true}
}

func workspaceBrowsePolicyFromConfig(cfg *config.Config) workspaceBrowsePolicy {
	policy := workspaceBrowsePolicy{
		HiddenPaths:    map[string]struct{}{},
		ProtectedPaths: map[string]struct{}{},
	}
	if cfg == nil {
		return policy
	}

	type candidate struct {
		Path           string
		CanSelectExact bool
	}

	candidates := make([]candidate, 0, 3)
	candidates = append(candidates, candidate{
		Path:           strings.TrimSpace(cfg.Home),
		CanSelectExact: false,
	})
	candidates = append(candidates, candidate{
		Path:           strings.TrimSpace(cfg.DefaultWorkspace),
		CanSelectExact: true,
	})
	if cfg.BashRootDirExplicit {
		candidates = append(candidates, candidate{
			Path:           strings.TrimSpace(cfg.BashRootDir),
			CanSelectExact: true,
		})
	}

	roots := make([]workspaceBrowseRoot, 0, len(candidates))
	seen := make(map[string]int, len(candidates))
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.Path) == "" {
			continue
		}
		normalized, err := scope.NormalizeWorkspaceRoot(candidate.Path)
		if err != nil || normalized == "" {
			continue
		}
		if idx, ok := seen[normalized]; ok {
			roots[idx].CanSelectExact = roots[idx].CanSelectExact || candidate.CanSelectExact
			continue
		}
		seen[normalized] = len(roots)
		roots = append(roots, workspaceBrowseRoot{
			Path:           normalized,
			CanSelectExact: candidate.CanSelectExact,
		})
	}
	policy.Roots = roots

	home, err := scope.NormalizeWorkspaceRoot(strings.TrimSpace(cfg.Home))
	if err == nil && home != "" {
		policy.ProtectedPaths[filepath.Join(home, ".oneagent")] = struct{}{}
	}

	if home != "" && cfg.BashRootDirExplicit {
		if bashRoot, err := scope.NormalizeWorkspaceRoot(strings.TrimSpace(cfg.BashRootDir)); err == nil && bashRoot != "" && bashRoot != home && workspaceBrowsePathWithinRoot(home, bashRoot) {
			// Nested infrastructure roots are still available from the root list,
			// but we hide them from the parent listing to avoid duplicate/internal-looking entries.
			policy.HiddenPaths[bashRoot] = struct{}{}
		}
	}

	return policy
}

func browseRootEntries(roots []workspaceBrowseRoot) []workspaceBrowseEntry {
	entries := make([]workspaceBrowseEntry, 0, len(roots))
	for _, root := range roots {
		entries = append(entries, workspaceBrowseEntry{
			Name: workspaceBrowseDisplayName(root.Path),
			Path: root.Path,
		})
	}
	return entries
}

func resolveWorkspaceBrowsePath(requested string, policy workspaceBrowsePolicy) (string, workspaceBrowseRoot, error) {
	normalized, err := scope.NormalizeWorkspaceRoot(requested)
	if err != nil {
		return "", workspaceBrowseRoot{}, &PublicError{
			Status: http.StatusBadRequest,
			Code:   "workspace_browser_invalid_path",
			Public: "所选目录不存在、不可访问，或不是目录",
			Hint:   "请重新选择一个服务端可访问的目录",
			Err:    err,
		}
	}

	root, ok := findWorkspaceBrowseRoot(policy.Roots, normalized)
	if !ok {
		return "", workspaceBrowseRoot{}, &PublicError{
			Status: http.StatusForbidden,
			Code:   "workspace_browser_path_denied",
			Public: "所选目录不在当前服务端允许浏览的范围内",
			Hint:   "请从允许的根目录重新选择，或手动填写服务端工作区路径",
			Err:    scope.ErrPathOutsideWorkspace,
		}
	}

	if workspaceBrowsePathProtected(policy, normalized) {
		return "", workspaceBrowseRoot{}, &PublicError{
			Status: http.StatusForbidden,
			Code:   "workspace_browser_path_denied",
			Public: "所选目录不在当前服务端允许浏览的范围内",
			Hint:   "请重新选择一个项目目录，不要使用 oneAgent 的内部状态目录",
			Err:    scope.ErrPathOutsideWorkspace,
		}
	}

	return normalized, root, nil
}

func findWorkspaceBrowseRoot(roots []workspaceBrowseRoot, target string) (workspaceBrowseRoot, bool) {
	var (
		best    workspaceBrowseRoot
		bestLen = -1
	)
	for _, root := range roots {
		if !workspaceBrowsePathWithinRoot(root.Path, target) {
			continue
		}
		if len(root.Path) > bestLen {
			best = root
			bestLen = len(root.Path)
		}
	}
	return best, bestLen >= 0
}

func workspaceBrowsePathWithinRoot(root, target string) bool {
	root = filepath.Clean(strings.TrimSpace(root))
	target = filepath.Clean(strings.TrimSpace(target))
	if root == "" || target == "" {
		return false
	}
	if root == target {
		return true
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func listWorkspaceBrowseEntries(currentPath, rootPath string, policy workspaceBrowsePolicy) ([]workspaceBrowseEntry, error) {
	dirEntries, err := os.ReadDir(currentPath)
	if err != nil {
		return nil, err
	}

	entries := make([]workspaceBrowseEntry, 0, len(dirEntries))
	for _, entry := range dirEntries {
		childPath := filepath.Join(currentPath, entry.Name())
		if entry.Type()&os.ModeSymlink != 0 {
			resolved, err := filepath.EvalSymlinks(childPath)
			if err != nil {
				continue
			}
			info, err := os.Stat(resolved)
			if err != nil || !info.IsDir() || !workspaceBrowsePathWithinRoot(rootPath, resolved) {
				continue
			}
			childPath = filepath.Clean(resolved)
		} else if !entry.IsDir() {
			continue
		}

		if workspaceBrowsePathProtected(policy, childPath) || workspaceBrowsePathHidden(policy, childPath) {
			continue
		}

		entries = append(entries, workspaceBrowseEntry{
			Name: entry.Name(),
			Path: childPath,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return entries, nil
}

func workspaceBrowsePathHidden(policy workspaceBrowsePolicy, target string) bool {
	target = filepath.Clean(strings.TrimSpace(target))
	if target == "" {
		return false
	}
	_, ok := policy.HiddenPaths[target]
	return ok
}

func workspaceBrowsePathProtected(policy workspaceBrowsePolicy, target string) bool {
	target = filepath.Clean(strings.TrimSpace(target))
	if target == "" {
		return false
	}
	for protected := range policy.ProtectedPaths {
		if workspaceBrowsePathWithinRoot(protected, target) {
			return true
		}
	}
	return false
}

func workspaceBrowseCanSelectCurrent(policy workspaceBrowsePolicy, currentPath string, root workspaceBrowseRoot) bool {
	currentPath = filepath.Clean(strings.TrimSpace(currentPath))
	if currentPath == "" || workspaceBrowsePathProtected(policy, currentPath) {
		return false
	}
	if currentPath == root.Path && !root.CanSelectExact {
		return false
	}
	return true
}

func workspaceBrowseParentPath(currentPath, rootPath string) string {
	currentPath = filepath.Clean(strings.TrimSpace(currentPath))
	rootPath = filepath.Clean(strings.TrimSpace(rootPath))
	if currentPath == "" || rootPath == "" || currentPath == rootPath {
		return ""
	}
	parent := filepath.Dir(currentPath)
	if parent == "." || parent == currentPath || !workspaceBrowsePathWithinRoot(rootPath, parent) {
		return ""
	}
	return parent
}

func workspaceBrowseDisplayName(path string) string {
	cleaned := filepath.Clean(strings.TrimSpace(path))
	if cleaned == "" {
		return ""
	}
	name := filepath.Base(cleaned)
	if name == "." || name == string(filepath.Separator) || name == "" {
		return cleaned
	}
	return name
}
