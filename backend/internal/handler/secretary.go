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
		Memory:   rt.Memory,
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

	// Secretary session is a canonical permanent session per principal (best-effort).
	if strings.TrimSpace(req.SessionID) == "" {
		if sid, err := h.rt.ResolveSecretarySessionID(c.Request.Context(), userID); err == nil {
			req.SessionID = sid
		} else {
			RespondError(c, http.StatusInternalServerError, err)
			return
		}
	}

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
	SessionID        string `json:"session_id,omitempty"`
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

	if strings.TrimSpace(req.SessionID) == "" {
		if sid, err := h.rt.ResolveSecretarySessionID(c.Request.Context(), userID); err == nil {
			req.SessionID = sid
		} else {
			RespondError(c, http.StatusInternalServerError, err)
			return
		}
	}

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
		"session_id":          req.SessionID,
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

	userID := middleware.GetUserID(c)

	sessionID := strings.TrimSpace(c.Query("session_id"))
	if sessionID == "" {
		if sid, err := h.rt.ResolveSecretarySessionID(c.Request.Context(), userID); err == nil {
			sessionID = sid
		} else {
			RespondError(c, http.StatusInternalServerError, err)
			return
		}
	}
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id is required"})
		return
	}

	st, err := h.orch.GetState(c.Request.Context(), userID, sessionID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Best-effort: secretary session is "permanent". If it doesn't exist yet, create it so
			// clients can bootstrap without inventing a session id.
			if _, createErr := h.rt.Sessions.GetOrCreateSession(sessionID, userID, "assistant", "Secretary"); createErr != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
				return
			}
			st = secretary.StateResult{CursorMessageID: 0, TriageRuns: nil}
			err = nil
		}
		if err != nil {
			RespondError(c, http.StatusBadRequest, err)
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id":       sessionID,
		"cursor_message_id": st.CursorMessageID,
		"triage_runs":      st.TriageRuns,
	})
}
