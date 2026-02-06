package handler

import (
	"bufio"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/middleware"
)

type attemptFileEntry struct {
	Path      string `json:"path"`
	Available bool   `json:"available"`
	SizeBytes int64  `json:"size_bytes,omitempty"`
}

type listAttemptFilesResponse struct {
	Files   []attemptFileEntry `json:"files"`
	Omitted int                `json:"omitted,omitempty"`
}

func ListTaskAttemptFiles(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Tasks == nil || rt.Layout == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "task store not initialized"})
		return
	}

	userID := middleware.GetUserID(c)
	if strings.TrimSpace(userID) == "" {
		userID = "local"
	}

	taskID := strings.TrimSpace(c.Param("id"))
	task, err := rt.Tasks.GetTask(taskID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
			return
		}
		RespondError(c, http.StatusInternalServerError, err)
		return
	}
	if task.UserID != userID {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}

	attemptID := strings.TrimSpace(c.Param("attempt_id"))
	attempt := findAttempt(task.Attempts, attemptID)
	if attempt == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "attempt not found"})
		return
	}

	changedFilesPath := strings.TrimSpace(attempt.ChangedFilesPath)
	if changedFilesPath == "" {
		changedFilesPath = filepath.Join(rt.Layout.TasksDir, task.ID, "attempts", attempt.ID, "review", "changed_files.txt")
	}
	changedFilesPath = filepath.Clean(changedFilesPath)

	files, err := readChangedFilesReportPaths(changedFilesPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{"error": "changed_files not available"})
			return
		}
		RespondError(c, http.StatusInternalServerError, err)
		return
	}

	const maxFiles = 200
	omitted := 0
	if len(files) > maxFiles {
		omitted = len(files) - maxFiles
		files = files[:maxFiles]
	}

	filesDir := filepath.Join(filepath.Dir(changedFilesPath), "files")
	out := make([]attemptFileEntry, 0, len(files))
	for _, rel := range files {
		target, err := resolveSnapshotPath(filesDir, rel)
		if err != nil {
			out = append(out, attemptFileEntry{Path: rel, Available: false})
			continue
		}
		if st, err := os.Stat(target); err == nil && st.Mode().IsRegular() {
			out = append(out, attemptFileEntry{Path: rel, Available: true, SizeBytes: st.Size()})
			continue
		}
		out = append(out, attemptFileEntry{Path: rel, Available: false})
	}

	c.JSON(http.StatusOK, listAttemptFilesResponse{Files: out, Omitted: omitted})
}

func ReadTaskAttemptFileSnapshot(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Tasks == nil || rt.Layout == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "task store not initialized"})
		return
	}

	userID := middleware.GetUserID(c)
	if strings.TrimSpace(userID) == "" {
		userID = "local"
	}

	taskID := strings.TrimSpace(c.Param("id"))
	task, err := rt.Tasks.GetTask(taskID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
			return
		}
		RespondError(c, http.StatusInternalServerError, err)
		return
	}
	if task.UserID != userID {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}

	attemptID := strings.TrimSpace(c.Param("attempt_id"))
	attempt := findAttempt(task.Attempts, attemptID)
	if attempt == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "attempt not found"})
		return
	}

	rel := strings.TrimSpace(c.Query("path"))
	if rel == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
		return
	}

	changedFilesPath := strings.TrimSpace(attempt.ChangedFilesPath)
	if changedFilesPath == "" {
		changedFilesPath = filepath.Join(rt.Layout.TasksDir, task.ID, "attempts", attempt.ID, "review", "changed_files.txt")
	}
	changedFilesPath = filepath.Clean(changedFilesPath)

	files, err := readChangedFilesReportPaths(changedFilesPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{"error": "changed_files not available"})
			return
		}
		RespondError(c, http.StatusInternalServerError, err)
		return
	}

	allowed := make(map[string]struct{}, len(files))
	for _, p := range files {
		allowed[p] = struct{}{}
	}
	if _, ok := allowed[normalizeChangedFilePath(rel)]; !ok {
		// Path traversal is treated as a request error (400). Otherwise, it's simply not in the list (404).
		if isPathTraversal(rel) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path"})
		} else {
			c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		}
		return
	}

	filesDir := filepath.Join(filepath.Dir(changedFilesPath), "files")
	target, err := resolveSnapshotPath(filesDir, rel)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid path"})
		return
	}

	tail := strings.TrimSpace(c.Query("tail"))
	useTail := tail == "1" || strings.EqualFold(tail, "true")

	var content string
	var truncated bool
	var readErr error
	if useTail {
		content, truncated, readErr = readFileTailLimited(target, maxArtifactBytes)
	} else {
		content, truncated, readErr = readFileLimited(target, maxArtifactBytes)
	}
	if readErr != nil {
		if errors.Is(readErr, os.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{"error": "file not available"})
			return
		}
		RespondError(c, http.StatusInternalServerError, readErr)
		return
	}

	c.JSON(http.StatusOK, artifactContentResponse{
		Path:      target,
		Content:   content,
		Truncated: truncated,
	})
}

func readChangedFilesReportPaths(path string) ([]string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("path is required")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	seen := make(map[string]struct{}, 32)
	out := make([]string, 0, 32)

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		if line == "- (none)" || strings.EqualFold(line, "(none)") {
			continue
		}
		if strings.HasPrefix(line, "- ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "- "))
		}
		rel := normalizeChangedFilePath(line)
		if rel == "" {
			continue
		}
		if _, ok := seen[rel]; ok {
			continue
		}
		seen[rel] = struct{}{}
		out = append(out, rel)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	sort.Strings(out)
	return out, nil
}

func normalizeChangedFilePath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	canonical := strings.ReplaceAll(raw, "\\", "/")
	if strings.HasPrefix(canonical, "/") {
		return ""
	}
	canonical = strings.TrimPrefix(canonical, "./")
	canonical = strings.TrimSpace(canonical)
	if canonical == "" || canonical == "." || canonical == ".." {
		return ""
	}
	if strings.Contains(canonical, "\x00") {
		return ""
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(canonical)))
	clean = strings.TrimPrefix(clean, "./")
	clean = strings.TrimPrefix(clean, "/")
	if clean == "" || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return ""
	}
	return clean
}

func isPathTraversal(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	canonical := strings.ReplaceAll(raw, "\\", "/")
	if strings.HasPrefix(canonical, "/") {
		return true
	}
	canonical = strings.TrimPrefix(canonical, "./")
	canonical = filepath.ToSlash(filepath.Clean(filepath.FromSlash(canonical)))
	return canonical == ".." || strings.HasPrefix(canonical, "../")
}

func resolveSnapshotPath(filesDir, rel string) (string, error) {
	filesDir = strings.TrimSpace(filesDir)
	if filesDir == "" {
		return "", errors.New("filesDir is required")
	}
	relNorm := normalizeChangedFilePath(rel)
	if relNorm == "" {
		return "", errors.New("invalid path")
	}
	target := filepath.Join(filesDir, filepath.FromSlash(relNorm))
	if !isWithinRoot(filesDir, target) {
		return "", errors.New("invalid path")
	}
	return target, nil
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
