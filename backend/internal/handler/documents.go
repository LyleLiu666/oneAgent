package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/middleware"
	"github.com/liu_y/oneAgent/backend/internal/scope"
	"github.com/liu_y/oneAgent/backend/internal/tool"
)

type exportDocumentRequest struct {
	Workspace    string `json:"workspace"`
	InputPath    string `json:"input_path"`
	Format       string `json:"format"`
	OutputPath   string `json:"output_path,omitempty"`
	TemplatePath string `json:"template_path,omitempty"`
}

func ExportDocument(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Settings == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "runtime not initialized"})
		return
	}

	userID := middleware.GetUserID(c)
	if strings.TrimSpace(userID) == "" {
		userID = "local"
	}

	var req exportDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	workspaceRoot := strings.TrimSpace(req.Workspace)
	if workspaceRoot == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workspace is required"})
		return
	}
	normalized, err := scope.NormalizeWorkspaceRoot(workspaceRoot)
	if err != nil {
		RespondError(c, http.StatusBadRequest, err)
		return
	}

	policySnap, err := rt.ResolveToolPolicySnapshot(c.Request.Context(), userID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}

	defs, err := tool.MountWithSnapshot([]string{tool.ToolIDDocumentExport}, policySnap)
	if err != nil || len(defs) == 0 {
		if err == nil {
			err = os.ErrNotExist
		}
		RespondError(c, http.StatusBadRequest, err)
		return
	}

	toolCtx := c.Request.Context()
	toolCtx = tool.ContextWithUserID(toolCtx, userID)
	toolCtx = tool.ContextWithPolicySnapshot(toolCtx, policySnap)
	toolCtx = tool.ContextWithWorkspace(toolCtx, tool.WorkspaceConfig{Enabled: true, Root: normalized})
	toolCtx = tool.ContextWithOCC(toolCtx, strings.TrimSpace(os.Getenv("ONEAGENT_DISABLE_OCC")) != "1")

	// Reuse the tool handler contract (JSON args).
	raw, _ := json.Marshal(map[string]any{
		"input_path":    strings.TrimSpace(req.InputPath),
		"format":        strings.TrimSpace(req.Format),
		"output_path":   strings.TrimSpace(req.OutputPath),
		"template_path": strings.TrimSpace(req.TemplatePath),
	})
	out, err := defs[0].Handler(toolCtx, raw)
	if err != nil {
		RespondError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, out)
}
