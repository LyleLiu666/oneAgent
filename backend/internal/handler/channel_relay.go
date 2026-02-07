package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/liu_y/oneAgent/backend/internal/channelrelay"
	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/middleware"
	"github.com/liu_y/oneAgent/backend/internal/model"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/secretary"
)

type ChannelRelayHandler struct {
	rt   *runtime.Runtime
	orch *secretary.Orchestrator
}

func NewChannelRelayHandler(rt *runtime.Runtime) *ChannelRelayHandler {
	h := &ChannelRelayHandler{rt: rt}
	if rt == nil {
		return h
	}

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

type channelRelayInboundRequest struct {
	Provider    string `json:"provider,omitempty"`
	PrincipalID string `json:"principal_id,omitempty"`

	ChannelID string `json:"channel_id"`
	ThreadID  string `json:"thread_id"`
	MessageID string `json:"message_id"`

	Workspace string `json:"workspace,omitempty"`
	Content   string `json:"content"`
}

func (h *ChannelRelayHandler) Inbound(c *gin.Context) {
	rt := middleware.GetRuntime(c)
	if h == nil || rt == nil || rt.Config == nil || rt.Layout == nil || h.orch == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "runtime not initialized"})
		return
	}

	secret := strings.TrimSpace(rt.Config.ChannelRelaySecret)
	if secret == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "channel relay is not configured"})
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	sigHex := strings.TrimSpace(c.GetHeader("X-OneAgent-Signature"))
	if !channelrelay.VerifySignature(secret, body, sigHex) {
		_ = channelrelay.AppendTrace(rt.Layout.TraceLogsDir, channelrelay.TraceEntry{
			Direction: "inbound",
			OK:        false,
			Error:     "invalid_signature",
		})
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid signature"})
		return
	}

	var req channelRelayInboundRequest
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	req.Provider = strings.TrimSpace(req.Provider)
	if req.Provider == "" {
		req.Provider = channelrelay.ProviderWebhookV1
	}
	req.PrincipalID = strings.TrimSpace(req.PrincipalID)
	if req.PrincipalID == "" {
		req.PrincipalID = "local"
	}
	req.ChannelID = strings.TrimSpace(req.ChannelID)
	req.ThreadID = strings.TrimSpace(req.ThreadID)
	req.MessageID = strings.TrimSpace(req.MessageID)
	req.Content = strings.TrimSpace(req.Content)

	if req.ChannelID == "" || req.ThreadID == "" || req.MessageID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "channel_id, thread_id and message_id are required"})
		return
	}
	if req.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content is required"})
		return
	}

	sessionID := channelrelay.DeriveSessionID(req.Provider, req.PrincipalID, req.ChannelID, req.ThreadID)
	idKey := channelrelay.InboundIdempotencyKey(req.Provider, req.PrincipalID, req.ChannelID, req.ThreadID, req.MessageID)
	idempotent, markErr := channelrelay.MarkInboundProcessed(rt.Layout.DataDir, idKey)
	if markErr != nil {
		RespondError(c, http.StatusInternalServerError, markErr)
		return
	}

	var inboxRes secretary.InboxAppendResult
	if !idempotent {
		traceMeta := map[string]any{
			"channel_relay": map[string]any{
				"provider":     req.Provider,
				"channel_id":   req.ChannelID,
				"thread_id":    req.ThreadID,
				"message_id":   req.MessageID,
				"principal_id": req.PrincipalID,
			},
		}

		inboxRes, err = h.orch.AppendInboxMessageWithTrace(c.Request.Context(), req.PrincipalID, sessionID, req.Content, req.Workspace, traceMeta)
		if err != nil {
			RespondError(c, http.StatusBadRequest, err)
			return
		}

		// Best-effort: persist session-scoped relay metadata for task linkage.
		if rt.Sessions != nil {
			session, _, err := rt.Sessions.GetSessionWithMessages(sessionID, req.PrincipalID)
			if err == nil {
				meta := session.Metadata
				if meta == nil {
					meta = model.JSONB{}
				}
				meta["channel_relay"] = model.JSONB{
					"provider":     req.Provider,
					"channel_id":   req.ChannelID,
					"thread_id":    req.ThreadID,
					"message_id":   req.MessageID,
					"principal_id": req.PrincipalID,
				}
				_ = rt.Sessions.UpdateSessionMetadata(sessionID, meta)
			}
		}
	}

	// Best-effort: triage after ingestion. The triage cursor makes this idempotent across retries.
	triageRes, triageErr := h.orch.Triage(c.Request.Context(), req.PrincipalID, sessionID, nil)
	triageErrorText := ""
	if triageErr != nil {
		triageErrorText = strings.TrimSpace(triageErr.Error())
	}

	_ = channelrelay.AppendTrace(rt.Layout.TraceLogsDir, channelrelay.TraceEntry{
		Direction:  "inbound",
		Provider:   req.Provider,
		Principal:  req.PrincipalID,
		ChannelID:  req.ChannelID,
		ThreadID:   req.ThreadID,
		MessageID:  req.MessageID,
		SessionID:  sessionID,
		Idempotent: idempotent,
		OK:         triageErr == nil,
		Error:      triageErrorText,
	})

	c.JSON(http.StatusOK, gin.H{
		"ok":         true,
		"idempotent": idempotent,
		"session_id": sessionID,
		"message_id": inboxRes.MessageID,
		"created_task_ids": func() []string {
			if triageErr != nil {
				return []string{}
			}
			return triageRes.CreatedTaskIDs
		}(),
		"triage_error": triageErrorText,
	})
}
