package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestOpenAIResponsesClient_ChatCompletionWithTools_ParsesFunctionCall(t *testing.T) {
	var (
		mu       sync.Mutex
		requests [][]byte
	)
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		requests = append(requests, body)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
  "id": "resp_1",
  "output": [
    {
      "type": "function_call",
      "call_id": "call_1",
      "name": "ls",
      "arguments": "{\"path\":\".\"}"
    }
  ]
}`))
	}))
	t.Cleanup(mock.Close)

	client := NewOpenAIResponsesClient(ClientConfig{
		Endpoint: mock.URL,
		APIKey:   "sk-test",
		Model:    "gpt-test",
	})

	opts := &ChatCompletionOptions{
		Tools: []Tool{{
			Type: "function",
			Function: ToolFunction{
				Name:        "ls",
				Description: "list files",
				Parameters: map[string]any{
					"type": "object",
				},
			},
		}},
	}

	result, err := client.ChatCompletionWithTools(
		context.Background(),
		[]ChatMessage{{Role: "system", Content: "sys"}, {Role: "user", Content: "hi"}},
		opts,
	)
	if err != nil {
		t.Fatalf("ChatCompletionWithTools: %v", err)
	}
	if result.Content != "" {
		t.Fatalf("expected empty content (tool call), got %q", result.Content)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result.ToolCalls))
	}
	if result.ToolCalls[0].ID != "call_1" {
		t.Fatalf("expected tool_call_id=call_1, got %q", result.ToolCalls[0].ID)
	}
	if result.ToolCalls[0].Function.Name != "ls" {
		t.Fatalf("expected tool name=ls, got %q", result.ToolCalls[0].Function.Name)
	}
	if result.ToolCalls[0].Function.Arguments == "" {
		t.Fatalf("expected arguments to be set")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(requests))
	}
	var parsed map[string]any
	if err := json.Unmarshal(requests[0], &parsed); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	if got, ok := parsed["stream"].(bool); !ok || got != false {
		t.Fatalf("expected stream=false, got %#v", parsed["stream"])
	}
	tools, ok := parsed["tools"].([]any)
	if !ok || len(tools) == 0 {
		t.Fatalf("expected tools to be present, got %#v", parsed["tools"])
	}
	first, _ := tools[0].(map[string]any)
	if first["type"] != "function" {
		t.Fatalf("expected tools[0].type=function, got %#v", first["type"])
	}
	if first["name"] != "ls" {
		t.Fatalf("expected tools[0].name=ls, got %#v", first["name"])
	}
	if _, hasNested := first["function"]; hasNested {
		t.Fatalf("expected responses tools to be flattened (no .function field), got %#v", first)
	}
	if instr, ok := parsed["instructions"].(string); !ok || instr == "" {
		t.Fatalf("expected instructions to be set from system message, got %#v", parsed["instructions"])
	}
}

func TestOpenAIResponsesClient_ChatCompletionWithTools_IncludesFunctionCallOutputInInput(t *testing.T) {
	var (
		mu       sync.Mutex
		requests [][]byte
	)
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		requests = append(requests, body)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
  "id": "resp_1",
  "output": [
    {
      "type": "message",
      "role": "assistant",
      "content": [{"type":"output_text","text":"ok"}]
    }
  ]
}`))
	}))
	t.Cleanup(mock.Close)

	client := NewOpenAIResponsesClient(ClientConfig{
		Endpoint: mock.URL,
		APIKey:   "sk-test",
		Model:    "gpt-test",
	})

	_, err := client.ChatCompletionWithTools(
		context.Background(),
		[]ChatMessage{
			{Role: "system", Content: "sys"},
			{Role: "user", Content: "hi"},
			{
				Role:    "assistant",
				Content: "",
				ToolCalls: []ToolCall{{
					ID:   "call_1",
					Type: "function",
					Function: ToolCallFunction{
						Name:      "ls",
						Arguments: `{"path":"."}`,
					},
				}},
			},
			{Role: "tool", Name: "ls", ToolCallID: "call_1", Content: `{"ok":true}`},
		},
		&ChatCompletionOptions{
			Tools: []Tool{{
				Type: "function",
				Function: ToolFunction{
					Name:        "ls",
					Description: "list files",
					Parameters: map[string]any{
						"type": "object",
					},
				},
			}},
		},
	)
	if err != nil {
		t.Fatalf("ChatCompletionWithTools: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(requests))
	}
	var parsed map[string]any
	if err := json.Unmarshal(requests[0], &parsed); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	input, ok := parsed["input"].([]any)
	if !ok {
		t.Fatalf("expected input to be a list, got %#v", parsed["input"])
	}

	var (
		sawFunctionCall       bool
		sawFunctionCallOutput bool
	)
	for _, item := range input {
		obj, _ := item.(map[string]any)
		if obj == nil {
			continue
		}
		if obj["type"] == "function_call" && obj["call_id"] == "call_1" {
			sawFunctionCall = true
		}
		if obj["type"] == "function_call_output" && obj["call_id"] == "call_1" {
			sawFunctionCallOutput = true
		}
	}
	if !sawFunctionCall {
		t.Fatalf("expected input to include function_call item for call_1, got %#v", input)
	}
	if !sawFunctionCallOutput {
		t.Fatalf("expected input to include function_call_output item for call_1, got %#v", input)
	}
}
