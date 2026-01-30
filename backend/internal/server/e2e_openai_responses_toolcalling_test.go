package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/config"
	oneruntime "github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/tool"
)

func TestE2E_OpenAIResponsesProvider_ToolLoop_MVP(t *testing.T) {
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

	var (
		mu       sync.Mutex
		requests [][]byte
		call     int
	)
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		requests = append(requests, body)
		call++
		n := call
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		switch n {
		case 1:
			_, _ = w.Write([]byte(`{
  "id": "resp_1",
  "output": [
    {
      "type": "function_call",
      "call_id": "call_1",
      "name": "ls",
      "arguments": "{\"path\":\".\"}"
    }
  ]
}`))
		default:
			_, _ = w.Write([]byte(`{
  "id": "resp_2",
  "output": [
    {
      "type": "message",
      "role": "assistant",
      "content": [{"type":"output_text","text":"DONE"}]
    }
  ]
}`))
		}
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
		"provider_type": "openai_response",
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

	workspace := t.TempDir()
	reqBody := map[string]any{
		"message":       "list workspace",
		"tool_protocol": "json",
		"tool_ids":      []string{tool.ToolIDLs},
		"workspace":     workspace,
	}
	data, _ := json.Marshal(reqBody)
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/chat", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("new chat request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+rt.AuthToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("chat request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("chat status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	got := readAssistantTextFromChat(t, resp.Body)
	if strings.TrimSpace(got) != "DONE" {
		t.Fatalf("expected assistant=DONE, got %q", got)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(requests) < 2 {
		t.Fatalf("expected at least 2 provider requests, got %d", len(requests))
	}

	var second map[string]any
	if err := json.Unmarshal(requests[1], &second); err != nil {
		t.Fatalf("unmarshal second request: %v", err)
	}
	input, ok := second["input"].([]any)
	if !ok {
		t.Fatalf("expected second request input list, got %#v", second["input"])
	}
	var sawOutput bool
	for _, item := range input {
		obj, _ := item.(map[string]any)
		if obj == nil {
			continue
		}
		if obj["type"] == "function_call_output" && obj["call_id"] == "call_1" {
			sawOutput = true
			break
		}
	}
	if !sawOutput {
		t.Fatalf("expected second request to include function_call_output for call_1, got %#v", input)
	}
}
