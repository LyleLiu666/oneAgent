package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/memorydb"
	oneruntime "github.com/liu_y/oneAgent/backend/internal/runtime"
)

func TestServer_SecretaryState_NoSessionID_BootstrapsCanonicalSession(t *testing.T) {
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

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	body := mustGet(t, srv.URL, rt.AuthToken, "/api/secretary/state")
	var st struct {
		SessionID       string `json:"session_id"`
		CursorMessageID uint   `json:"cursor_message_id"`
	}
	if err := json.Unmarshal(body, &st); err != nil {
		t.Fatalf("unmarshal state: %v", err)
	}
	if strings.TrimSpace(st.SessionID) == "" {
		t.Fatalf("expected session_id, got %+v", st)
	}
	if st.CursorMessageID != 0 {
		t.Fatalf("expected cursor=0 for new session, got %+v", st)
	}

	// The canonical session should now exist in the session store.
	_ = mustGet(t, srv.URL, rt.AuthToken, "/api/sessions/"+st.SessionID)
}

func TestServer_SecretaryInboxMessage_MemorySyncInjected_AndCursorAdvances(t *testing.T) {
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

	ctx := context.Background()
	if rt.Memory == nil {
		t.Fatalf("expected memory db")
	}
	if _, err := rt.Memory.AppendEntry(ctx, memorydb.Entry{
		PrincipalID: "local",
		Writer:      "SW",
		Type:        "worklog",
		Title:       "worker_note",
		Content:     "from worker",
	}); err != nil {
		t.Fatalf("append SW entry: %v", err)
	}

	var mu sync.Mutex
	var sawFailure string
	ackCalls := 0

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}

		var req struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		system := ""
		userMsgs := make([]string, 0, 4)
		for _, m := range req.Messages {
			if m.Role == "system" && system == "" {
				system = m.Content
			}
			if m.Role == "user" {
				userMsgs = append(userMsgs, m.Content)
			}
		}

		content := "ok"
		if strings.Contains(system, "ONEAGENT_SECRETARY_ACK") {
			ackCalls++
			hasSync := false
			for _, um := range userMsgs {
				if strings.Contains(um, "[MEMORY_SYNC from=SW") {
					hasSync = true
					break
				}
			}

			mu.Lock()
			if ackCalls == 1 && !hasSync && sawFailure == "" {
				sawFailure = "expected first ack request to include memory sync prompt"
			}
			if ackCalls >= 2 && hasSync && sawFailure == "" {
				sawFailure = "expected subsequent ack requests to omit memory sync after cursor advances"
			}
			mu.Unlock()

			content = "收到"
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		chunk := map[string]any{
			"id": "cmpl-test",
			"choices": []any{
				map[string]any{
					"delta": map[string]any{"content": content},
				},
			},
		}
		data, _ := json.Marshal(chunk)
		_, _ = w.Write([]byte("data: "))
		_, _ = w.Write(data)
		_, _ = w.Write([]byte("\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	t.Cleanup(mock.Close)

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	// Configure mock model for secretary ack.
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

	var r1 struct {
		SessionID string `json:"session_id"`
		AckText   string `json:"ack_text"`
	}
	mustPostJSON(t, srv.URL, rt.AuthToken, "/api/secretary/inbox/messages", map[string]any{
		"content": "你好",
	}, &r1)
	if strings.TrimSpace(r1.SessionID) == "" {
		t.Fatalf("expected session id")
	}
	if strings.TrimSpace(r1.AckText) != "收到" {
		t.Fatalf("expected ack text %q, got %q", "收到", r1.AckText)
	}

	var r2 struct {
		SessionID string `json:"session_id"`
		AckText   string `json:"ack_text"`
	}
	mustPostJSON(t, srv.URL, rt.AuthToken, "/api/secretary/inbox/messages", map[string]any{
		"content": "再来一次",
	}, &r2)
	if r2.SessionID != r1.SessionID {
		t.Fatalf("expected canonical session id to be stable, got %q then %q", r1.SessionID, r2.SessionID)
	}
	if strings.TrimSpace(r2.AckText) != "收到" {
		t.Fatalf("expected ack text %q, got %q", "收到", r2.AckText)
	}

	if ackCalls < 2 {
		t.Fatalf("expected at least 2 ack calls, got %d", ackCalls)
	}
	mu.Lock()
	defer mu.Unlock()
	if sawFailure != "" {
		t.Fatalf("%s", sawFailure)
	}
}

