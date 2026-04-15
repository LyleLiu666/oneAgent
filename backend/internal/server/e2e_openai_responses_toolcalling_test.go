package server

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/config"
	oneruntime "github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/tool"
)

type chatUsageEvent struct {
	ResponseTokens int     `json:"response_tokens,omitempty"`
	InputTokens    int     `json:"input_tokens,omitempty"`
	OutputTokens   int     `json:"output_tokens,omitempty"`
	TotalTokens    int     `json:"total_tokens,omitempty"`
	CachedTokens   int     `json:"cached_tokens,omitempty"`
	CacheHitRatio  float64 `json:"cache_hit_ratio,omitempty"`
}

func readAssistantTextAndUsageFromChat(t *testing.T, body io.Reader) (assistantText string, usage chatUsageEvent) {
	t.Helper()
	var out strings.Builder

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		var evt streamEvent
		if err := json.Unmarshal([]byte(payload), &evt); err != nil {
			continue
		}

		switch evt.Type {
		case "usage":
			if err := json.Unmarshal([]byte(evt.Data), &usage); err != nil {
				continue
			}
		case "msg":
			var msg streamMsg
			if err := json.Unmarshal([]byte(evt.Data), &msg); err != nil {
				continue
			}
			if msg.MsgType != "text" || msg.Role != "assistant" {
				continue
			}
			if msg.Op == "delta" && msg.Delta != "" {
				out.WriteString(msg.Delta)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan chat stream: %v", err)
	}
	return out.String(), usage
}

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

		w.Header().Set("Content-Type", "text/event-stream")
		switch n {
		case 1:
			_, _ = io.WriteString(w, "data: {\"type\":\"response.output_item.added\",\"item\":{\"type\":\"function_call\",\"id\":\"fc_1\",\"call_id\":\"call_1\",\"name\":\"ls\",\"arguments\":\"{\\\"path\\\":\\\".\\\"}\"}}\n\n")
			_, _ = io.WriteString(w, "data: [DONE]\n\n")
		default:
			_, _ = io.WriteString(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"DONE\"}\n\n")
			_, _ = io.WriteString(w, "data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":100,\"output_tokens\":4,\"total_tokens\":104,\"input_tokens_details\":{\"cached_tokens\":60}}}}\n\n")
			_, _ = io.WriteString(w, "data: [DONE]\n\n")
		}
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
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
		"provider_id":     providerResp.ID,
		"name":            "mock-model",
		"model":           "gpt-test",
		"enable_kv_cache": true,
		"is_default":      true,
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

	got, usage := readAssistantTextAndUsageFromChat(t, resp.Body)
	if strings.TrimSpace(got) != "DONE" {
		t.Fatalf("expected assistant=DONE, got %q", got)
	}
	if usage.ResponseTokens != 4 {
		t.Fatalf("expected response_tokens=4, got %d", usage.ResponseTokens)
	}
	if usage.InputTokens != 100 || usage.TotalTokens != 104 {
		t.Fatalf("unexpected usage totals: %+v", usage)
	}
	if usage.CachedTokens != 60 {
		t.Fatalf("expected cached_tokens=60, got %d", usage.CachedTokens)
	}
	if math.Abs(usage.CacheHitRatio-0.6) > 0.0001 {
		t.Fatalf("expected cache_hit_ratio=0.6, got %v", usage.CacheHitRatio)
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

	logPaths, err := filepath.Glob(filepath.Join(rt.Layout.LLMLogsDir, "*", "*", "*.json"))
	if err != nil {
		t.Fatalf("glob llm logs: %v", err)
	}
	if len(logPaths) != 1 {
		t.Fatalf("expected 1 llm log, got %d (%v)", len(logPaths), logPaths)
	}

	rawLog, err := os.ReadFile(logPaths[0])
	if err != nil {
		t.Fatalf("read llm log: %v", err)
	}

	var logged struct {
		Usage *chatUsageEvent `json:"usage"`
	}
	if err := json.Unmarshal(rawLog, &logged); err != nil {
		t.Fatalf("unmarshal llm log: %v", err)
	}
	if logged.Usage == nil {
		t.Fatal("expected llm log to include usage")
	}
	if logged.Usage.CachedTokens != 60 || logged.Usage.OutputTokens != 4 {
		t.Fatalf("unexpected logged usage: %+v", logged.Usage)
	}
}
