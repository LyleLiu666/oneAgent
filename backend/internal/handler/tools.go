package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/middleware"
	"github.com/liu_y/oneAgent/backend/internal/tool"
)

// ListTools returns available tool metadata.
func ListTools(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "runtime not initialized"})
		return
	}
	userID := middleware.GetUserID(c)
	snap, err := rt.ResolveToolPolicySnapshot(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tool.InfosWithSnapshot(snap))
}
