package handler

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/middleware"
)

type artifactContentResponse struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
}

const maxArtifactBytes = 512 * 1024

func GetTaskAttemptDiffPatch(c *gin.Context) {
	getTaskAttemptArtifact(c, "diff_patch")
}

func GetTaskAttemptChangedFiles(c *gin.Context) {
	getTaskAttemptArtifact(c, "changed_files")
}

func getTaskAttemptArtifact(c *gin.Context, kind string) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Tasks == nil {
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

	path := ""
	switch kind {
	case "diff_patch":
		path = strings.TrimSpace(attempt.DiffPatchPath)
		if path == "" && rt.Layout != nil {
			path = filepath.Join(rt.Layout.TasksDir, task.ID, "attempts", attempt.ID, "review", "diff.patch")
		}
	case "changed_files":
		path = strings.TrimSpace(attempt.ChangedFilesPath)
		if path == "" && rt.Layout != nil {
			path = filepath.Join(rt.Layout.TasksDir, task.ID, "attempts", attempt.ID, "review", "changed_files.txt")
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported artifact kind"})
		return
	}
	path = filepath.Clean(path)
	if strings.TrimSpace(path) == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "artifact not available"})
		return
	}

	content, truncated, err := readFileLimited(path, maxArtifactBytes)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{"error": "artifact not available"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, artifactContentResponse{
		Path:      path,
		Content:   content,
		Truncated: truncated,
	})
}

func readFileLimited(path string, maxBytes int64) (string, bool, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", false, errors.New("path is required")
	}
	if maxBytes <= 0 {
		maxBytes = maxArtifactBytes
	}

	f, err := os.Open(path)
	if err != nil {
		return "", false, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err == nil && info.Size() > maxBytes {
		buf := make([]byte, maxBytes)
		n, readErr := f.Read(buf)
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return "", false, readErr
		}
		return string(buf[:n]), true, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", false, err
	}
	return string(data), false, nil
}
