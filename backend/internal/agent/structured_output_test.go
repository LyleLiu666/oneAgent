package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/llm"
)

type structuredClient struct {
	toolResults []any
	chatResults []any

	toolIndex int
	chatIndex int
}

func (c *structuredClient) ChatCompletion(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatCompletionOptions) (string, error) {
	_ = ctx
	_ = messages
	_ = opts
	if c.chatIndex >= len(c.chatResults) {
		return "", errors.New("no more chat results")
	}
	v := c.chatResults[c.chatIndex]
	c.chatIndex++
	if err, ok := v.(error); ok {
		return "", err
	}
	return v.(string), nil
}

func (c *structuredClient) ChatCompletionStream(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatCompletionOptions, callback llm.StreamCallback) error {
	_ = ctx
	_ = messages
	_ = opts
	_ = callback
	return nil
}

func (c *structuredClient) ChatCompletionWithTools(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatCompletionOptions) (llm.ChatCompletionResult, error) {
	_ = ctx
	_ = messages
	_ = opts
	if c.toolIndex >= len(c.toolResults) {
		return llm.ChatCompletionResult{}, errors.New("no more tool results")
	}
	v := c.toolResults[c.toolIndex]
	c.toolIndex++
	if err, ok := v.(error); ok {
		return llm.ChatCompletionResult{}, err
	}
	return v.(llm.ChatCompletionResult), nil
}

type chatOnlyClient struct {
	chatResults []any
	chatIndex   int
}

func (c *chatOnlyClient) ChatCompletion(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatCompletionOptions) (string, error) {
	_ = ctx
	_ = messages
	_ = opts
	if c.chatIndex >= len(c.chatResults) {
		return "", errors.New("no more chat results")
	}
	v := c.chatResults[c.chatIndex]
	c.chatIndex++
	if err, ok := v.(error); ok {
		return "", err
	}
	return v.(string), nil
}

func (c *chatOnlyClient) ChatCompletionStream(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatCompletionOptions, callback llm.StreamCallback) error {
	_ = ctx
	_ = messages
	_ = opts
	_ = callback
	return nil
}

type testPayload struct {
	Value string `json:"value"`
}

func TestRequestStructuredOutput_PrefersToolCall(t *testing.T) {
	client := &structuredClient{
		toolResults: []any{
			llm.ChatCompletionResult{
				ToolCalls: []llm.ToolCall{
					{
						ID:   "call_1",
						Type: "function",
						Function: llm.ToolCallFunction{
							Name:      "emit_payload",
							Arguments: `{"value":"ok"}`,
						},
					},
				},
			},
		},
	}

	tool := llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name: "emit_payload",
		},
	}

	out, meta, err := RequestStructuredOutput[testPayload](context.Background(), client, nil, nil, StructuredOutputSpec[testPayload]{
		Tool:     tool,
		ToolName: "emit_payload",
		ParseToolArgs: func(raw json.RawMessage) (testPayload, error) {
			var p testPayload
			return p, json.Unmarshal(raw, &p)
		},
		ParseTags: func(text string) (testPayload, bool) {
			_ = text
			return testPayload{}, false
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Value != "ok" {
		t.Fatalf("expected value=ok, got %+v", out)
	}
	if meta.Mode != StructuredOutputModeToolCall {
		t.Fatalf("expected mode=tool_call, got %+v", meta)
	}
}

func TestRequestStructuredOutput_MatchesSanitizedToolCallName(t *testing.T) {
	client := &structuredClient{
		toolResults: []any{
			llm.ChatCompletionResult{
				ToolCalls: []llm.ToolCall{
					{
						ID:   "call_1",
						Type: "function",
						Function: llm.ToolCallFunction{
							Name:      "emit_payload",
							Arguments: `{"value":"ok"}`,
						},
					},
				},
			},
		},
	}

	tool := llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name: "emit.payload",
		},
	}

	out, meta, err := RequestStructuredOutput[testPayload](context.Background(), client, nil, nil, StructuredOutputSpec[testPayload]{
		Tool:     tool,
		ToolName: "emit.payload",
		ParseToolArgs: func(raw json.RawMessage) (testPayload, error) {
			var p testPayload
			return p, json.Unmarshal(raw, &p)
		},
		ParseTags: func(text string) (testPayload, bool) {
			_ = text
			return testPayload{}, false
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Value != "ok" {
		t.Fatalf("expected value=ok, got %+v", out)
	}
	if meta.Mode != StructuredOutputModeToolCall {
		t.Fatalf("expected mode=tool_call, got %+v", meta)
	}
}

func TestRequestStructuredOutput_FallsBackToTagsWhenToolCallErrors(t *testing.T) {
	client := &structuredClient{
		toolResults: []any{
			&llm.APIError{StatusCode: 400, Message: "invalid function arguments json string"},
		},
		chatResults: []any{
			"<payload><value>fallback</value></payload>",
		},
	}

	tool := llm.Tool{
		Type: "function",
		Function: llm.ToolFunction{
			Name: "emit_payload",
		},
	}

	out, meta, err := RequestStructuredOutput[testPayload](context.Background(), client, nil, nil, StructuredOutputSpec[testPayload]{
		Tool:     tool,
		ToolName: "emit_payload",
		ParseToolArgs: func(raw json.RawMessage) (testPayload, error) {
			var p testPayload
			return p, json.Unmarshal(raw, &p)
		},
		ParseTags: func(text string) (testPayload, bool) {
			block, ok := ExtractLatestTagBlock(text, "payload")
			if !ok {
				return testPayload{}, false
			}
			value, ok := ExtractTagValue(block, "value", []string{"value"})
			if !ok {
				return testPayload{}, false
			}
			return testPayload{Value: value}, true
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Value != "fallback" {
		t.Fatalf("expected value=fallback, got %+v", out)
	}
	if meta.Mode != StructuredOutputModeTags || !meta.FellBackFromToolCall {
		t.Fatalf("expected fallback to tags, got %+v", meta)
	}
}

func TestRequestStructuredOutput_UsesTagsWhenProviderLacksToolCalling(t *testing.T) {
	client := &chatOnlyClient{
		chatResults: []any{
			"<payload><value>tags</value></payload>",
		},
	}

	out, meta, err := RequestStructuredOutput[testPayload](context.Background(), client, nil, nil, StructuredOutputSpec[testPayload]{
		Tool:     llm.Tool{Type: "function", Function: llm.ToolFunction{Name: "emit_payload"}},
		ToolName: "emit_payload",
		ParseToolArgs: func(raw json.RawMessage) (testPayload, error) {
			var p testPayload
			return p, json.Unmarshal(raw, &p)
		},
		ParseTags: func(text string) (testPayload, bool) {
			block, ok := ExtractLatestTagBlock(text, "payload")
			if !ok {
				return testPayload{}, false
			}
			value, ok := ExtractTagValue(block, "value", []string{"value"})
			if !ok {
				return testPayload{}, false
			}
			return testPayload{Value: value}, true
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Value != "tags" {
		t.Fatalf("expected value=tags, got %+v", out)
	}
	if meta.Mode != StructuredOutputModeTags {
		t.Fatalf("expected mode=tags, got %+v", meta)
	}
}
