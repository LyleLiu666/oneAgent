package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIClient_ChatCompletionWithTools_SanitizesToolNamesForRequestAndSchemaLookup(t *testing.T) {
	var requestBody []byte
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		requestBody = append([]byte(nil), body...)

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"id\":\"cmpl-1\",\"choices\":[{\"delta\":{\"role\":\"assistant\",\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"memory_recall\",\"arguments\":\"{}\"}}]},\"finish_reason\":\"tool_calls\"}]}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(mock.Close)

	client := NewOpenAIClient(ClientConfig{
		Endpoint: mock.URL,
		APIKey:   "sk-test",
		Model:    "gpt-test",
	})

	result, err := client.ChatCompletionWithTools(context.Background(), []ChatMessage{
		BuildUserMessage("hi"),
	}, &ChatCompletionOptions{
		Tools: []Tool{{
			Type: "function",
			Function: ToolFunction{
				Name: "memory.recall",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{"type": "string"},
					},
					"required": []string{"query"},
				},
			},
		}},
	})
	if err != nil {
		t.Fatalf("ChatCompletionWithTools: %v", err)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result.ToolCalls))
	}

	var req struct {
		Tools []struct {
			Type     string `json:"type"`
			Function struct {
				Name string `json:"name"`
			} `json:"function"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(requestBody, &req); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	if len(req.Tools) != 1 {
		t.Fatalf("expected 1 request tool, got %d", len(req.Tools))
	}
	if req.Tools[0].Function.Name != "memory_recall" {
		t.Fatalf("expected sanitized request tool name, got %q", req.Tools[0].Function.Name)
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(result.ToolCalls[0].Function.Arguments), &args); err != nil {
		t.Fatalf("unmarshal tool args: %v", err)
	}
	if _, ok := args["query"]; !ok {
		t.Fatalf("expected required field to be backfilled via sanitized schema lookup, got %#v", args)
	}
}

func TestOpenAIResponsesClient_ChatCompletionWithTools_SanitizesToolNamesForRequestAndSchemaLookup(t *testing.T) {
	var requestBody []byte
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		requestBody = append([]byte(nil), body...)

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.output_item.added\",\"item\":{\"type\":\"function_call\",\"id\":\"fc_1\",\"call_id\":\"call_1\",\"name\":\"memory_recall\",\"arguments\":\"{}\"}}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}))
	t.Cleanup(mock.Close)

	client := NewOpenAIResponsesClient(ClientConfig{
		Endpoint: mock.URL,
		APIKey:   "sk-test",
		Model:    "gpt-test",
	})

	result, err := client.ChatCompletionWithTools(context.Background(), []ChatMessage{
		BuildUserMessage("hi"),
	}, &ChatCompletionOptions{
		Tools: []Tool{{
			Type: "function",
			Function: ToolFunction{
				Name: "memory.recall",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{"type": "string"},
					},
					"required": []string{"query"},
				},
			},
		}},
	})
	if err != nil {
		t.Fatalf("ChatCompletionWithTools: %v", err)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result.ToolCalls))
	}

	var req struct {
		Tools []struct {
			Type string `json:"type"`
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(requestBody, &req); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	if len(req.Tools) != 1 {
		t.Fatalf("expected 1 request tool, got %d", len(req.Tools))
	}
	if req.Tools[0].Name != "memory_recall" {
		t.Fatalf("expected sanitized request tool name, got %q", req.Tools[0].Name)
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(result.ToolCalls[0].Function.Arguments), &args); err != nil {
		t.Fatalf("unmarshal tool args: %v", err)
	}
	if _, ok := args["query"]; !ok {
		t.Fatalf("expected required field to be backfilled via sanitized schema lookup, got %#v", args)
	}
}

func TestOpenAIClient_ChatCompletionWithTools_SanitizesToolChoiceFunctionName(t *testing.T) {
	var requestBody []byte
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		requestBody = append([]byte(nil), body...)

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"id\":\"cmpl-1\",\"choices\":[{\"delta\":{\"role\":\"assistant\",\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"memory_recall\",\"arguments\":\"{}\"}}]},\"finish_reason\":\"tool_calls\"}]}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(mock.Close)

	client := NewOpenAIClient(ClientConfig{
		Endpoint: mock.URL,
		APIKey:   "sk-test",
		Model:    "gpt-test",
	})

	_, err := client.ChatCompletionWithTools(context.Background(), []ChatMessage{
		BuildUserMessage("hi"),
	}, &ChatCompletionOptions{
		Tools: []Tool{{
			Type: "function",
			Function: ToolFunction{
				Name: "memory.recall",
				Parameters: map[string]any{
					"type": "object",
				},
			},
		}},
		ToolChoice: map[string]any{
			"type": "function",
			"function": map[string]any{
				"name": "memory.recall",
			},
		},
	})
	if err != nil {
		t.Fatalf("ChatCompletionWithTools: %v", err)
	}

	var req struct {
		ToolChoice struct {
			Type     string `json:"type"`
			Function struct {
				Name string `json:"name"`
			} `json:"function"`
		} `json:"tool_choice"`
	}
	if err := json.Unmarshal(requestBody, &req); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	if req.ToolChoice.Function.Name != "memory_recall" {
		t.Fatalf("expected sanitized tool_choice.function.name, got %q", req.ToolChoice.Function.Name)
	}
}

func TestOpenAIResponsesClient_ChatCompletionWithTools_SanitizesToolChoiceFunctionName(t *testing.T) {
	var requestBody []byte
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		requestBody = append([]byte(nil), body...)

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.output_item.added\",\"item\":{\"type\":\"function_call\",\"id\":\"fc_1\",\"call_id\":\"call_1\",\"name\":\"memory_recall\",\"arguments\":\"{}\"}}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}))
	t.Cleanup(mock.Close)

	client := NewOpenAIResponsesClient(ClientConfig{
		Endpoint: mock.URL,
		APIKey:   "sk-test",
		Model:    "gpt-test",
	})

	_, err := client.ChatCompletionWithTools(context.Background(), []ChatMessage{
		BuildUserMessage("hi"),
	}, &ChatCompletionOptions{
		Tools: []Tool{{
			Type: "function",
			Function: ToolFunction{
				Name: "memory.recall",
				Parameters: map[string]any{
					"type": "object",
				},
			},
		}},
		ToolChoice: map[string]any{
			"type": "function",
			"name": "memory.recall",
		},
	})
	if err != nil {
		t.Fatalf("ChatCompletionWithTools: %v", err)
	}

	var req struct {
		ToolChoice struct {
			Type string `json:"type"`
			Name string `json:"name"`
		} `json:"tool_choice"`
	}
	if err := json.Unmarshal(requestBody, &req); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	if req.ToolChoice.Name != "memory_recall" {
		t.Fatalf("expected sanitized tool_choice.name, got %q", req.ToolChoice.Name)
	}
}

func TestAnthropicClient_ChatCompletionWithTools_SanitizesToolNamesForRequestAndSchemaLookup(t *testing.T) {
	var requestBody []byte
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/messages" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		requestBody = append([]byte(nil), body...)

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"tool_use\",\"id\":\"toolu_1\",\"name\":\"memory_recall\",\"input\":{}}}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(mock.Close)

	client := NewAnthropicClient(ClientConfig{
		Endpoint: mock.URL,
		APIKey:   "sk-test",
		Model:    "claude-test",
	})

	result, err := client.ChatCompletionWithTools(context.Background(), []ChatMessage{
		BuildUserMessage("hi"),
	}, &ChatCompletionOptions{
		Tools: []Tool{{
			Type: "function",
			Function: ToolFunction{
				Name: "memory.recall",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{"type": "string"},
					},
					"required": []string{"query"},
				},
			},
		}},
		ToolChoice: map[string]any{
			"type": "tool",
			"name": "memory.recall",
		},
	})
	if err != nil {
		t.Fatalf("ChatCompletionWithTools: %v", err)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result.ToolCalls))
	}

	var req struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
		ToolChoice struct {
			Type string `json:"type"`
			Name string `json:"name"`
		} `json:"tool_choice"`
	}
	if err := json.Unmarshal(requestBody, &req); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	if len(req.Tools) != 1 {
		t.Fatalf("expected 1 request tool, got %d", len(req.Tools))
	}
	if req.Tools[0].Name != "memory_recall" {
		t.Fatalf("expected sanitized anthropic tool name, got %q", req.Tools[0].Name)
	}
	if req.ToolChoice.Name != "memory_recall" {
		t.Fatalf("expected sanitized anthropic tool_choice.name, got %q", req.ToolChoice.Name)
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(result.ToolCalls[0].Function.Arguments), &args); err != nil {
		t.Fatalf("unmarshal tool args: %v", err)
	}
	if _, ok := args["query"]; !ok {
		t.Fatalf("expected required field to be backfilled via sanitized schema lookup, got %#v", args)
	}
}
