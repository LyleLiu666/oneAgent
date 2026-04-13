package handler

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/formalmemory"
	"github.com/liu_y/oneAgent/backend/internal/tool"
)

func TestResolveChatToolSet_JSONAddsMemoryToolsButDoesNotInheritThem(t *testing.T) {
	baseDefs, err := tool.Mount([]string{tool.ToolIDRunCommand})
	if err != nil {
		t.Fatalf("mount base tools: %v", err)
	}

	memoryDefs := buildChatLocalMemoryToolDefinitions(&fakeFormalMemoryTools{})
	got := resolveChatToolSet(baseDefs, memoryDefs, "json")

	if !containsToolID(got.RuntimeToolIDs, tool.ToolIDMemoryRecall) ||
		!containsToolID(got.RuntimeToolIDs, tool.ToolIDMemoryRemember) ||
		!containsToolID(got.RuntimeToolIDs, tool.ToolIDMemoryForget) {
		t.Fatalf("expected runtime tool ids to include memory tools, got %+v", got.RuntimeToolIDs)
	}
	if containsToolID(got.InheritableToolIDs, tool.ToolIDMemoryRecall) ||
		containsToolID(got.InheritableToolIDs, tool.ToolIDMemoryRemember) ||
		containsToolID(got.InheritableToolIDs, tool.ToolIDMemoryForget) {
		t.Fatalf("expected inheritable tool ids to exclude memory tools, got %+v", got.InheritableToolIDs)
	}
	if !got.MemoryToolsActive {
		t.Fatalf("expected memory tools to be active under json")
	}
}

func TestResolveChatToolSet_XMLSkipsMemoryTools(t *testing.T) {
	baseDefs, err := tool.Mount([]string{tool.ToolIDRunCommand})
	if err != nil {
		t.Fatalf("mount base tools: %v", err)
	}

	memoryDefs := buildChatLocalMemoryToolDefinitions(&fakeFormalMemoryTools{})
	got := resolveChatToolSet(baseDefs, memoryDefs, "xml")

	if containsToolID(got.RuntimeToolIDs, tool.ToolIDMemoryRecall) ||
		containsToolID(got.RuntimeToolIDs, tool.ToolIDMemoryRemember) ||
		containsToolID(got.RuntimeToolIDs, tool.ToolIDMemoryForget) {
		t.Fatalf("expected xml runtime tool ids to exclude memory tools, got %+v", got.RuntimeToolIDs)
	}
	if got.MemoryToolsActive {
		t.Fatalf("expected memory tools inactive under xml")
	}
}

func TestToolSetRequiresWorkspace_MemoryToolsOnlyDoesNotRequireWorkspace(t *testing.T) {
	defs := buildChatLocalMemoryToolDefinitions(&fakeFormalMemoryTools{})
	if toolSetRequiresWorkspace(defs) {
		t.Fatalf("expected formal memory tools to work without workspace")
	}
}

type fakeFormalMemoryTools struct{}

func (f *fakeFormalMemoryTools) ExecuteTool(_ context.Context, req formalmemory.ToolInvokeRequest) (any, error) {
	return map[string]any{
		"name":       req.Name,
		"tool_call":  req.ToolCallID,
		"protocol":   req.Protocol,
		"has_args":   len(req.Arguments) > 0,
		"session_id": req.SessionID,
	}, nil
}

func containsToolID(ids []string, target string) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

func TestBuildChatLocalMemoryToolDefinitions_UseSDKToolNames(t *testing.T) {
	defs := buildChatLocalMemoryToolDefinitions(&fakeFormalMemoryTools{})
	want := []string{"memory.recall", "memory.remember", "memory.forget"}
	if len(defs) != len(want) {
		t.Fatalf("expected %d defs, got %d", len(want), len(defs))
	}
	for i, def := range defs {
		if def.Spec.Function.Name != want[i] {
			t.Fatalf("expected tool name %q, got %q", want[i], def.Spec.Function.Name)
		}
	}
}

func TestBuildChatLocalMemoryToolDefinitions_HandlerPassesInvocationMeta(t *testing.T) {
	svc := &capturingFormalMemoryTools{}
	defs := buildChatLocalMemoryToolDefinitions(svc)
	if len(defs) == 0 {
		t.Fatalf("expected memory tool definitions")
	}

	ctx := context.Background()
	ctx = tool.ContextWithUserID(ctx, "u1")
	ctx = tool.ContextWithSessionID(ctx, "session-1")
	ctx = tool.ContextWithWorkspace(ctx, tool.WorkspaceConfig{Enabled: true, Root: "/tmp/ws"})
	ctx = tool.ContextWithInvocationMeta(ctx, tool.InvocationMeta{
		RunID:      "chat:session-1",
		TurnID:     "turn-1",
		ToolCallID: "call-1",
		ToolName:   "memory.recall",
		Protocol:   "json",
	})

	if _, err := defs[0].Handler(ctx, json.RawMessage(`{"query":"hello","intent":"general","limit":3}`)); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if svc.last.Name != "memory.recall" || svc.last.ToolCallID != "call-1" || svc.last.Protocol != "json" {
		t.Fatalf("unexpected invocation meta: %+v", svc.last)
	}
	if svc.last.RunID != "chat:session-1" || svc.last.TurnID != "turn-1" {
		t.Fatalf("unexpected run/turn ids: %+v", svc.last)
	}
	if svc.last.SessionID != "session-1" || svc.last.UserID != "u1" || svc.last.WorkspaceRoot != "/tmp/ws" {
		t.Fatalf("unexpected host context: %+v", svc.last)
	}
}

type capturingFormalMemoryTools struct {
	last formalmemory.ToolInvokeRequest
}

func (c *capturingFormalMemoryTools) ExecuteTool(_ context.Context, req formalmemory.ToolInvokeRequest) (any, error) {
	c.last = req
	return map[string]any{"ok": true}, nil
}
