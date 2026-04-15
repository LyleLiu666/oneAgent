package llm

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestOpenAIResponsesClient_ChatCompletionStream_EmitsTraceTokens(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\n")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\" world\"}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(mock.Close)

	client := NewOpenAIResponsesClient(ClientConfig{
		Endpoint: mock.URL,
		APIKey:   "sk-test",
		Model:    "gpt-test",
	})

	var (
		mu             sync.Mutex
		startCalled    bool
		firstTokenHits int
		tokens         []string
		completeOut    string
		completeErr    error
	)

	opts := &ChatCompletionOptions{
		Trace: &TraceCallback{
			OnStart: func(ctx context.Context, input []ChatMessage) {
				mu.Lock()
				startCalled = true
				mu.Unlock()
			},
			OnFirstToken: func(ctx context.Context) {
				mu.Lock()
				firstTokenHits++
				mu.Unlock()
			},
			OnToken: func(ctx context.Context, token string) {
				mu.Lock()
				tokens = append(tokens, token)
				mu.Unlock()
			},
			OnComplete: func(ctx context.Context, fullOutput string, err error) {
				mu.Lock()
				completeOut = fullOutput
				completeErr = err
				mu.Unlock()
			},
		},
	}

	var got strings.Builder
	if err := client.ChatCompletionStream(
		context.Background(),
		[]ChatMessage{{Role: "system", Content: "sys"}, {Role: "user", Content: "hi"}},
		opts,
		func(chunk string) error {
			got.WriteString(chunk)
			return nil
		},
	); err != nil {
		t.Fatalf("ChatCompletionStream: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if !startCalled {
		t.Fatalf("expected OnStart to be called")
	}
	if firstTokenHits != 1 {
		t.Fatalf("expected OnFirstToken to be called once, got %d", firstTokenHits)
	}
	if strings.TrimSpace(got.String()) != "hello world" {
		t.Fatalf("expected callback output %q, got %q", "hello world", got.String())
	}
	if strings.TrimSpace(completeOut) != "hello world" {
		t.Fatalf("expected OnComplete output %q, got %q", "hello world", completeOut)
	}
	if completeErr != nil {
		t.Fatalf("expected OnComplete err=nil, got %v", completeErr)
	}
	if len(tokens) != 2 {
		t.Fatalf("expected OnToken to be called twice, got %d (%#v)", len(tokens), tokens)
	}
	if tokens[0] != "hello" || tokens[1] != " world" {
		t.Fatalf("unexpected tokens: %#v", tokens)
	}
}

func TestOpenAIResponsesClient_ChatCompletionStream_EmitsUsageFromResponseCompleted(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\n")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":9021,\"output_tokens\":12,\"total_tokens\":9033,\"input_tokens_details\":{\"cached_tokens\":8832}}}}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(mock.Close)

	client := NewOpenAIResponsesClient(ClientConfig{
		Endpoint: mock.URL,
		APIKey:   "sk-test",
		Model:    "gpt-test",
	})

	var gotUsage UsageInfo
	opts := &ChatCompletionOptions{
		Trace: &TraceCallback{
			OnUsage: func(ctx context.Context, usage UsageInfo) {
				gotUsage = usage
			},
		},
	}

	if err := client.ChatCompletionStream(
		context.Background(),
		[]ChatMessage{{Role: "user", Content: "hi"}},
		opts,
		func(string) error { return nil },
	); err != nil {
		t.Fatalf("ChatCompletionStream: %v", err)
	}

	if gotUsage.InputTokens != 9021 {
		t.Fatalf("expected input_tokens=9021, got %d", gotUsage.InputTokens)
	}
	if gotUsage.OutputTokens != 12 {
		t.Fatalf("expected output_tokens=12, got %d", gotUsage.OutputTokens)
	}
	if gotUsage.TotalTokens != 9033 {
		t.Fatalf("expected total_tokens=9033, got %d", gotUsage.TotalTokens)
	}
	if gotUsage.CachedTokens != 8832 {
		t.Fatalf("expected cached_tokens=8832, got %d", gotUsage.CachedTokens)
	}
}

func TestOpenAIResponsesClient_ChatCompletionStreamWithTools_EmitsUsageFromResponseCompleted(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.output_item.added\",\"item\":{\"type\":\"function_call\",\"id\":\"fc_1\",\"call_id\":\"call_1\",\"name\":\"lookup\",\"arguments\":\"{\\\"q\\\":\\\"hi\\\"}\"}}\n\n")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":40,\"output_tokens\":5,\"total_tokens\":45,\"input_tokens_details\":{\"cached_tokens\":32}}}}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(mock.Close)

	client := NewOpenAIResponsesClient(ClientConfig{
		Endpoint: mock.URL,
		APIKey:   "sk-test",
		Model:    "gpt-test",
	})

	var gotUsage UsageInfo
	opts := &ChatCompletionOptions{
		Tools: []Tool{
			{
				Type: "function",
				Function: ToolFunction{
					Name: "lookup",
				},
			},
		},
		Trace: &TraceCallback{
			OnUsage: func(ctx context.Context, usage UsageInfo) {
				gotUsage = usage
			},
		},
	}

	result, err := client.ChatCompletionStreamWithTools(
		context.Background(),
		[]ChatMessage{{Role: "user", Content: "hi"}},
		opts,
		nil,
	)
	if err != nil {
		t.Fatalf("ChatCompletionStreamWithTools: %v", err)
	}

	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result.ToolCalls))
	}
	if result.ToolCalls[0].Function.Name != "lookup" {
		t.Fatalf("expected tool name lookup, got %q", result.ToolCalls[0].Function.Name)
	}
	if gotUsage.CachedTokens != 32 {
		t.Fatalf("expected cached_tokens=32, got %d", gotUsage.CachedTokens)
	}
	if gotUsage.TotalTokens != 45 {
		t.Fatalf("expected total_tokens=45, got %d", gotUsage.TotalTokens)
	}
}
