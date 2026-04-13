package handler

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/formalmemory"
)

func TestBuildChatTurnContext_AppendsFormalMemorySection(t *testing.T) {
	fake := &fakeFormalMemory{result: formalmemory.PreRecallResult{TurnContext: "【FormalMemory 预召回】\n1. hello"}}

	got := buildChatTurnContext(context.Background(), chatTurnContextInput{
		FormalMemory:  fake,
		UserID:        "u1",
		SessionID:     "session-1",
		WorkspaceRoot: "/tmp/workspace",
		AgentID:       "oneagent.chat.assistant",
		UserMessage:   "继续帮我做这件事",
	})

	if !strings.Contains(got.Content, "FormalMemory 预召回") {
		t.Fatalf("expected formal memory section, got %q", got.Content)
	}
	if len(got.TraceMessages) != 0 {
		t.Fatalf("expected no trace messages on healthy prerecall, got %+v", got.TraceMessages)
	}
	if len(fake.calls) != 1 {
		t.Fatalf("expected one prerecall call, got %d", len(fake.calls))
	}
	if fake.calls[0].UserID != "u1" || fake.calls[0].SessionID != "session-1" {
		t.Fatalf("unexpected prerecall request: %+v", fake.calls[0])
	}
}

func TestBuildChatTurnContext_FormalMemoryErrorDoesNotBreakTurn(t *testing.T) {
	fake := &fakeFormalMemory{err: errors.New("boom")}

	got := buildChatTurnContext(context.Background(), chatTurnContextInput{
		FormalMemory: fake,
		UserID:       "u1",
		SessionID:    "session-1",
		AgentID:      "oneagent.chat.assistant",
		UserMessage:  "继续帮我做这件事",
	})

	if got.Content != "" {
		t.Fatalf("expected empty turn context when prerecall fails and no other section exists, got %q", got.Content)
	}
	if len(got.TraceMessages) != 1 || got.TraceMessages[0] != formalMemoryPreRecallDegradedTrace {
		t.Fatalf("expected degraded prerecall trace message, got %+v", got.TraceMessages)
	}
}

func TestBuildChatTurnContext_FormalMemoryDegradedAddsTraceMessage(t *testing.T) {
	fake := &fakeFormalMemory{result: formalmemory.PreRecallResult{Degraded: true}}

	got := buildChatTurnContext(context.Background(), chatTurnContextInput{
		FormalMemory: fake,
		UserID:       "u1",
		SessionID:    "session-1",
		AgentID:      "oneagent.chat.assistant",
		UserMessage:  "继续帮我做这件事",
	})

	if got.Content != "" {
		t.Fatalf("expected empty turn context on degraded prerecall without items, got %q", got.Content)
	}
	if len(got.TraceMessages) != 1 || got.TraceMessages[0] != formalMemoryPreRecallDegradedTrace {
		t.Fatalf("expected degraded prerecall trace message, got %+v", got.TraceMessages)
	}
}

type fakeFormalMemory struct {
	result formalmemory.PreRecallResult
	err    error
	calls  []formalmemory.PreRecallRequest
}

func (f *fakeFormalMemory) PreRecallTurnContext(_ context.Context, req formalmemory.PreRecallRequest) (formalmemory.PreRecallResult, error) {
	f.calls = append(f.calls, req)
	if f.err != nil {
		return formalmemory.PreRecallResult{}, f.err
	}
	return f.result, nil
}
