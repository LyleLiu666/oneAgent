package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/config"
	oneruntime "github.com/liu_y/oneAgent/backend/internal/runtime"
)

func TestServer_SecretaryInboxMessage_DoesNotReturnAck_AndDoesNotCallLLM(t *testing.T) {
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

	var calls atomic.Int64
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "unexpected LLM request (quick-ack is disabled)", http.StatusBadRequest)
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

	if strings.TrimSpace(inboxResp.SessionID) == "" || inboxResp.MessageID == 0 {
		t.Fatalf("unexpected inbox response: %+v", inboxResp)
	}
	if inboxResp.AckMessageID != 0 || strings.TrimSpace(inboxResp.AckText) != "" {
		t.Fatalf("expected empty ack, got ack_message_id=%d ack_text=%q", inboxResp.AckMessageID, inboxResp.AckText)
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("expected 0 LLM calls for inbox, got %d", got)
	}
}
