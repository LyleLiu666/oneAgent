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
		Sessions:                 rt.Sessions,
		Tasks:                    rt.Tasks,
		Runner:                   rt.TaskRunner,
		Settings:                 rt.Settings,
		Memory:                   rt.Memory,
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
	// Ignore any client-provided session_id to avoid cross-module session pollution.
	sid, err := h.rt.ResolveSecretarySessionID(c.Request.Context(), userID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}
	req.SessionID = sid

	res, err := h.orch.AppendInboxMessage(c.Request.Context(), userID, req.SessionID, req.Content, req.Workspace)
	if err != nil {
		RespondError(c, http.StatusBadRequest, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id":     res.SessionID,
		"message_id":     res.MessageID,
		"ack_message_id": res.AckMessageID,
		"ack_text":       res.AckText,
	})
}

type triageRequest struct {
	SessionID       string `json:"session_id,omitempty"`
	CursorMessageID *uint  `json:"cursor_message_id,omitempty"`
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

	sid, err := h.rt.ResolveSecretarySessionID(c.Request.Context(), userID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}
	req.SessionID = sid

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
		"session_id":         req.SessionID,
		"summary_message":    res.SummaryMessage,
		"summary_message_id": res.SummaryMessageID,
		"cursor_message_id":  res.CursorMessageID,
		"created_task_ids":   res.CreatedTaskIDs,
		"canceled_task_ids":  res.CanceledTaskIDs,
		"resumed_task_ids":   res.ResumedTaskIDs,
		"questions":          res.Questions,
		"workspaces_created": res.WorkspacesCreated,
	})
}

func (h *SecretaryHandler) GetState(c *gin.Context) {
	if h == nil || h.rt == nil || h.orch == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "runtime not initialized"})
		return
	}

	userID := middleware.GetUserID(c)

	// Always use canonical secretary session; ignore any client-provided session_id.
	sessionID, err := h.rt.ResolveSecretarySessionID(c.Request.Context(), userID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
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
			if _, createErr := h.rt.Sessions.GetOrCreateSession(sessionID, userID, "secretary", "Secretary"); createErr != nil {
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
		"session_id":        sessionID,
		"cursor_message_id": st.CursorMessageID,
		"triage_runs":       st.TriageRuns,
		"recovery_focus":    st.RecoveryFocus,
	})
}

type recoveryFocusRequest struct {
	TaskID    string `json:"task_id,omitempty"`
	AttemptID string `json:"attempt_id,omitempty"`
}

func (h *SecretaryHandler) SetRecoveryFocus(c *gin.Context) {
	if h == nil || h.rt == nil || h.orch == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "runtime not initialized"})
		return
	}

	var req recoveryFocusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err)
		return
	}

	userID := middleware.GetUserID(c)

	sessionID, err := h.rt.ResolveSecretarySessionID(c.Request.Context(), userID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}
	if strings.TrimSpace(sessionID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id is required"})
		return
	}

	st, err := h.orch.SetRecoveryFocus(c.Request.Context(), userID, sessionID, req.TaskID, req.AttemptID)
	if err != nil {
		RespondError(c, http.StatusBadRequest, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id":     sessionID,
		"recovery_focus": st.RecoveryFocus,
	})
}

func (h *SecretaryHandler) GetSession(c *gin.Context) {
	if h == nil || h.rt == nil || h.rt.Sessions == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "runtime not initialized"})
		return
	}

	userID := middleware.GetUserID(c)
	sessionID, err := h.rt.ResolveSecretarySessionID(c.Request.Context(), userID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}
	if strings.TrimSpace(sessionID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id is required"})
		return
	}

	// Best-effort: canonical secretary session should always exist for bootstrap.
	if _, err := h.rt.Sessions.GetOrCreateSession(sessionID, userID, "secretary", "Secretary"); err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}

	session, msgs, err := h.rt.Sessions.GetSessionWithMessages(sessionID, userID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load session"})
		return
	}
	if strings.TrimSpace(session.Module) != "secretary" {
		RespondError(c, http.StatusConflict, sessionModuleMismatchError("secretary", session.Module))
		return
	}

	session.Messages = msgs
	c.JSON(http.StatusOK, session)
}

func (h *SecretaryHandler) ResetSession(c *gin.Context) {
	if h == nil || h.rt == nil || h.rt.Sessions == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "runtime not initialized"})
		return
	}

	userID := middleware.GetUserID(c)
	sessionID, err := h.rt.ResolveSecretarySessionID(c.Request.Context(), userID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id is required"})
		return
	}

	if err := h.rt.Sessions.DeleteSession(sessionID, userID); err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}

	// Recreate the canonical session so the client can immediately bootstrap with empty state.
	if _, err := h.rt.Sessions.GetOrCreateSession(sessionID, userID, "secretary", "Secretary"); err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"session_id": sessionID})
}
