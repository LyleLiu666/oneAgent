package llm

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	agentsdk "codeup.aliyun.com/5f3ea334769820a3e8181c1e/go/agentsdk.git"
	agentsdkprovider "codeup.aliyun.com/5f3ea334769820a3e8181c1e/go/agentsdk.git/provider"
)

func TestBuildPromptCacheKey_ChangesWhenStableSummaryChanges(t *testing.T) {
	t.Parallel()

	base := PromptCacheKeyInput{
		SessionID:    "session-1",
		Epoch:        1,
		Model:        "gpt-test",
		ToolProtocol: "json",
		Messages: []ChatMessage{
			{Role: "system", Content: "stable-system"},
			{Role: "assistant", Content: "summary-v1", ForceCacheable: true},
			{Role: "user", Content: "turn-context-a", Volatile: true},
		},
		Tools: []Tool{{
			Type: "function",
			Function: ToolFunction{
				Name: "echo",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"x": map[string]any{"type": "integer"},
					},
				},
			},
		}},
	}

	changedSummary := base
	changedSummary.Messages = append([]ChatMessage(nil), base.Messages...)
	changedSummary.Messages[1].Content = "summary-v2"

	baseKey, err := BuildPromptCacheKey(base)
	if err != nil {
		t.Fatalf("BuildPromptCacheKey(base): %v", err)
	}
	summaryKey, err := BuildPromptCacheKey(changedSummary)
	if err != nil {
		t.Fatalf("BuildPromptCacheKey(changedSummary): %v", err)
	}

	if baseKey == summaryKey {
		t.Fatalf("summary change should change cache key: %q", baseKey)
	}
}

func TestBuildPromptCacheKey_NormalizesAgentSDKOutputToProviderSafeLength(t *testing.T) {
	t.Parallel()

	in := PromptCacheKeyInput{
		SessionID:    "session-1",
		Epoch:        3,
		Model:        "gpt-test",
		ToolProtocol: "json",
		Messages: []ChatMessage{
			{Role: "system", Content: "stable-system"},
			{Role: "assistant", Content: "summary-v1", ForceCacheable: true},
			{Role: "user", Content: "turn-context-a", Volatile: true},
		},
		Tools: []Tool{{
			Type: "function",
			Function: ToolFunction{
				Name:        "echo",
				Description: "echo input",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"x": map[string]any{"type": "integer"},
					},
				},
			},
		}},
	}

	got, err := BuildPromptCacheKey(in)
	if err != nil {
		t.Fatalf("BuildPromptCacheKey: %v", err)
	}

	want, err := agentsdkprovider.BuildPromptCacheKey(agentsdkprovider.PromptCacheKeyInput{
		SessionID:     in.SessionID,
		PrefixVersion: in.Epoch,
		Model:         in.Model,
		ToolProtocol:  in.ToolProtocol,
		Messages: []agentsdkprovider.ChatMessage{
			{Role: "system", Content: "stable-system"},
			{
				Role:                "assistant",
				Content:             "summary-v1",
				PromptCacheBehavior: agentsdkprovider.PromptCacheBehaviorForceCacheable,
			},
			{
				Role:                "user",
				Content:             "turn-context-a",
				PromptCacheBehavior: agentsdkprovider.PromptCacheBehaviorVolatile,
			},
		},
		Tools: []agentsdkprovider.ToolSpec{{
			Name:        "echo",
			Description: "echo input",
			InputSchema: `{"properties":{"x":{"type":"integer"}},"type":"object"}`,
		}},
	})
	if err != nil {
		t.Fatalf("agentsdk BuildPromptCacheKey: %v", err)
	}

	expected := want
	if len(expected) > 64 {
		sum := sha256.Sum256([]byte(expected))
		expected = hex.EncodeToString(sum[:])
	}

	if got != expected {
		t.Fatalf("cache key mismatch: got %q want %q", got, expected)
	}
	if len(got) > 64 {
		t.Fatalf("cache key too long: len=%d key=%q", len(got), got)
	}
}

func TestCacheableMessageIndexes_MatchesAgentSDK(t *testing.T) {
	t.Parallel()

	msgs := []ChatMessage{
		{Role: "system", Content: "stable-system-1"},
		{Role: "system", Content: "turn-context", Volatile: true},
		{Role: "assistant", Content: "session-summary", ForceCacheable: true},
		{Role: "user", Content: "u1"},
		{Role: "tool", Content: `{"ok":true}`},
		{Role: "user", Content: "per-turn-note", Volatile: true},
	}

	got := CacheableMessageIndexes(msgs)
	want := agentsdkprovider.CacheableMessageIndexes([]agentsdkprovider.ChatMessage{
		{Role: "system", Content: "stable-system-1"},
		{Role: "system", Content: "turn-context", PromptCacheBehavior: agentsdkprovider.PromptCacheBehaviorVolatile},
		{Role: "assistant", Content: "session-summary", PromptCacheBehavior: agentsdkprovider.PromptCacheBehaviorForceCacheable},
		{Role: "user", Content: "u1"},
		{Role: "tool", Content: `{"ok":true}`},
		{Role: "user", Content: "per-turn-note", PromptCacheBehavior: agentsdkprovider.PromptCacheBehaviorVolatile},
	})

	if !equalIntSlices(got, want) {
		t.Fatalf("cacheable indexes mismatch: got %v want %v", got, want)
	}
}

func TestSanitizeToolArgumentsJSON_MatchesAgentSDK(t *testing.T) {
	t.Parallel()

	raw := `{"path":"."`
	got := SanitizeToolArgumentsJSON(raw)
	want := agentsdk.SanitizeToolArgumentsJSON(raw)

	if got != want {
		t.Fatalf("sanitize mismatch: got %q want %q", got, want)
	}
	if !json.Valid([]byte(got)) {
		t.Fatalf("expected valid JSON, got %q", got)
	}
}
