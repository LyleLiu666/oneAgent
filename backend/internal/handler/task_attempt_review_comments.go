package handler

import (
	"bufio"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/middleware"
	"github.com/liu_y/oneAgent/backend/internal/taskqueue"
)

type reviewCommentRecord struct {
	TS          time.Time `json:"ts"`
	PrincipalID string    `json:"principal_id"`
	TaskID      string    `json:"task_id"`
	AttemptID   string    `json:"attempt_id"`
	Comment     string    `json:"comment"`
}

type postReviewCommentRequest struct {
	Comment string `json:"comment"`
}

func PostTaskAttemptReviewComment(c *gin.Context) {
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

	var req postReviewCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	comment := strings.TrimSpace(req.Comment)
	if comment == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "comment is required"})
		return
	}
	if len([]rune(comment)) > 4000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "comment is too long"})
		return
	}

	path := strings.TrimSpace(attempt.ReviewCommentsPath)
	if path == "" && rt.Layout != nil {
		path = filepath.Join(rt.Layout.TasksDir, task.ID, "attempts", attempt.ID, "review", "review_comments.jsonl")
	}
	path = filepath.Clean(path)
	if path == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "review_comments_path not available"})
		return
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}

	rec := reviewCommentRecord{
		TS:          time.Now().UTC(),
		PrincipalID: userID,
		TaskID:      task.ID,
		AttemptID:   attempt.ID,
		Comment:     comment,
	}
	line, _ := json.Marshal(rec)
	line = append(line, '\n')

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}
	_, writeErr := f.Write(line)
	closeErr := f.Close()
	if writeErr != nil {
		RespondError(c, http.StatusInternalServerError, writeErr)
		return
	}
	if closeErr != nil {
		RespondError(c, http.StatusInternalServerError, closeErr)
		return
	}

	_ = rt.Tasks.AppendEvent(taskqueue.Event{
		TaskID:    task.ID,
		AttemptID: attempt.ID,
		Type:      "attempt.review_comment.added",
		Message:   "Review comment added",
		Data: map[string]any{
			"review_comments_path": path,
		},
	})

	c.JSON(http.StatusOK, rec)
}

func ListTaskAttemptReviewComments(c *gin.Context) {
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

	path := strings.TrimSpace(attempt.ReviewCommentsPath)
	if path == "" && rt.Layout != nil {
		path = filepath.Join(rt.Layout.TasksDir, task.ID, "attempts", attempt.ID, "review", "review_comments.jsonl")
	}
	path = filepath.Clean(path)
	if path == "" {
		c.JSON(http.StatusOK, []reviewCommentRecord{})
		return
	}

	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.JSON(http.StatusOK, []reviewCommentRecord{})
			return
		}
		RespondError(c, http.StatusInternalServerError, err)
		return
	}
	defer f.Close()

	out := make([]reviewCommentRecord, 0, 16)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var rec reviewCommentRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		out = append(out, rec)
	}
	if err := scanner.Err(); err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, out)
}

func findAttempt(attempts []taskqueue.Attempt, attemptID string) *taskqueue.Attempt {
	attemptID = strings.TrimSpace(attemptID)
	if attemptID == "" {
		return nil
	}
	for i := range attempts {
		if attempts[i].ID == attemptID {
			return &attempts[i]
		}
	}
	return nil
}
