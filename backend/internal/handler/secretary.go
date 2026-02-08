package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/middleware"
	"github.com/liu_y/oneAgent/backend/internal/model"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/secretary"
	"github.com/liu_y/oneAgent/backend/internal/taskqueue"
)

type SecretaryHandler struct {
	rt   *runtime.Runtime
	orch *secretary.Orchestrator

	streamManager *StreamManager

	autoTriageMu       sync.Mutex
	autoTriageTimers   map[string]*time.Timer
	autoTriageSeq      map[string]uint64
	autoTriageInflight map[string]int
	autoTriageDebounce time.Duration
}

func NewSecretaryHandler(rt *runtime.Runtime) *SecretaryHandler {
	h := &SecretaryHandler{
		rt:                 rt,
		streamManager:      NewStreamManager(),
		autoTriageTimers:   make(map[string]*time.Timer),
		autoTriageSeq:      make(map[string]uint64),
		autoTriageInflight: make(map[string]int),
		autoTriageDebounce: resolveSecretaryAutoTriageDebounce(),
	}
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

	h.autoTriageBegin(req.SessionID)
	defer h.autoTriageEnd(req.SessionID)

	res, err := h.orch.AppendInboxMessage(c.Request.Context(), userID, req.SessionID, req.Content, req.Workspace)
	if err != nil {
		RespondError(c, http.StatusBadRequest, err)
		return
	}

	h.broadcastTextInsert(req.SessionID, res.MessageID, "user", req.Content)
	h.scheduleAutoTriage(userID, req.SessionID)

	c.JSON(http.StatusOK, gin.H{
		"session_id":     res.SessionID,
		"message_id":     res.MessageID,
		"ack_message_id": res.AckMessageID,
		"ack_text":       res.AckText,
	})
}

type handoffTaskRequest struct {
	Prompt    string           `json:"prompt" binding:"required"`
	Workspace string           `json:"workspace" binding:"required"`
	ModelID   string           `json:"model_id,omitempty"`
	Limits    taskqueue.Limits `json:"limits,omitempty"`
}

func (h *SecretaryHandler) HandoffTask(c *gin.Context) {
	if h == nil || h.rt == nil || h.rt.Sessions == nil || h.orch == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "runtime not initialized"})
		return
	}

	var req handoffTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, err)
		return
	}

	userID := middleware.GetUserID(c)
	if strings.TrimSpace(userID) == "" {
		userID = "local"
	}

	sid, err := h.rt.ResolveSecretarySessionID(c.Request.Context(), userID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}

	res, err := h.orch.HandoffTaskToQueue(
		c.Request.Context(),
		userID,
		sid,
		req.Workspace,
		req.Prompt,
		req.ModelID,
		req.Limits,
	)
	if err != nil {
		RespondError(c, http.StatusBadRequest, err)
		return
	}

	h.broadcastInsert(sid, res.UserMessageID, "user", "handoff", req.Prompt)
	h.broadcastInsert(sid, res.AssistantMessageID, "assistant", "handoff", res.ReceiptText)

	c.JSON(http.StatusOK, gin.H{
		"session_id":           res.SessionID,
		"task_id":              res.TaskID,
		"user_message_id":      res.UserMessageID,
		"assistant_message_id": res.AssistantMessageID,
		"receipt_text":         res.ReceiptText,
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

func (h *SecretaryHandler) AttachSessionStream(c *gin.Context) {
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

	// Best-effort: canonical secretary session should always exist for bootstrap.
	if _, err := h.rt.Sessions.GetOrCreateSession(sessionID, userID, "secretary", "Secretary"); err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}

	sess, _, err := h.rt.Sessions.GetSessionWithMessages(sessionID, userID)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err)
		return
	}
	if strings.TrimSpace(sess.Module) != "secretary" {
		RespondError(c, http.StatusConflict, sessionModuleMismatchError("secretary", sess.Module))
		return
	}

	// SSE headers.
	c.Writer.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	c.Writer.Header().Set("Cache-Control", "no-cache, no-transform")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Streaming not supported"})
		return
	}

	c.Writer.WriteHeaderNow()
	fmt.Fprintf(c.Writer, ":%s\n\n", strings.Repeat(" ", 2048))
	flusher.Flush()

	sendSSE(c.Writer, flusher, StreamEvent{Type: "session", Data: sessionID})

	broadcaster := h.streamManager.GetOrCreate(sessionID)
	clientChan := broadcaster.Subscribe()
	defer broadcaster.Unsubscribe(clientChan)

	notify := c.Request.Context().Done()
loop:
	for {
		select {
		case <-notify:
			break loop
		case event, ok := <-clientChan:
			if !ok {
				break loop
			}
			sendSSE(c.Writer, flusher, event)
		}
	}

	sendSSE(c.Writer, flusher, StreamEvent{Type: "done", Data: ""})
}

func (h *SecretaryHandler) broadcastTextInsert(sessionID string, messageID uint, role, content string) {
	h.broadcastInsert(sessionID, messageID, role, model.MessageTypeText, content)
}

func (h *SecretaryHandler) broadcastInsert(sessionID string, messageID uint, role, msgType, content string) {
	if h == nil || h.streamManager == nil {
		return
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" || messageID == 0 {
		return
	}
	role = strings.TrimSpace(role)
	if role == "" {
		return
	}
	msgType = strings.TrimSpace(msgType)
	if msgType == "" {
		msgType = model.MessageTypeText
	}

	broadcaster := h.streamManager.GetOrCreate(sessionID)
	broadcastMsg(broadcaster, streamMsg{
		Op:      "insert",
		ID:      fmt.Sprintf("%d", messageID),
		Role:    role,
		MsgType: msgType,
		Delta:   content,
	})
}

func resolveSecretaryAutoTriageDebounce() time.Duration {
	const defaultDebounce = 900 * time.Millisecond

	raw := strings.TrimSpace(os.Getenv("ONEAGENT_SECRETARY_AUTOTRIAGE_DEBOUNCE_MS"))
	if raw == "" {
		return defaultDebounce
	}

	ms, err := strconv.Atoi(raw)
	if err != nil {
		return defaultDebounce
	}
	if ms <= 0 {
		return 0
	}
	return time.Duration(ms) * time.Millisecond
}

func (h *SecretaryHandler) autoTriageBegin(sessionID string) {
	if h == nil || h.autoTriageDebounce <= 0 {
		return
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return
	}

	h.autoTriageMu.Lock()
	defer h.autoTriageMu.Unlock()
	h.autoTriageInflight[sessionID]++
}

func (h *SecretaryHandler) autoTriageEnd(sessionID string) {
	if h == nil || h.autoTriageDebounce <= 0 {
		return
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return
	}

	h.autoTriageMu.Lock()
	defer h.autoTriageMu.Unlock()

	n := h.autoTriageInflight[sessionID]
	if n <= 1 {
		delete(h.autoTriageInflight, sessionID)
		return
	}
	h.autoTriageInflight[sessionID] = n - 1
}

func (h *SecretaryHandler) scheduleAutoTriage(userID, sessionID string) {
	if h == nil || h.orch == nil || h.autoTriageDebounce <= 0 {
		return
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return
	}

	userID = strings.TrimSpace(userID)
	if userID == "" {
		userID = "local"
	}

	h.autoTriageMu.Lock()
	h.autoTriageSeq[sessionID]++
	gen := h.autoTriageSeq[sessionID]

	if existing := h.autoTriageTimers[sessionID]; existing != nil {
		existing.Stop()
	}

	h.autoTriageTimers[sessionID] = time.AfterFunc(h.autoTriageDebounce, func() {
		h.autoTriageMu.Lock()
		if h.autoTriageSeq[sessionID] != gen {
			h.autoTriageMu.Unlock()
			return
		}
		if h.autoTriageInflight[sessionID] > 0 {
			h.autoTriageMu.Unlock()
			h.scheduleAutoTriage(userID, sessionID)
			return
		}
		delete(h.autoTriageTimers, sessionID)
		h.autoTriageMu.Unlock()

		h.runAutoTriage(userID, sessionID)
	})
	h.autoTriageMu.Unlock()
}

func (h *SecretaryHandler) runAutoTriage(userID, sessionID string) {
	if h == nil || h.orch == nil {
		return
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		userID = "local"
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	res, err := h.orch.Triage(ctx, userID, sessionID, nil)
	if err != nil {
		return
	}
	if res.SummaryMessageID == 0 || strings.TrimSpace(res.SummaryMessage) == "" {
		return
	}
	h.broadcastTextInsert(sessionID, res.SummaryMessageID, "assistant", res.SummaryMessage)
}
