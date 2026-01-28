package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/config"
	oneruntime "github.com/liu_y/oneAgent/backend/internal/runtime"
)

func TestE2E_Chat_StandardSafetyTier_BlocksBashForNonAdmin(t *testing.T) {
	home := t.TempDir()
	prevCfg := config.AppConfig
	t.Cleanup(func() { config.AppConfig = prevCfg })

	cfg, err := config.Load(config.LoadOptions{
		Home:     home,
		Profile:  "dev",
		AuthMode: "token",
	})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	rt, err := oneruntime.Init(cfg)
	if err != nil {
		t.Fatalf("init runtime: %v", err)
	}
	t.Cleanup(func() { _ = rt.Close() })

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		finish := "stop"
		writeOpenAISSE(t, w, openAIStreamChunk{
			ID: "cmpl-1",
			Choices: []struct {
				Delta struct {
					Role      string `json:"role,omitempty"`
					Content   string `json:"content,omitempty"`
					ToolCalls []struct {
						Index    int    `json:"index"`
						ID       string `json:"id,omitempty"`
						Type     string `json:"type,omitempty"`
						Function struct {
							Name      string `json:"name,omitempty"`
							Arguments string `json:"arguments,omitempty"`
						} `json:"function,omitempty"`
					} `json:"tool_calls,omitempty"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			}{{
				Delta: struct {
					Role      string `json:"role,omitempty"`
					Content   string `json:"content,omitempty"`
					ToolCalls []struct {
						Index    int    `json:"index"`
						ID       string `json:"id,omitempty"`
						Type     string `json:"type,omitempty"`
						Function struct {
							Name      string `json:"name,omitempty"`
							Arguments string `json:"arguments,omitempty"`
						} `json:"function,omitempty"`
					} `json:"tool_calls,omitempty"`
				}{
					Role:    "assistant",
					Content: "OK",
				},
				FinishReason: &finish,
			}},
		})
		writeOpenAISSEDone(w)
	}))
	t.Cleanup(mock.Close)

	router, err := NewRouter(rt)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	// Create a non-admin token (principal_id=bob).
	var tokResp struct {
		Token string `json:"token"`
	}
	mustPostJSON(t, srv.URL, rt.AuthToken, "/api/admin/tokens", map[string]any{
		"principal_id": "bob",
	}, &tokResp)

	var providerResp createProviderResp
	mustPostJSON(t, srv.URL, tokResp.Token, "/api/llm/providers", map[string]any{
		"name":          "mock",
		"provider_type": "openai",
		"base_url":      mock.URL,
		"api_key":       "sk-test",
	}, &providerResp)

	var modelResp createModelResp
	mustPostJSON(t, srv.URL, tokResp.Token, "/api/llm/models", map[string]any{
		"provider_id":     providerResp.ID,
		"name":            "mock-model",
		"model":           "gpt-test",
		"is_default":      true,
		"safety_tier":     "standard",
		"enable_kv_cache": true,
	}, &modelResp)

	workspace := t.TempDir()
	reqBody := map[string]any{
		"message":       "hello",
		"model_id":      modelResp.ID,
		"tool_protocol": "json",
		"tool_ids":      []string{"bash"},
		"workspace":     workspace,
	}
	data, _ := json.Marshal(reqBody)
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/chat", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("new chat request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokResp.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("chat request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 400, got status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var got map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if !strings.Contains(strings.ToLower(got["error"].(string)), "safety_tier") {
		t.Fatalf("expected safety_tier mention, got %+v", got)
	}
}
