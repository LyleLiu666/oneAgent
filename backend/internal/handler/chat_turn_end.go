package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"codeup.aliyun.com/5f3ea334769820a3e8181c1e/go/memorySdk.git/core"
	"github.com/liu_y/oneAgent/backend/internal/formalmemory"
)

type formalMemoryTurnEnd interface {
	EnqueueTurnEndExtractJob(ctx context.Context, req formalmemory.TurnEndRequest) (formalmemory.TurnEndResult, error)
}

type chatTurnEndInput struct {
	FormalMemory  formalMemoryTurnEnd
	RunID         string
	TurnID        string
	UserID        string
	SessionID     string
	WorkspaceRoot string
	AgentID       string
	ToolProtocol  string
	TurnErr       error
}

func enqueueChatTurnEndFormalMemory(ctx context.Context, input chatTurnEndInput) error {
	if input.FormalMemory == nil {
		return nil
	}

	payload, err := buildChatTurnEndPayload(input)
	if err != nil {
		return err
	}

	_, err = input.FormalMemory.EnqueueTurnEndExtractJob(ctx, formalmemory.TurnEndRequest{
		RunID:         strings.TrimSpace(input.RunID),
		TurnID:        strings.TrimSpace(input.TurnID),
		UserID:        strings.TrimSpace(input.UserID),
		SessionID:     strings.TrimSpace(input.SessionID),
		WorkspaceRoot: strings.TrimSpace(input.WorkspaceRoot),
		AgentID:       strings.TrimSpace(input.AgentID),
		Payload:       payload,
	})
	return err
}

func buildChatTurnEndPayload(input chatTurnEndInput) (json.RawMessage, error) {
	sessionID := strings.TrimSpace(input.SessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("session_id is required for chat turn-end payload")
	}

	turnID := strings.TrimSpace(input.TurnID)
	if turnID == "" {
		return nil, fmt.Errorf("turn_id is required for chat turn-end payload")
	}

	runID := strings.TrimSpace(input.RunID)
	if runID == "" {
		runID = "chat:" + sessionID
	}

	toolProtocol := strings.TrimSpace(input.ToolProtocol)
	if toolProtocol == "" {
		toolProtocol = "none"
	}

	payload := map[string]any{
		"boundary_kind": string(core.BoundaryKindContextCompaction),
		"turn_ref":      fmt.Sprintf("chat:%s:%s", sessionID, turnID),
		"runlog_ref":    fmt.Sprintf("runlog:%s:%s", runID, turnID),
		"run_id":        runID,
		"turn_id":       turnID,
		"session_id":    sessionID,
		"tool_protocol": toolProtocol,
		"status":        "completed",
	}
	if strings.TrimSpace(input.AgentID) != "" {
		payload["agent_id"] = strings.TrimSpace(input.AgentID)
	}
	if input.TurnErr != nil {
		payload["status"] = "error"
		payload["error"] = strings.TrimSpace(input.TurnErr.Error())
	}

	return json.Marshal(payload)
}
