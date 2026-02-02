package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/config"
	oneruntime "github.com/liu_y/oneAgent/backend/internal/runtime"
)

func TestServer_SecretaryInboxMessage_NonStreamingChatCompletions_ReturnsAck(t *testing.T) {
	home := t.TempDir()
	cfg := &config.Config{
		Profile:          "local",
		Bind:             "127.0.0.1",
		Port:             "0",
		Home:             home,
		AuthMode:         "none",
		LogRetentionDays: 1,
	}

	rt, err := oneruntime.Init(cfg)
	if err != nil {
		t.Fatalf("init runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	expectedAck := "收到，马上整理报告。"
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "cmpl-test",
			"choices": []any{
				map[string]any{
					"message": map[string]any{
						"role":    "assistant",
						"content": expectedAck,
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     1,
				"completion_tokens": 1,
				"total_tokens":      2,
			},
		})
	}))
	t.Cleanup(mock.Close)

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	var providerResp createProviderResp
	mustPostJSON(t, srv.URL, rt.AuthToken, "/api/llm/providers", map[string]any{
		"name":          "mock",
		"provider_type": "openai",
		"base_url":      mock.URL,
		"api_key":       "sk-test",
	}, &providerResp)

	var modelResp createModelResp
	mustPostJSON(t, srv.URL, rt.AuthToken, "/api/llm/models", map[string]any{
		"provider_id": providerResp.ID,
		"name":        "mock-model",
		"model":       "gpt-test",
		"is_default":  true,
	}, &modelResp)

	var inboxResp struct {
		SessionID    string `json:"session_id"`
		MessageID    uint   `json:"message_id"`
		AckMessageID uint   `json:"ack_message_id"`
		AckText      string `json:"ack_text"`
	}
	mustPostJSON(t, srv.URL, rt.AuthToken, "/api/secretary/inbox/messages", map[string]any{
		"content": "帮我整理一份报告",
	}, &inboxResp)

	if strings.TrimSpace(inboxResp.SessionID) == "" || inboxResp.MessageID == 0 || inboxResp.AckMessageID == 0 {
		t.Fatalf("unexpected inbox response: %+v", inboxResp)
	}
	if inboxResp.AckText != expectedAck {
		t.Fatalf("expected ack %q, got %q", expectedAck, inboxResp.AckText)
	}
}

func TestServer_SecretaryInboxMessage_EmptyStreamAck_FallsBack(t *testing.T) {
	home := t.TempDir()
	cfg := &config.Config{
		Profile:          "local",
		Bind:             "127.0.0.1",
		Port:             "0",
		Home:             home,
		AuthMode:         "none",
		LogRetentionDays: 1,
	}

	rt, err := oneruntime.Init(cfg)
	if err != nil {
		t.Fatalf("init runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	// Streaming endpoint that immediately completes without emitting content.
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	t.Cleanup(mock.Close)

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	var providerResp createProviderResp
	mustPostJSON(t, srv.URL, rt.AuthToken, "/api/llm/providers", map[string]any{
		"name":          "mock",
		"provider_type": "openai",
		"base_url":      mock.URL,
		"api_key":       "sk-test",
	}, &providerResp)

	var modelResp createModelResp
	mustPostJSON(t, srv.URL, rt.AuthToken, "/api/llm/models", map[string]any{
		"provider_id": providerResp.ID,
		"name":        "mock-model",
		"model":       "gpt-test",
		"is_default":  true,
	}, &modelResp)

	var inboxResp struct {
		SessionID    string `json:"session_id"`
		MessageID    uint   `json:"message_id"`
		AckMessageID uint   `json:"ack_message_id"`
		AckText      string `json:"ack_text"`
	}
	mustPostJSON(t, srv.URL, rt.AuthToken, "/api/secretary/inbox/messages", map[string]any{
		"content": "帮我整理一份报告",
	}, &inboxResp)

	if strings.TrimSpace(inboxResp.SessionID) == "" || inboxResp.MessageID == 0 || inboxResp.AckMessageID == 0 {
		t.Fatalf("unexpected inbox response: %+v", inboxResp)
	}
	if strings.TrimSpace(inboxResp.AckText) == "" {
		t.Fatalf("expected fallback ack, got empty")
	}
	if strings.Contains(inboxResp.AckText, "已记下") {
		t.Fatalf("expected fallback ack to avoid repeating 已记下, got %q", inboxResp.AckText)
	}
}
