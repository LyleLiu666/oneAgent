package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIClient_ChatCompletionWithTools_SanitizesInvalidToolArguments(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")

		_, _ = io.WriteString(w, "data: {\"id\":\"cmpl-1\",\"choices\":[{\"delta\":{\"role\":\"assistant\",\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"ls\",\"arguments\":\"{\\\"path\\\":\\\".\\\"\"}}]},\"finish_reason\":\"tool_calls\"}]}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(mock.Close)

	client := NewOpenAIClient(ClientConfig{
		Endpoint: mock.URL,
		APIKey:   "sk-test",
		Model:    "gpt-test",
	})

	opts := &ChatCompletionOptions{
		Tools: []Tool{{
			Type: "function",
			Function: ToolFunction{
				Name: "ls",
				Parameters: map[string]any{
					"type": "object",
				},
			},
		}},
	}

	result, err := client.ChatCompletionWithTools(context.Background(), []ChatMessage{
		BuildUserMessage("hi"),
	}, opts)
	if err != nil {
		t.Fatalf("ChatCompletionWithTools: %v", err)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result.ToolCalls))
	}

	args := result.ToolCalls[0].Function.Arguments
	if strings.TrimSpace(args) == "" {
		t.Fatalf("expected tool arguments to be non-empty")
	}
	if !json.Valid([]byte(args)) {
		t.Fatalf("expected tool arguments to be valid JSON, got %q", args)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(args), &parsed); err != nil {
		t.Fatalf("expected tool arguments to be JSON object, got %q: %v", args, err)
	}
	if _, ok := parsed["_raw"]; !ok {
		t.Fatalf("expected sanitized tool arguments to include _raw, got %q", args)
	}
}
