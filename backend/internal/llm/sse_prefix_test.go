package llm

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIClient_ParsesSSEDataWithoutSpace(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data:{\"id\":\"cmpl-1\",\"choices\":[{\"delta\":{\"role\":\"assistant\",\"content\":\"OK\"},\"finish_reason\":\"stop\"}]}\n\n")
		_, _ = io.WriteString(w, "data:[DONE]\n\n")
	}))
	t.Cleanup(mock.Close)

	client := NewOpenAIClient(ClientConfig{
		Endpoint: mock.URL,
		APIKey:   "sk-test",
		Model:    "gpt-test",
	})

	var out bytes.Buffer
	err := client.ChatCompletionStream(context.Background(), []ChatMessage{
		BuildSystemMessage("sys"),
		BuildUserMessage("hi"),
	}, nil, func(chunk string) error {
		out.WriteString(chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if out.String() != "OK" {
		t.Fatalf("expected OK, got %q", out.String())
	}
}

func TestAnthropicClient_ParsesSSEDataWithoutSpace(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/messages" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data:{\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"OK\"}}\n\n")
		_, _ = io.WriteString(w, "data:[DONE]\n\n")
	}))
	t.Cleanup(mock.Close)

	client := NewAnthropicClient(ClientConfig{
		Endpoint: mock.URL,
		APIKey:   "sk-test",
		Model:    "claude-test",
	})

	var out bytes.Buffer
	err := client.ChatCompletionStream(context.Background(), []ChatMessage{
		BuildUserMessage("hi"),
	}, nil, func(chunk string) error {
		out.WriteString(chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if out.String() != "OK" {
		t.Fatalf("expected OK, got %q", out.String())
	}
}

