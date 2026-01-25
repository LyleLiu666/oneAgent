package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestOpenAIClient_DowngradesPromptCacheKeyWhenUnsupported(t *testing.T) {
	var (
		mu        sync.Mutex
		requests  []map[string]any
		attempts  int
	)

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}

		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(body, &payload)

		mu.Lock()
		requests = append(requests, payload)
		attempts++
		attempt := attempts
		mu.Unlock()

		if attempt == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"unknown field: prompt_cache_key"}}`))
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"id\":\"cmpl-1\",\"choices\":[{\"delta\":{\"role\":\"assistant\",\"content\":\"OK\"},\"finish_reason\":\"stop\"}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	t.Cleanup(mock.Close)

	client := NewOpenAIClient(ClientConfig{
		Endpoint: mock.URL,
		APIKey:   "sk-test",
		Model:    "gpt-test",
	})

	opts := &ChatCompletionOptions{
		EnablePromptCache: true,
		PromptCacheKey:    "v1:session:0:hash",
	}

	var out bytes.Buffer
	err := client.ChatCompletionStream(context.Background(), []ChatMessage{
		BuildSystemMessage("sys"),
		BuildUserMessage("hi"),
	}, opts, func(chunk string) error {
		out.WriteString(chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if out.String() != "OK" {
		t.Fatalf("expected OK, got %q", out.String())
	}

	if !opts.PromptCacheDowngraded {
		t.Fatalf("expected prompt cache downgraded")
	}
	if opts.PromptCacheDowngradeReason == "" {
		t.Fatalf("expected downgrade reason")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(requests) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(requests))
	}
	if _, ok := requests[0]["prompt_cache_key"]; !ok {
		t.Fatalf("expected first request to include prompt_cache_key")
	}
	if _, ok := requests[1]["prompt_cache_key"]; ok {
		t.Fatalf("expected second request to omit prompt_cache_key")
	}
}
