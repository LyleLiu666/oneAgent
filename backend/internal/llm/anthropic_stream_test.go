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

func TestAnthropicClient_ChatCompletionStream_EmitsTraceTokens(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/messages" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"hello\"}}\n\n")
		_, _ = io.WriteString(w, "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\" world\"}}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(mock.Close)

	client := NewAnthropicClient(ClientConfig{
		Endpoint: mock.URL,
		APIKey:   "sk-test",
		Model:    "claude-test",
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
				_ = ctx
				_ = input
				mu.Lock()
				startCalled = true
				mu.Unlock()
			},
			OnFirstToken: func(ctx context.Context) {
				_ = ctx
				mu.Lock()
				firstTokenHits++
				mu.Unlock()
			},
			OnToken: func(ctx context.Context, token string) {
				_ = ctx
				mu.Lock()
				tokens = append(tokens, token)
				mu.Unlock()
			},
			OnComplete: func(ctx context.Context, fullOutput string, err error) {
				_ = ctx
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
		[]ChatMessage{{Role: "user", Content: "hi"}},
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

