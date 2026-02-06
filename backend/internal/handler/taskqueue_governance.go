package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/middleware"
	"github.com/liu_y/oneAgent/backend/internal/scope"
	"github.com/liu_y/oneAgent/backend/internal/taskqueue"
)

func GetTaskQueueGovernance(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Tasks == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "task queue not initialized"})
		return
	}

	g, err := rt.Tasks.GetGovernance()
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, g)
}

func GetTaskQueueGovernanceSnapshot(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.TaskRunner == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "task runner not initialized"})
		return
	}

	snap := rt.TaskRunner.GovernanceSnapshot(time.Now())
	c.JSON(http.StatusOK, snap)
}

func UpdateTaskQueueGlobalPolicy(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Tasks == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "task queue not initialized"})
		return
	}

	var req struct {
		MaxRunningWorkspaces int `json:"max_running_workspaces"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if req.MaxRunningWorkspaces < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "max_running_workspaces must be >= 0"})
		return
	}

	g, err := rt.Tasks.UpdateGovernance(func(g *taskqueue.QueueGovernance) error {
		g.Global.MaxRunningWorkspaces = req.MaxRunningWorkspaces
		return nil
	})
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, g)
}

func UpdateTaskQueueWorkspacePolicy(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Tasks == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "task queue not initialized"})
		return
	}

	var req struct {
		Workspace string `json:"workspace"`
		Paused    *bool  `json:"paused,omitempty"`
		Priority  *int   `json:"priority,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	ws, err := scope.NormalizeWorkspaceRoot(req.Workspace)
	if err != nil || strings.TrimSpace(ws) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid workspace"})
		return
	}

	g, err := rt.Tasks.UpdateGovernance(func(g *taskqueue.QueueGovernance) error {
		if g.Workspaces == nil {
			g.Workspaces = map[string]taskqueue.WorkspacePolicy{}
		}
		p := g.Workspaces[ws]
		if req.Paused != nil {
			p.Paused = *req.Paused
		}
		if req.Priority != nil {
			p.Priority = *req.Priority
		}
		g.Workspaces[ws] = p
		return nil
	})
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, g)
}

func CreateTaskQueueSchedule(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Tasks == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "task queue not initialized"})
		return
	}

	var req struct {
		Workspace     string           `json:"workspace"`
		Title         string           `json:"title,omitempty"`
		Prompt        string           `json:"prompt"`
		ModelID       string           `json:"model_id,omitempty"`
		MisfirePolicy string           `json:"misfire_policy,omitempty"`
		EverySeconds  int              `json:"every_seconds"`
		Enabled       bool             `json:"enabled"`
		Limits        taskqueue.Limits `json:"limits,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ws, err := scope.NormalizeWorkspaceRoot(req.Workspace)
	if err != nil || strings.TrimSpace(ws) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid workspace"})
		return
	}
	if strings.TrimSpace(req.Prompt) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "prompt is required"})
		return
	}
	if req.EverySeconds <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "every_seconds must be > 0"})
		return
	}

	userID := strings.TrimSpace(middleware.GetUserID(c))
	if userID == "" {
		userID = "local"
	}

	now := time.Now().UTC()
	schedule := taskqueue.Schedule{
		ID:            taskqueue.NewID(),
		Enabled:       req.Enabled,
		UserID:        userID,
		Workspace:     ws,
		Title:         strings.TrimSpace(req.Title),
		Prompt:        strings.TrimSpace(req.Prompt),
		ModelID:       strings.TrimSpace(req.ModelID),
		Limits:        taskqueue.ResolveLimits(req.Limits),
		EverySeconds:  req.EverySeconds,
		NextRunAt:     now.Add(time.Duration(req.EverySeconds) * time.Second),
		MisfirePolicy: strings.TrimSpace(req.MisfirePolicy),
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	g, err := rt.Tasks.UpdateGovernance(func(g *taskqueue.QueueGovernance) error {
		g.Schedules = append(g.Schedules, schedule)
		return nil
	})
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, g)
}
