package handler

import (
	"database/sql"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/middleware"
)

type toolApprovalDecisionRequest struct {
	Reason string `json:"reason"`
}

func ApproveToolApproval(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Settings == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "runtime not initialized"})
		return
	}

	userID := middleware.GetUserID(c)
	if strings.TrimSpace(userID) == "" {
		userID = "local"
	}

	approvalID := strings.TrimSpace(c.Param("id"))
	if approvalID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "approval id is required"})
		return
	}

	var req toolApprovalDecisionRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if err := rt.Settings.ApproveToolApprovalForPrincipal(c.Request.Context(), userID, approvalID, req.Reason); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "approval not found"})
			return
		}
		RespondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func DenyToolApproval(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if rt == nil || rt.Settings == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "runtime not initialized"})
		return
	}

	userID := middleware.GetUserID(c)
	if strings.TrimSpace(userID) == "" {
		userID = "local"
	}

	approvalID := strings.TrimSpace(c.Param("id"))
	if approvalID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "approval id is required"})
		return
	}

	var req toolApprovalDecisionRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if err := rt.Settings.DenyToolApprovalForPrincipal(c.Request.Context(), userID, approvalID, req.Reason); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "approval not found"})
			return
		}
		RespondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
