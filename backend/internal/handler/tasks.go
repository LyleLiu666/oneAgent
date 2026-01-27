package handler

import (
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/middleware"
	"github.com/liu_y/oneAgent/backend/internal/taskqueue"
)

type createTaskRequest struct {
	Workspace string          `json:"workspace"`
	Title     string          `json:"title,omitempty"`
	Prompt    string          `json:"prompt"`
	ModelID   string          `json:"model_id,omitempty"`
	Limits    taskqueue.Limits `json:"limits,omitempty"`
}

func CreateTask(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Tasks == nil || rt.TaskRunner == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "task queue not initialized"})
		return
	}

	userID := middleware.GetUserID(c)
	if strings.TrimSpace(userID) == "" {
		userID = "local"
	}

	var req createTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = deriveTaskTitle(req.Prompt)
	}

	task, err := rt.Tasks.CreateTask(userID, req.Workspace, title, req.Prompt, req.ModelID, req.Limits)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := rt.TaskRunner.Enqueue(task.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}

func ListTasks(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Tasks == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "task store not initialized"})
		return
	}

	userID := middleware.GetUserID(c)
	if strings.TrimSpace(userID) == "" {
		userID = "local"
	}

	workspace := strings.TrimSpace(c.Query("workspace"))
	tasks, err := rt.Tasks.ListTasks(userID, workspace)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tasks)
}

func GetTask(c *gin.Context) {
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
	c.JSON(http.StatusOK, task)
}

func CancelTask(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Tasks == nil || rt.TaskRunner == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "task queue not initialized"})
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

	updated, err := rt.TaskRunner.Cancel(taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, updated)
}

func ResumeTask(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Tasks == nil || rt.TaskRunner == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "task queue not initialized"})
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

	updated, err := rt.TaskRunner.Resume(taskID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, updated)
}

func GetTaskEvents(c *gin.Context) {
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

	evs, err := rt.Tasks.ReadEvents(taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, evs)
}

func deriveTaskTitle(prompt string) string {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return "Task"
	}
	r := []rune(prompt)
	if len(r) <= 60 {
		return prompt
	}
	return string(r[:60]) + "..."
}

