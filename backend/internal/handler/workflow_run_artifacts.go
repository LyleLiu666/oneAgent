package handler

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/middleware"
)

func GetWorkflowNodeArtifact(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Workflows == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "workflow store not initialized"})
		return
	}

	workflowID := strings.TrimSpace(c.Param("id"))
	runID := strings.TrimSpace(c.Param("run_id"))
	nodeID := strings.TrimSpace(c.Param("node_id"))
	kind := strings.TrimSpace(c.Param("kind"))
	workspace := strings.TrimSpace(c.Query("workspace"))
	if workflowID == "" || runID == "" || nodeID == "" || kind == "" || workspace == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workspace, workflow id, run id, node id, and kind are required"})
		return
	}

	nodeRoot := rt.Workflows.NodeRoot(workspace, workflowID, runID, nodeID)
	if strings.TrimSpace(nodeRoot) == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "artifact not available"})
		return
	}

	var path string
	switch kind {
	case "inputs":
		path = filepath.Join(nodeRoot, "inputs.json")
	case "ledger":
		path = filepath.Join(nodeRoot, "LEDGER.md")
	case "findings":
		path = filepath.Join(nodeRoot, "FINDINGS.md")
	case "trace":
		path = filepath.Join(nodeRoot, "trace.jsonl")
	case "manifest":
		path = filepath.Join(nodeRoot, "artifacts.json")
	case "changed_files":
		path = filepath.Join(nodeRoot, "changed_files.txt")
	case "diff_patch":
		path = filepath.Join(nodeRoot, "diff.patch")
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported artifact kind"})
		return
	}

	path = filepath.Clean(path)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{"error": "artifact not available"})
			return
		}
		RespondError(c, http.StatusInternalServerError, err)
		return
	}

	content, truncated, err := readFileLimited(path, maxArtifactBytes)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{"error": "artifact not available"})
			return
		}
		RespondError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, artifactContentResponse{
		Path:      path,
		Content:   content,
		Truncated: truncated,
	})
}
