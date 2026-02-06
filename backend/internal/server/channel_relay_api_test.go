package server

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/config"
	"github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/settingsdb"
	"github.com/liu_y/oneAgent/backend/internal/taskqueue"
)

func signBody(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestChannelRelay_Inbound_RejectsInvalidSignature(t *testing.T) {
	t.Setenv("ONEAGENT_DISABLE_DAILY_LEARNING", "1")

	home := t.TempDir()
	cfg := &config.Config{
		Profile:                 "local",
		Bind:                    "127.0.0.1",
		Port:                    "0",
		Home:                    home,
		AuthMode:                "none",
		LogRetentionDays:        1,
		ChannelRelaySecret:      "secret",
		ChannelRelayOutboundURL: "",
	}
	rt, err := runtime.Init(cfg)
	if err != nil {
		t.Fatalf("init runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	body, _ := json.Marshal(map[string]any{
		"provider":     "webhook.v1",
		"principal_id": "local",
		"channel_id":   "c1",
		"thread_id":    "th1",
		"message_id":   "m1",
		"workspace":    t.TempDir(),
		"content":      "hello",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/channel_relay/v1/inbound", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-OneAgent-Signature", "deadbeef")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestChannelRelay_Inbound_IdempotentByMessageID(t *testing.T) {
	t.Setenv("ONEAGENT_DISABLE_DAILY_LEARNING", "1")

	home := t.TempDir()
	cfg := &config.Config{
		Profile:                 "local",
		Bind:                    "127.0.0.1",
		Port:                    "0",
		Home:                    home,
		AuthMode:                "none",
		LogRetentionDays:        1,
		ChannelRelaySecret:      "secret",
		ChannelRelayOutboundURL: "",
	}
	rt, err := runtime.Init(cfg)
	if err != nil {
		t.Fatalf("init runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	ws := t.TempDir()
	body, _ := json.Marshal(map[string]any{
		"provider":     "webhook.v1",
		"principal_id": "local",
		"channel_id":   "c1",
		"thread_id":    "th1",
		"message_id":   "m1",
		"workspace":    ws,
		"content":      "hello",
	})
	sig := signBody(cfg.ChannelRelaySecret, body)

	post := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/channel_relay/v1/inbound", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-OneAgent-Signature", sig)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	rec1 := post()
	if rec1.Code != http.StatusOK {
		t.Fatalf("first status=%d body=%s", rec1.Code, rec1.Body.String())
	}
	rec2 := post()
	if rec2.Code != http.StatusOK {
		t.Fatalf("second status=%d body=%s", rec2.Code, rec2.Body.String())
	}

	var resp2 map[string]any
	_ = json.Unmarshal(rec2.Body.Bytes(), &resp2)
	if id, ok := resp2["idempotent"].(bool); !ok || !id {
		t.Fatalf("expected idempotent=true, got %v", resp2["idempotent"])
	}
}

func TestChannelRelay_Inbound_EndToEnd_TriageAndOutboundNotification(t *testing.T) {
	t.Setenv("ONEAGENT_DISABLE_DAILY_LEARNING", "1")

	var (
		mu       sync.Mutex
		requests []map[string]any
	)
	outbound := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		mu.Lock()
		requests = append(requests, payload)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(outbound.Close)

	// Minimal OpenAI-compatible mock for secretary triage planning.
	llm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		resp := map[string]any{
			"id":      "chatcmpl_mock",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "mock",
			"choices": []any{
				map[string]any{
					"index": 0,
					"message": map[string]any{
						"role": "assistant",
						"content": `<secretary_triage_plan>
  <intent>dispatch</intent>
  <summary_message>ok</summary_message>
  <tasks>
    <task>
      <title>relay task</title>
      <prompt>echo ok</prompt>
      <workspace_strategy>session</workspace_strategy>
    </task>
  </tasks>
  <task_actions></task_actions>
  <questions></questions>
</secretary_triage_plan>`,
					},
					"finish_reason": "stop",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(llm.Close)

	home := t.TempDir()
	cfg := &config.Config{
		Profile:                 "local",
		Bind:                    "127.0.0.1",
		Port:                    "0",
		Home:                    home,
		AuthMode:                "none",
		LogRetentionDays:        1,
		ChannelRelaySecret:      "secret",
		ChannelRelayOutboundURL: outbound.URL,
	}
	rt, err := runtime.Init(cfg)
	if err != nil {
		t.Fatalf("init runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	// Configure a default LLM provider/model for secretary triage.
	if rt.Settings == nil {
		t.Fatalf("expected settings db")
	}
	_, err = rt.Settings.CreateProvider(context.Background(), settingsdb.Provider{
		ID:           "p1",
		UserID:       "local",
		Name:         "mock",
		ProviderType: "openai",
		BaseURL:      llm.URL,
		APIKey:       "k",
	})
	if err != nil {
		t.Fatalf("CreateProvider: %v", err)
	}
	_, err = rt.Settings.CreateModel(context.Background(), settingsdb.Model{
		ID:         "m1",
		ProviderID: "p1",
		UserID:     "local",
		Name:       "mock",
		Model:      "mock",
		IsDefault:  true,
	})
	if err != nil {
		t.Fatalf("CreateModel: %v", err)
	}

	// Use a fast deterministic task runner for the e2e path.
	rt.TaskRunner = &taskqueue.TaskRunner{
		Store: rt.Tasks,
		ExecuteAttempt: func(ctx context.Context, task taskqueue.Task, attempt taskqueue.Attempt, resumedFrom *taskqueue.Attempt) (taskqueue.AttemptResult, error) {
			_ = ctx
			_ = resumedFrom
			return taskqueue.AttemptResult{Summary: "ok"}, nil
		},
		DecideOutcome: func(ctx context.Context, task taskqueue.Task, attempt taskqueue.Attempt) (taskqueue.ObserverDecision, error) {
			_ = ctx
			_ = task
			_ = attempt
			return taskqueue.ObserverDecision{Pass: true}, nil
		},
	}

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	ws := t.TempDir()
	body, _ := json.Marshal(map[string]any{
		"provider":     "webhook.v1",
		"principal_id": "local",
		"channel_id":   "c1",
		"thread_id":    "th1",
		"message_id":   "m1",
		"workspace":    ws,
		"content":      "please do it",
	})
	sig := signBody(cfg.ChannelRelaySecret, body)

	req := httptest.NewRequest(http.MethodPost, "/api/channel_relay/v1/inbound", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-OneAgent-Signature", sig)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	// Wait for outbound notification.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(requests)
		mu.Unlock()
		if n > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(requests) == 0 {
		t.Fatalf("expected outbound notification to be sent")
	}
	if requests[0]["task_id"] == "" {
		t.Fatalf("expected task_id in payload, got %+v", requests[0])
	}
}

func TestChannelRelay_Outbound_RetriesOnFailures(t *testing.T) {
	t.Setenv("ONEAGENT_DISABLE_DAILY_LEARNING", "1")

	var (
		mu           sync.Mutex
		requestCount int
	)
	outbound := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		mu.Lock()
		requestCount++
		n := requestCount
		mu.Unlock()
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(outbound.Close)

	llm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		resp := map[string]any{
			"id":      "chatcmpl_mock",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   "mock",
			"choices": []any{
				map[string]any{
					"index": 0,
					"message": map[string]any{
						"role": "assistant",
						"content": `<secretary_triage_plan>
  <intent>dispatch</intent>
  <summary_message>ok</summary_message>
  <tasks>
    <task>
      <title>retry task</title>
      <prompt>echo ok</prompt>
      <workspace_strategy>session</workspace_strategy>
    </task>
  </tasks>
  <task_actions></task_actions>
  <questions></questions>
</secretary_triage_plan>`,
					},
					"finish_reason": "stop",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(llm.Close)

	home := t.TempDir()
	cfg := &config.Config{
		Profile:                 "local",
		Bind:                    "127.0.0.1",
		Port:                    "0",
		Home:                    home,
		AuthMode:                "none",
		LogRetentionDays:        1,
		ChannelRelaySecret:      "secret",
		ChannelRelayOutboundURL: outbound.URL,
	}
	rt, err := runtime.Init(cfg)
	if err != nil {
		t.Fatalf("init runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	_, err = rt.Settings.CreateProvider(context.Background(), settingsdb.Provider{
		ID:           "p1",
		UserID:       "local",
		Name:         "mock",
		ProviderType: "openai",
		BaseURL:      llm.URL,
		APIKey:       "k",
	})
	if err != nil {
		t.Fatalf("CreateProvider: %v", err)
	}
	_, err = rt.Settings.CreateModel(context.Background(), settingsdb.Model{
		ID:         "m1",
		ProviderID: "p1",
		UserID:     "local",
		Name:       "mock",
		Model:      "mock",
		IsDefault:  true,
	})
	if err != nil {
		t.Fatalf("CreateModel: %v", err)
	}

	rt.TaskRunner = &taskqueue.TaskRunner{
		Store: rt.Tasks,
		ExecuteAttempt: func(ctx context.Context, task taskqueue.Task, attempt taskqueue.Attempt, resumedFrom *taskqueue.Attempt) (taskqueue.AttemptResult, error) {
			_ = ctx
			_ = resumedFrom
			return taskqueue.AttemptResult{Summary: "ok"}, nil
		},
		DecideOutcome: func(ctx context.Context, task taskqueue.Task, attempt taskqueue.Attempt) (taskqueue.ObserverDecision, error) {
			_ = ctx
			_ = task
			_ = attempt
			return taskqueue.ObserverDecision{Pass: true}, nil
		},
	}

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	ws := t.TempDir()
	body, _ := json.Marshal(map[string]any{
		"provider":     "webhook.v1",
		"principal_id": "local",
		"channel_id":   "c1",
		"thread_id":    "th1",
		"message_id":   "m1",
		"workspace":    ws,
		"content":      "please do it",
	})
	sig := signBody(cfg.ChannelRelaySecret, body)

	req := httptest.NewRequest(http.MethodPost, "/api/channel_relay/v1/inbound", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-OneAgent-Signature", sig)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := requestCount
		mu.Unlock()
		if n >= 3 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	if requestCount < 3 {
		t.Fatalf("expected outbound retries (>=3 requests), got %d", requestCount)
	}
}
