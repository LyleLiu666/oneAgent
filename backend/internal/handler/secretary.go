package handler

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/middleware"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/secretary"
)

type SecretaryHandler struct {
	rt   *runtime.Runtime
	orch *secretary.Orchestrator
}

func NewSecretaryHandler(rt *runtime.Runtime) *SecretaryHandler {
	h := &SecretaryHandler{rt: rt}
	if rt == nil {
		return h
	}

	// Reuse chat model resolution logic to keep behavior consistent.
	chat := NewChatHandler(rt)

	defaultPoolRoot := ""
	if rt.Layout != nil {
		defaultPoolRoot = filepath.Join(rt.Layout.OneAgentDir, "workspaces")
	}

	h.orch = &secretary.Orchestrator{
		Sessions: rt.Sessions,
		Tasks:    rt.Tasks,
		Runner:   rt.TaskRunner,
		DefaultWorkspacePoolRoot: defaultPoolRoot,
		ResolveModel: func(ctx context.Context, userID, modelID string) (llm.Client, string, error) {
			resolved, err := chat.resolveModel(ctx, userID, modelID)
			if err != nil {
				return nil, "", err
			}
			return resolved.Client, resolved.ModelID, nil
		},
	}

	return h
}

type inboxAppendRequest struct {
	SessionID string `json:"session_id,omitempty"`
	Content   string `json:"content" binding:"required"`
	Workspace string `json:"workspace,omitempty"`
}

func (h *SecretaryHandler) AppendInboxMessage(c *gin.Context) {
	if h == nil || h.rt == nil || h.rt.Sessions == nil || h.orch == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "runtime not initialized"})
		return
	}

	var req inboxAppendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err)
		return
	}

	userID := middleware.GetUserID(c)
	res, err := h.orch.AppendInboxMessage(c.Request.Context(), userID, req.SessionID, req.Content, req.Workspace)
	if err != nil {
		RespondError(c, http.StatusBadRequest, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id":      res.SessionID,
		"message_id":      res.MessageID,
		"ack_message_id":  res.AckMessageID,
		"ack_text":        res.AckText,
	})
}

type triageRequest struct {
	SessionID        string `json:"session_id" binding:"required"`
	CursorMessageID  *uint  `json:"cursor_message_id,omitempty"`
}

func (h *SecretaryHandler) Triage(c *gin.Context) {
	if h == nil || h.rt == nil || h.rt.Sessions == nil || h.orch == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "runtime not initialized"})
		return
	}

	var req triageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err)
		return
	}

	userID := middleware.GetUserID(c)
	res, err := h.orch.Triage(c.Request.Context(), userID, req.SessionID, req.CursorMessageID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
			return
		}
		RespondError(c, http.StatusBadRequest, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"summary_message":     res.SummaryMessage,
		"summary_message_id":  res.SummaryMessageID,
		"cursor_message_id":   res.CursorMessageID,
		"created_task_ids":    res.CreatedTaskIDs,
		"questions":           res.Questions,
		"workspaces_created":  res.WorkspacesCreated,
	})
}

func (h *SecretaryHandler) GetState(c *gin.Context) {
	if h == nil || h.rt == nil || h.orch == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "runtime not initialized"})
		return
	}

	sessionID := strings.TrimSpace(c.Query("session_id"))
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id is required"})
		return
	}

	userID := middleware.GetUserID(c)
	st, err := h.orch.GetState(c.Request.Context(), userID, sessionID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
			return
		}
		RespondError(c, http.StatusBadRequest, err)
		return
	}

	c.JSON(http.StatusOK, st)
}
