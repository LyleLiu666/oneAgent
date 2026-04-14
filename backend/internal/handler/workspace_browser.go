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

type workspaceBrowseEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type workspaceBrowseResponse struct {
	CurrentPath string                 `json:"current_path,omitempty"`
	RootPath    string                 `json:"root_path,omitempty"`
	ParentPath  string                 `json:"parent_path,omitempty"`
	Entries     []workspaceBrowseEntry `json:"entries"`
}

func BrowseWorkspace(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Config == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "runtime not initialized"})
		return
	}

	roots := workspaceBrowseRootsFromConfig(rt.Config)
	if len(roots) == 0 {
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
			Entries: browseRootEntries(roots),
		})
		return
	}

	currentPath, rootPath, err := resolveWorkspaceBrowsePath(requestedPath, roots)
	if err != nil {
		var publicErr *PublicError
		if errors.As(err, &publicErr) {
			RespondError(c, publicErr.Status, publicErr)
			return
		}
		RespondError(c, http.StatusBadRequest, err)
		return
	}

	entries, err := listWorkspaceBrowseEntries(currentPath, rootPath)
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
		CurrentPath: currentPath,
		RootPath:    rootPath,
		ParentPath:  workspaceBrowseParentPath(currentPath, rootPath),
		Entries:     entries,
	})
}

const workspaceBrowserUnsupportedReason = "请手动填写服务端工作区路径，或为当前服务端配置一个可浏览的目录根"

func defaultWorkspaceBrowserCapability(cfg *config.Config) workspaceBrowserCapability {
	if len(workspaceBrowseRootsFromConfig(cfg)) == 0 {
		return workspaceBrowserCapability{
			Supported: false,
			Reason:    workspaceBrowserUnsupportedReason,
		}
	}
	return workspaceBrowserCapability{Supported: true}
}

func workspaceBrowseRootsFromConfig(cfg *config.Config) []string {
	if cfg == nil {
		return nil
	}

	candidates := make([]string, 0, 3)
	candidates = append(candidates, strings.TrimSpace(cfg.Home))
	candidates = append(candidates, strings.TrimSpace(cfg.DefaultWorkspace))
	if cfg.BashRootDirExplicit {
		candidates = append(candidates, strings.TrimSpace(cfg.BashRootDir))
	}

	roots := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		normalized, err := scope.NormalizeWorkspaceRoot(candidate)
		if err != nil || normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		roots = append(roots, normalized)
	}
	return roots
}

func browseRootEntries(roots []string) []workspaceBrowseEntry {
	entries := make([]workspaceBrowseEntry, 0, len(roots))
	for _, root := range roots {
		entries = append(entries, workspaceBrowseEntry{
			Name: workspaceBrowseDisplayName(root),
			Path: root,
		})
	}
	return entries
}

func resolveWorkspaceBrowsePath(requested string, roots []string) (string, string, error) {
	normalized, err := scope.NormalizeWorkspaceRoot(requested)
	if err != nil {
		return "", "", &PublicError{
			Status: http.StatusBadRequest,
			Code:   "workspace_browser_invalid_path",
			Public: "所选目录不存在、不可访问，或不是目录",
			Hint:   "请重新选择一个服务端可访问的目录",
			Err:    err,
		}
	}

	root := findWorkspaceBrowseRoot(roots, normalized)
	if root == "" {
		return "", "", &PublicError{
			Status: http.StatusForbidden,
			Code:   "workspace_browser_path_denied",
			Public: "所选目录不在当前服务端允许浏览的范围内",
			Hint:   "请从允许的根目录重新选择，或手动填写服务端工作区路径",
			Err:    scope.ErrPathOutsideWorkspace,
		}
	}

	return normalized, root, nil
}

func findWorkspaceBrowseRoot(roots []string, target string) string {
	for _, root := range roots {
		if workspaceBrowsePathWithinRoot(root, target) {
			return root
		}
	}
	return ""
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

func listWorkspaceBrowseEntries(currentPath, rootPath string) ([]workspaceBrowseEntry, error) {
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
