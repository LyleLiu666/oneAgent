package handler

import (
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/checkpoint"
	"github.com/liu_y/oneAgent/backend/internal/middleware"
	"github.com/liu_y/oneAgent/backend/internal/taskqueue"
)

func RollbackTaskAttempt(c *gin.Context) {
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
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "task id is required"})
		return
	}
	attemptID := strings.TrimSpace(c.Param("attempt_id"))
	if attemptID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "attempt id is required"})
		return
	}

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

	attempt := findAttempt(task.Attempts, attemptID)
	if attempt == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "attempt not found"})
		return
	}
	if attempt.Status == taskqueue.AttemptQueued || attempt.Status == taskqueue.AttemptRunning {
		c.JSON(http.StatusBadRequest, gin.H{"error": "attempt is not in terminal state"})
		return
	}

	checkpointPath := strings.TrimSpace(attempt.CheckpointPath)
	if checkpointPath == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "checkpoint not available"})
		return
	}

	// Idempotency.
	if attempt.RolledBackAt != nil && strings.TrimSpace(attempt.RollbackError) == "" {
		_ = rt.Tasks.AppendEvent(taskqueue.Event{
			TaskID:    task.ID,
			AttemptID: attempt.ID,
			Type:      "attempt.rollback.repeat",
			Message:   "rollback already applied",
			Data: map[string]any{
				"checkpoint_path": checkpointPath,
			},
		})
		c.JSON(http.StatusOK, gin.H{"ok": true, "idempotent": true})
		return
	}

	restoreErr := checkpoint.RestoreWorkspaceCheckpoint(c.Request.Context(), task.Workspace, checkpointPath)
	if restoreErr != nil {
		_, _ = rt.Tasks.UpdateTask(task.ID, func(tk *taskqueue.Task) error {
			a := findAttempt(tk.Attempts, attemptID)
			if a == nil {
				return nil
			}
			a.RollbackError = restoreErr.Error()
			return nil
		})
		_ = rt.Tasks.AppendEvent(taskqueue.Event{
			TaskID:    task.ID,
			AttemptID: attempt.ID,
			Type:      "attempt.rollback.failed",
			Message:   "rollback failed",
			Data: map[string]any{
				"checkpoint_path": checkpointPath,
				"error":           restoreErr.Error(),
			},
		})
		RespondError(c, http.StatusInternalServerError, restoreErr)
		return
	}

	now := time.Now()
	_, _ = rt.Tasks.UpdateTask(task.ID, func(tk *taskqueue.Task) error {
		a := findAttempt(tk.Attempts, attemptID)
		if a == nil {
			return nil
		}
		a.RolledBackAt = &now
		a.RollbackError = ""
		return nil
	})
	_ = rt.Tasks.AppendEvent(taskqueue.Event{
		TaskID:    task.ID,
		AttemptID: attempt.ID,
		Type:      "attempt.rollback.succeeded",
		Message:   "rollback succeeded",
		Data: map[string]any{
			"checkpoint_path": checkpointPath,
			"principal_id":    userID,
		},
	})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
