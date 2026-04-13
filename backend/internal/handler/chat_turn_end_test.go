package handler

import (
	"encoding/json"
	"testing"
)

func TestBuildChatTurnEndPayload_UsesStableRefsWithoutLocalPaths(t *testing.T) {
	payload, err := buildChatTurnEndPayload(chatTurnEndInput{
		RunID:         "chat:session-1",
		TurnID:        "turn-1",
		SessionID:     "session-1",
		WorkspaceRoot: "/Users/demo/workspace",
		AgentID:       "oneagent.chat.assistant",
		ToolProtocol:  "json",
	})
	if err != nil {
		t.Fatalf("build payload: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	if decoded["turn_ref"] != "chat:session-1:turn-1" {
		t.Fatalf("unexpected turn_ref: %+v", decoded["turn_ref"])
	}
	if decoded["runlog_ref"] != "runlog:chat:session-1:turn-1" {
		t.Fatalf("unexpected runlog_ref: %+v", decoded["runlog_ref"])
	}
	if decoded["run_id"] != "chat:session-1" {
		t.Fatalf("unexpected run_id: %+v", decoded["run_id"])
	}
	if decoded["turn_id"] != "turn-1" {
		t.Fatalf("unexpected turn_id: %+v", decoded["turn_id"])
	}
	if decoded["session_id"] != "session-1" {
		t.Fatalf("unexpected session_id: %+v", decoded["session_id"])
	}
	if decoded["tool_protocol"] != "json" {
		t.Fatalf("unexpected tool_protocol: %+v", decoded["tool_protocol"])
	}
	if decoded["agent_id"] != "oneagent.chat.assistant" {
		t.Fatalf("unexpected agent_id: %+v", decoded["agent_id"])
	}
	if _, ok := decoded["workspace_root"]; ok {
		t.Fatalf("workspace_root should not be persisted in turn-end payload: %+v", decoded)
	}
	if _, ok := decoded["llm_log_path_hint"]; ok {
		t.Fatalf("llm_log_path_hint should not be persisted in turn-end payload: %+v", decoded)
	}
	if _, ok := decoded["session_messages_path_hint"]; ok {
		t.Fatalf("session_messages_path_hint should not be persisted in turn-end payload: %+v", decoded)
	}
}
