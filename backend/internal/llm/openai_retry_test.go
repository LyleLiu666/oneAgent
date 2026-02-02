package llm

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestOpenAIClient_ChatCompletionStream_RetriesOn5xx(t *testing.T) {
	var calls int32
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}

		call := atomic.AddInt32(&calls, 1)
		if call == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"type":"error","error":{"type":"api_error","message":"unknown error, 520 (1000)"},"request_id":"req-test-1"}`)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"id\":\"cmpl-1\",\"choices\":[{\"delta\":{\"role\":\"assistant\",\"content\":\"OK\"},\"finish_reason\":\"stop\"}]}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(mock.Close)

	client := NewOpenAIClient(ClientConfig{
		Endpoint: mock.URL,
		APIKey:   "sk-test",
		Model:    "gpt-test",
	})

	var out strings.Builder
	if err := client.ChatCompletionStream(
		context.Background(),
		[]ChatMessage{BuildUserMessage("hi")},
		nil,
		func(chunk string) error {
			out.WriteString(chunk)
			return nil
		},
	); err != nil {
		t.Fatalf("ChatCompletionStream: %v", err)
	}

	if out.String() != "OK" {
		t.Fatalf("expected output %q, got %q", "OK", out.String())
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected 2 calls, got %d", got)
	}
}

func TestOpenAIClient_ChatCompletionStream_ParsesProviderErrorPayload(t *testing.T) {
	var calls int32
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}

		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"type":"error","error":{"type":"api_error","message":"unknown error, 520 (1000)"},"request_id":"req-test-err"}`)
	}))
	t.Cleanup(mock.Close)

	client := NewOpenAIClient(ClientConfig{
		Endpoint: mock.URL,
		APIKey:   "sk-test",
		Model:    "gpt-test",
	})

	err := client.ChatCompletionStream(
		context.Background(),
		[]ChatMessage{BuildUserMessage("hi")},
		nil,
		func(string) error { return nil },
	)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T (%v)", err, err)
	}
	if apiErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, apiErr.StatusCode)
	}
	if apiErr.RequestID != "req-test-err" {
		t.Fatalf("expected request_id %q, got %q", "req-test-err", apiErr.RequestID)
	}
	if apiErr.Type != "api_error" {
		t.Fatalf("expected type %q, got %q", "api_error", apiErr.Type)
	}
	if apiErr.Message != "unknown error, 520 (1000)" {
		t.Fatalf("expected message %q, got %q", "unknown error, 520 (1000)", apiErr.Message)
	}
	if got := atomic.LoadInt32(&calls); got != int32(defaultAPIRetryAttempts) {
		t.Fatalf("expected %d calls, got %d", defaultAPIRetryAttempts, got)
	}
}
