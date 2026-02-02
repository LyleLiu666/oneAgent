package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/middleware"
	"github.com/liu_y/oneAgent/backend/internal/workflow"
	"github.com/liu_y/oneAgent/backend/internal/workflowexec"
)

type createWorkflowRequest struct {
	WorkspaceRoot string `json:"workspace_root"`
	Name          string `json:"name"`
}

func ListWorkflows(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Workflows == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "workflow store not initialized"})
		return
	}

	workspace := strings.TrimSpace(c.Query("workspace"))
	if workspace == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workspace is required"})
		return
	}

	list, err := rt.Workflows.ListWorkflows(c.Request.Context(), workspace)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, list)
}

func CreateWorkflow(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Workflows == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "workflow store not initialized"})
		return
	}

	var req createWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	req.WorkspaceRoot = strings.TrimSpace(req.WorkspaceRoot)
	req.Name = strings.TrimSpace(req.Name)
	if req.WorkspaceRoot == "" || req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workspace_root and name are required"})
		return
	}

	wf, err := rt.Workflows.CreateWorkflow(c.Request.Context(), req.WorkspaceRoot, req.Name)
	if err != nil {
		RespondError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, wf)
}

type renameWorkflowRequest struct {
	WorkspaceRoot string `json:"workspace_root"`
	Name          string `json:"name"`
}

func RenameWorkflow(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Workflows == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "workflow store not initialized"})
		return
	}

	workflowID := strings.TrimSpace(c.Param("id"))
	if workflowID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workflow id is required"})
		return
	}

	var req renameWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	req.WorkspaceRoot = strings.TrimSpace(req.WorkspaceRoot)
	req.Name = strings.TrimSpace(req.Name)
	if req.WorkspaceRoot == "" || req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workspace_root and name are required"})
		return
	}

	wf, err := rt.Workflows.RenameWorkflow(c.Request.Context(), req.WorkspaceRoot, workflowID, req.Name)
	if err != nil {
		RespondError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, wf)
}

func DeleteWorkflow(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Workflows == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "workflow store not initialized"})
		return
	}

	workflowID := strings.TrimSpace(c.Param("id"))
	workspace := strings.TrimSpace(c.Query("workspace"))
	if workflowID == "" || workspace == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workspace and workflow id are required"})
		return
	}

	if err := rt.Workflows.DeleteWorkflow(c.Request.Context(), workspace, workflowID); err != nil {
		RespondError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type publishWorkflowRequest struct {
	WorkspaceRoot string         `json:"workspace_root"`
	Graph         workflow.Graph `json:"graph"`
}

func PublishWorkflowVersion(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Workflows == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "workflow store not initialized"})
		return
	}

	workflowID := strings.TrimSpace(c.Param("id"))
	if workflowID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workflow id is required"})
		return
	}

	var req publishWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	req.WorkspaceRoot = strings.TrimSpace(req.WorkspaceRoot)
	if req.WorkspaceRoot == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workspace_root is required"})
		return
	}

	v, err := rt.Workflows.PublishVersion(c.Request.Context(), req.WorkspaceRoot, workflowID, req.Graph)
	if err != nil {
		RespondError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, v)
}

type createRunRequest struct {
	WorkspaceRoot string         `json:"workspace_root"`
	VersionID     string         `json:"version_id"`
	Inputs        map[string]any `json:"inputs,omitempty"`
}

func CreateWorkflowRun(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Workflows == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "workflow store not initialized"})
		return
	}

	workflowID := strings.TrimSpace(c.Param("id"))
	if workflowID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workflow id is required"})
		return
	}

	var req createRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	req.WorkspaceRoot = strings.TrimSpace(req.WorkspaceRoot)
	req.VersionID = strings.TrimSpace(req.VersionID)
	if req.WorkspaceRoot == "" || req.VersionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workspace_root and version_id are required"})
		return
	}

	run, err := rt.Workflows.CreateRun(c.Request.Context(), req.WorkspaceRoot, workflowID, req.VersionID, req.Inputs)
	if err != nil {
		RespondError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, run)
}

func GetWorkflowRun(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Workflows == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "workflow store not initialized"})
		return
	}

	workflowID := strings.TrimSpace(c.Param("id"))
	runID := strings.TrimSpace(c.Param("run_id"))
	workspace := strings.TrimSpace(c.Query("workspace"))
	if workflowID == "" || runID == "" || workspace == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workspace, workflow id, and run id are required"})
		return
	}

	run, err := rt.Workflows.GetRun(c.Request.Context(), workspace, workflowID, runID)
	if err != nil {
		RespondError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, run)
}

type executeRunRequest struct {
	WorkspaceRoot string `json:"workspace_root"`
	Concurrency   int    `json:"concurrency,omitempty"`
}

func ExecuteWorkflowRun(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Workflows == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "workflow store not initialized"})
		return
	}

	workflowID := strings.TrimSpace(c.Param("id"))
	runID := strings.TrimSpace(c.Param("run_id"))
	if workflowID == "" || runID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workflow id and run id are required"})
		return
	}

	var req executeRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	req.WorkspaceRoot = strings.TrimSpace(req.WorkspaceRoot)
	if req.WorkspaceRoot == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workspace_root is required"})
		return
	}

	principalID := strings.TrimSpace(middleware.GetUserID(c))
	if principalID == "" {
		principalID = "local"
	}

	exec, cleanup, err := workflowexec.Prepare(c.Request.Context(), rt, rt.Workflows, principalID, workflowexec.PrepareOptions{
		WorkspaceRoot: req.WorkspaceRoot,
		WorkflowID:    workflowID,
		RunID:         runID,
	})
	if err != nil {
		RespondError(c, http.StatusBadRequest, err)
		return
	}
	defer cleanup()

	run, err := rt.Workflows.RunWorkflow(c.Request.Context(), req.WorkspaceRoot, workflowID, runID, exec, workflow.RunOptions{
		Concurrency: req.Concurrency,
	})
	if err != nil {
		RespondError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, run)
}

type cancelRunRequest struct {
	WorkspaceRoot string `json:"workspace_root"`
	Reason        string `json:"reason,omitempty"`
}

func CancelWorkflowRun(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Workflows == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "workflow store not initialized"})
		return
	}

	workflowID := strings.TrimSpace(c.Param("id"))
	runID := strings.TrimSpace(c.Param("run_id"))
	if workflowID == "" || runID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workflow id and run id are required"})
		return
	}

	var req cancelRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	req.WorkspaceRoot = strings.TrimSpace(req.WorkspaceRoot)
	if req.WorkspaceRoot == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workspace_root is required"})
		return
	}

	run, err := rt.Workflows.CancelRun(c.Request.Context(), req.WorkspaceRoot, workflowID, runID, req.Reason)
	if err != nil {
		RespondError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, run)
}
