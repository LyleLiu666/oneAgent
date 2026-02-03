package agent

import (
	"strings"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/permissions"
	"github.com/liu_y/oneAgent/backend/internal/tool"
)

func TestAgentRuntime_PromptCacheKey_IgnoresTurnContext(t *testing.T) {
	snap := permissions.ResolveSnapshot("local", permissions.DefaultPolicy(), time.Now())

	rt, err := NewFactory().Build(BuildRequest{
		Spec: AgentSpec{
			ID:           "worker-chat",
			ToolIDs:      []string{tool.ToolIDLs},
			ToolProtocol: "",
		},
		Client:        &toolsClient{},
		PolicySnapshot: snap,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	messagesA := rt.BuildMessages(nil, "skills: a", "hi")
	messagesB := rt.BuildMessages(nil, "skills: b", "hi")

	keyA, err := llm.BuildPromptCacheKey(llm.PromptCacheKeyInput{
		SessionID:    "s1",
		Epoch:        1,
		Model:        "m1",
		ToolProtocol: string(rt.ToolProtocol),
		Messages:     messagesA,
		Tools:        rt.ToolsForLLM(),
	})
	if err != nil {
		t.Fatalf("keyA: %v", err)
	}

	keyB, err := llm.BuildPromptCacheKey(llm.PromptCacheKeyInput{
		SessionID:    "s1",
		Epoch:        1,
		Model:        "m1",
		ToolProtocol: string(rt.ToolProtocol),
		Messages:     messagesB,
		Tools:        rt.ToolsForLLM(),
	})
	if err != nil {
		t.Fatalf("keyB: %v", err)
	}

	if keyA != keyB {
		t.Fatalf("expected cache key stable across turn context; keyA=%q keyB=%q", keyA, keyB)
	}
}

func TestAgentFactory_StablePrefix_DoesNotIncludeTurnContext(t *testing.T) {
	snap := permissions.ResolveSnapshot("local", permissions.DefaultPolicy(), time.Now())

	rt, err := NewFactory().Build(BuildRequest{
		Spec: AgentSpec{
			ID:           "worker-chat",
			ToolIDs:      []string{tool.ToolIDLs},
			ToolProtocol: "",
		},
		Client:        &toolsClient{},
		PolicySnapshot: snap,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if strings.Contains(rt.StablePrefix, "TurnContext") || strings.Contains(rt.StablePrefix, "每轮变化") {
		t.Fatalf("stable prefix should not include turn context markers")
	}
}

