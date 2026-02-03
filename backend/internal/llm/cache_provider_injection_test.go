package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProviderInjection_OpenRouter_UsesCacheControl(t *testing.T) {
	var got map[string]any
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"id\":\"cmpl-test\",\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	t.Cleanup(mock.Close)

	client, err := NewClientForProvider(ProviderConfig{
		ProviderType: ProviderTypeOpenRouter,
		Endpoint:     mock.URL,
		APIKey:       "sk-test",
		Model:        "gpt-test",
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	msgs := []ChatMessage{
		{Role: "system", Content: "sys1"},
		{Role: "system", Content: "sys2"},
		{Role: "assistant", Content: "summary", ForceCacheable: true},
		{Role: "user", Content: "u1"},
		{Role: "user", Content: "turn-context", Volatile: true},
		{Role: "user", Content: "u2"},
	}
	opts := &ChatCompletionOptions{EnablePromptCache: true}

	if err := client.ChatCompletionStream(context.Background(), msgs, opts, func(string) error { return nil }); err != nil {
		t.Fatalf("chat stream: %v", err)
	}

	rawMsgs, ok := got["messages"].([]any)
	if !ok || len(rawMsgs) != len(msgs) {
		t.Fatalf("expected %d messages, got %T len=%d", len(msgs), got["messages"], len(rawMsgs))
	}

	// Selected by default selector + force cacheable: 0,1,2,3,5 (skip volatile at 4).
	cacheable := map[int]bool{0: true, 1: true, 2: true, 3: true, 5: true}
	for i, raw := range rawMsgs {
		m, _ := raw.(map[string]any)
		_, hasCacheControl := m["cache_control"]
		_, hasCachePoint := m["cachePoint"]
		if cacheable[i] && !hasCacheControl {
			t.Fatalf("expected message %d to have cache_control, got %v", i, m)
		}
		if !cacheable[i] && hasCacheControl {
			t.Fatalf("expected message %d to not have cache_control, got %v", i, m)
		}
		if hasCachePoint {
			t.Fatalf("expected no cachePoint for openrouter, got message %d %v", i, m)
		}
	}
}

func TestProviderInjection_Bedrock_UsesCachePoint(t *testing.T) {
	var got map[string]any
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"id\":\"cmpl-test\",\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	t.Cleanup(mock.Close)

	client, err := NewClientForProvider(ProviderConfig{
		ProviderType: ProviderTypeBedrock,
		Endpoint:     mock.URL,
		APIKey:       "sk-test",
		Model:        "gpt-test",
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	msgs := []ChatMessage{
		{Role: "system", Content: "sys1"},
		{Role: "system", Content: "sys2"},
		{Role: "assistant", Content: "summary", ForceCacheable: true},
		{Role: "user", Content: "u1"},
		{Role: "user", Content: "turn-context", Volatile: true},
		{Role: "user", Content: "u2"},
	}
	opts := &ChatCompletionOptions{EnablePromptCache: true}

	if err := client.ChatCompletionStream(context.Background(), msgs, opts, func(string) error { return nil }); err != nil {
		t.Fatalf("chat stream: %v", err)
	}

	rawMsgs, ok := got["messages"].([]any)
	if !ok || len(rawMsgs) != len(msgs) {
		t.Fatalf("expected %d messages, got %T len=%d", len(msgs), got["messages"], len(rawMsgs))
	}

	cacheable := map[int]bool{0: true, 1: true, 2: true, 3: true, 5: true}
	for i, raw := range rawMsgs {
		m, _ := raw.(map[string]any)
		_, hasCacheControl := m["cache_control"]
		_, hasCachePoint := m["cachePoint"]
		if cacheable[i] && !hasCachePoint {
			t.Fatalf("expected message %d to have cachePoint, got %v", i, m)
		}
		if !cacheable[i] && hasCachePoint {
			t.Fatalf("expected message %d to not have cachePoint, got %v", i, m)
		}
		if hasCacheControl {
			t.Fatalf("expected no cache_control for bedrock, got message %d %v", i, m)
		}
	}
}

func TestProviderInjection_OpenAI_UsesPromptCacheKey(t *testing.T) {
	var got map[string]any
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"id\":\"cmpl-test\",\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	t.Cleanup(mock.Close)

	client, err := NewClientForProvider(ProviderConfig{
		ProviderType: ProviderTypeOpenAI,
		Endpoint:     mock.URL,
		APIKey:       "sk-test",
		Model:        "gpt-test",
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	opts := &ChatCompletionOptions{
		EnablePromptCache: true,
		PromptCacheKey:    "v1:test",
	}
	if err := client.ChatCompletionStream(context.Background(), []ChatMessage{
		BuildSystemMessage("sys"),
		BuildUserMessage("hi"),
	}, opts, func(string) error { return nil }); err != nil {
		t.Fatalf("chat stream: %v", err)
	}

	if got["prompt_cache_key"] != "v1:test" {
		t.Fatalf("expected prompt_cache_key, got %v", got["prompt_cache_key"])
	}
	rawMsgs, ok := got["messages"].([]any)
	if !ok || len(rawMsgs) != 2 {
		t.Fatalf("expected 2 messages, got %T", got["messages"])
	}
	first := rawMsgs[0].(map[string]any)
	if _, ok := first["cache_control"]; ok {
		t.Fatalf("expected no cache_control for openai, got %v", first)
	}
	if _, ok := first["cachePoint"]; ok {
		t.Fatalf("expected no cachePoint for openai, got %v", first)
	}
}

func TestProviderInjection_Claude_SetsPromptCachingHeader(t *testing.T) {
	var gotHeader string
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/messages" {
			http.NotFound(w, r)
			return
		}
		gotHeader = r.Header.Get("anthropic-beta")
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"ok\"}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	t.Cleanup(mock.Close)

	client, err := NewClientForProvider(ProviderConfig{
		ProviderType: ProviderTypeClaude,
		Endpoint:     mock.URL,
		APIKey:       "sk-test",
		Model:        "claude-test",
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	opts := &ChatCompletionOptions{EnablePromptCache: true}
	if err := client.ChatCompletionStream(context.Background(), []ChatMessage{
		BuildSystemMessage("sys"),
		BuildUserMessage("hi"),
	}, opts, func(string) error { return nil }); err != nil {
		t.Fatalf("chat stream: %v", err)
	}
	if gotHeader != "prompt-caching" {
		t.Fatalf("expected anthropic-beta=prompt-caching, got %q", gotHeader)
	}
}
