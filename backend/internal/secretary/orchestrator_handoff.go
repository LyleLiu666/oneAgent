package secretary

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/liu_y/oneAgent/backend/internal/model"
	"github.com/liu_y/oneAgent/backend/internal/scope"
	"github.com/liu_y/oneAgent/backend/internal/taskqueue"
)

const secretaryMessageTypeHandoffReceipt = "handoff"

type HandoffReceiptResult struct {
	SessionID          string
	TaskID             string
	UserMessageID      uint
	AssistantMessageID uint
	ReceiptText        string
}

func (o *Orchestrator) HandoffTaskToQueue(
	ctx context.Context,
	userID, sessionID, workspace, prompt, modelID string,
	limits taskqueue.Limits,
) (HandoffReceiptResult, error) {
	if o == nil || o.Sessions == nil {
		return HandoffReceiptResult{}, errors.New("sessions store not initialized")
	}
	if o.Tasks == nil || o.Runner == nil {
		return HandoffReceiptResult{}, errors.New("task queue not initialized")
	}

	userID = strings.TrimSpace(userID)
	if userID == "" {
		userID = "local"
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return HandoffReceiptResult{}, errors.New("sessionID is required")
	}

	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return HandoffReceiptResult{}, errors.New("prompt is required")
	}

	workspace = strings.TrimSpace(workspace)
	if workspace == "" {
		return HandoffReceiptResult{}, errors.New("workspace is required")
	}
	normalizedWorkspace, err := scope.NormalizeWorkspaceRoot(workspace)
	if err != nil {
		return HandoffReceiptResult{}, err
	}

	// Best-effort per-session mutex (avoid stomping state updates with triage).
	mu := o.lock(sessionID)
	mu.Lock()
	defer mu.Unlock()

	// Ensure secretary session exists for bootstrap.
	if _, err := o.Sessions.GetOrCreateSession(sessionID, userID, secretaryModuleSU, "Secretary"); err != nil {
		return HandoffReceiptResult{}, err
	}

	session, _, err := o.Sessions.GetSessionWithMessages(sessionID, userID)
	if err != nil {
		return HandoffReceiptResult{}, err
	}

	// Best-effort: bind workspace for consistent progress snapshots.
	meta := session.Metadata
	if meta == nil {
		meta = model.JSONB{}
	}
	if existing, ok := meta["workspace"].(string); ok {
		existing = strings.TrimSpace(existing)
		if existing != "" && existing != normalizedWorkspace {
			return HandoffReceiptResult{}, fmt.Errorf("cannot change workspace for an existing session")
		}
	}
	meta["workspace"] = normalizedWorkspace

	title := deriveTaskTitle(prompt)
	resolvedLimits := taskqueue.ResolveLimits(limits)
	task, err := o.Tasks.CreateTask(userID, normalizedWorkspace, title, prompt, modelID, resolvedLimits)
	if err != nil {
		return HandoffReceiptResult{}, err
	}

	if err := o.Runner.Enqueue(task.ID); err != nil {
		return HandoffReceiptResult{}, err
	}

	userMsg, err := o.Sessions.AppendMessage(sessionID, model.ChatMessage{
		Role:    model.MessageRoleUser,
		Type:    secretaryMessageTypeHandoffReceipt,
		Content: prompt,
	})
	if err != nil {
		return HandoffReceiptResult{}, err
	}

	receipt := fmt.Sprintf("好的，我已交给后台任务处理；产出会出现在右侧交付区。（追踪号 %s）", shortTaskID(task.ID))
	assistantMsg, err := o.Sessions.AppendMessage(sessionID, model.ChatMessage{
		Role:    model.MessageRoleAssistant,
		Type:    secretaryMessageTypeHandoffReceipt,
		Content: receipt,
	})
	if err != nil {
		return HandoffReceiptResult{}, err
	}

	state, _ := decodeState(meta)
	state.TrackedTaskIDs = appendUniqueStrings(state.TrackedTaskIDs, task.ID, 200)
	meta = upsertState(meta, state)
	if err := o.Sessions.UpdateSessionMetadata(sessionID, meta); err != nil {
		return HandoffReceiptResult{}, err
	}

	return HandoffReceiptResult{
		SessionID:          sessionID,
		TaskID:             task.ID,
		UserMessageID:      userMsg.ID,
		AssistantMessageID: assistantMsg.ID,
		ReceiptText:        receipt,
	}, nil
}

func shortTaskID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

func appendUniqueStrings(list []string, value string, max int) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return list
	}

	out := make([]string, 0, len(list)+1)
	seen := false
	for _, v := range list {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if v == value {
			seen = true
		}
		out = append(out, v)
	}
	if !seen {
		out = append(out, value)
	}

	if max > 0 && len(out) > max {
		out = out[len(out)-max:]
	}
	return out
}
