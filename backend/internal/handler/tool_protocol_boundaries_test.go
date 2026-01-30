package handler

import (
	"context"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/llm"
	"github.com/liu_y/oneAgent/backend/internal/tool"
)

type noToolsClient struct{}

func (c *noToolsClient) ChatCompletion(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatCompletionOptions) (string, error) {
	_ = ctx
	_ = messages
	_ = opts
	return "", nil
}

func (c *noToolsClient) ChatCompletionStream(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatCompletionOptions, callback llm.StreamCallback) error {
	_ = ctx
	_ = messages
	_ = opts
	_ = callback
	return nil
}

type toolsClient struct{ noToolsClient }

func (c *toolsClient) ChatCompletionWithTools(ctx context.Context, messages []llm.ChatMessage, opts *llm.ChatCompletionOptions) (llm.ChatCompletionResult, error) {
	_ = ctx
	_ = messages
	_ = opts
	return llm.ChatCompletionResult{}, nil
}

func TestSelectToolProtocol_DefaultsToJSONWhenProviderSupportsTools(t *testing.T) {
	defs, err := tool.Mount([]string{tool.ToolIDLs})
	if err != nil {
		t.Fatalf("mount tools: %v", err)
	}

	protocol, fellBack := selectToolProtocol("", defs, &toolsClient{})
	if protocol != "json" {
		t.Fatalf("expected protocol=json, got %q", protocol)
	}
	if fellBack {
		t.Fatalf("expected fellBack=false")
	}
}

func TestSelectToolProtocol_FallsBackToXMLWhenProviderLacksTools(t *testing.T) {
	defs, err := tool.Mount([]string{tool.ToolIDLs})
	if err != nil {
		t.Fatalf("mount tools: %v", err)
	}

	protocol, fellBack := selectToolProtocol("", defs, &noToolsClient{})
	if protocol != "xml" {
		t.Fatalf("expected protocol=xml, got %q", protocol)
	}
	if !fellBack {
		t.Fatalf("expected fellBack=true")
	}
}

func TestSelectToolProtocol_ExplicitJSONStillFallsBackWhenProviderLacksTools(t *testing.T) {
	defs, err := tool.Mount([]string{tool.ToolIDLs})
	if err != nil {
		t.Fatalf("mount tools: %v", err)
	}

	protocol, fellBack := selectToolProtocol("json", defs, &noToolsClient{})
	if protocol != "xml" {
		t.Fatalf("expected protocol=xml, got %q", protocol)
	}
	if !fellBack {
		t.Fatalf("expected fellBack=true")
	}
}
