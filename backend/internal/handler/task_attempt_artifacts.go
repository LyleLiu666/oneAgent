package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/middleware"
	"github.com/liu_y/oneAgent/backend/internal/taskqueue"
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

func GetTaskAttemptArtifact(c *gin.Context) {
	kind := strings.TrimSpace(c.Param("kind"))
	getTaskAttemptArtifact(c, kind)
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

	path := ""
	switch kind {
	case "artifact_manifest":
		path = strings.TrimSpace(attempt.ArtifactManifestPath)
		if path == "" && rt.Layout != nil {
			path = filepath.Join(rt.Layout.TasksDir, task.ID, "attempts", attempt.ID, "artifact_manifest.v1.json")
		}
		path = filepath.Clean(path)
		if strings.TrimSpace(path) != "" {
			if _, err := os.Stat(path); err != nil && errors.Is(err, os.ErrNotExist) {
				// Back-compat: older attempts may not have persisted a manifest; try to write one on demand.
				isTerminal := attempt.Status != taskqueue.AttemptQueued && attempt.Status != taskqueue.AttemptRunning
				if isTerminal && rt.Tasks != nil && rt.Layout != nil {
					_, _ = rt.Tasks.UpdateTask(task.ID, func(tk *taskqueue.Task) error {
						for i := range tk.Attempts {
							if strings.TrimSpace(tk.Attempts[i].ID) != strings.TrimSpace(attempt.ID) {
								continue
							}
							if tk.Attempts[i].Status == taskqueue.AttemptQueued || tk.Attempts[i].Status == taskqueue.AttemptRunning {
								return nil
							}
							return taskqueue.EnsureArtifactManifestV1(rt.Layout.TasksDir, *tk, &tk.Attempts[i])
						}
						return nil
					})
				}
			}
		}
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
	case "review_comments":
		path = strings.TrimSpace(attempt.ReviewCommentsPath)
		if path == "" && rt.Layout != nil {
			path = filepath.Join(rt.Layout.TasksDir, task.ID, "attempts", attempt.ID, "review", "review_comments.jsonl")
		}
	case "findings":
		path = strings.TrimSpace(attempt.FindingsPath)
	case "trace":
		path = strings.TrimSpace(attempt.TraceLogPath)
	case "test_report":
		path = strings.TrimSpace(attempt.TestReportPath)
	case "project_config":
		path = strings.TrimSpace(attempt.ProjectConfigPath)
	case "copy_files_log":
		path = strings.TrimSpace(attempt.CopyFilesLogPath)
	case "setup_script_log":
		path = strings.TrimSpace(attempt.SetupScriptLogPath)
	case "test_script_log":
		path = strings.TrimSpace(attempt.TestScriptLogPath)
	case "cleanup_script_log":
		path = strings.TrimSpace(attempt.CleanupScriptLogPath)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported artifact kind"})
		return
	}
	if strings.TrimSpace(path) == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "artifact not available"})
		return
	}

	tail := strings.TrimSpace(c.Query("tail"))
	useTail := tail == "1" || strings.EqualFold(tail, "true")
	var content string
	var truncated bool
	var readErr error
	if useTail {
		content, truncated, readErr = readFileTailLimited(path, maxArtifactBytes)
	} else {
		content, truncated, readErr = readFileLimited(path, maxArtifactBytes)
	}
	if readErr != nil {
		if errors.Is(readErr, os.ErrNotExist) && kind == "artifact_manifest" {
			// Best-effort: synthesize manifest so older attempts still have an inspectable contract.
			isTerminal := attempt.Status != taskqueue.AttemptQueued && attempt.Status != taskqueue.AttemptRunning
			if !isTerminal {
				c.JSON(http.StatusNotFound, gin.H{"error": "artifact not available"})
				return
			}
			tasksDir := ""
			if rt.Layout != nil {
				tasksDir = rt.Layout.TasksDir
			}
			m := taskqueue.BuildArtifactManifestV1(tasksDir, task, *attempt)
			b, _ := json.MarshalIndent(m, "", "  ")
			c.JSON(http.StatusOK, artifactContentResponse{
				Path:      path,
				Content:   string(b) + "\n",
				Truncated: false,
			})
			return
		}
		if errors.Is(readErr, os.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{"error": "artifact not available"})
			return
		}
		RespondError(c, http.StatusInternalServerError, readErr)
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

func readFileTailLimited(path string, maxBytes int64) (string, bool, error) {
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
		offset := info.Size() - maxBytes
		if offset < 0 {
			offset = 0
		}
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return "", false, err
		}
		data, err := io.ReadAll(io.LimitReader(f, maxBytes))
		if err != nil {
			return "", false, err
		}
		return string(data), true, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", false, err
	}
	return string(data), false, nil
}
