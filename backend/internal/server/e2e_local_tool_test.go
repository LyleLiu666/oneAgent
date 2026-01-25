package server

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/config"
	oneruntime "github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/web"
)

type streamEvent struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

type streamMsg struct {
	Op      string `json:"op"`
	Role    string `json:"role,omitempty"`
	MsgType string `json:"msg_type,omitempty"`
	Delta   string `json:"delta,omitempty"`
	Error   string `json:"error,omitempty"`
}

type createProviderResp struct {
	ID string `json:"id"`
}

type createModelResp struct {
	ID string `json:"id"`
}

func mustPostJSON(t *testing.T, baseURL, token, path string, body any, out any) {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, baseURL+path, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post %s: %v", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("post %s status=%d body=%s", path, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	if out == nil {
		return
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

func mustPutJSON(t *testing.T, baseURL, token, path string, body any, out any) {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req, err := http.NewRequest(http.MethodPut, baseURL+path, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("put %s: %v", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("put %s status=%d body=%s", path, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	if out == nil {
		return
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

func mustGet(t *testing.T, baseURL, token, path string) []byte {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, baseURL+path, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get %s: %v", path, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("get %s status=%d body=%s", path, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body
}

func readAssistantTextFromChat(t *testing.T, body io.Reader) string {
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
		if evt.Type != "msg" {
			continue
		}
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
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan chat stream: %v", err)
	}
	return out.String()
}

type openAIStreamChunk struct {
	ID      string `json:"id"`
	Choices []struct {
		Delta struct {
			Role    string `json:"role,omitempty"`
			Content string `json:"content,omitempty"`
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
	} `json:"choices"`
}

func writeOpenAISSE(t *testing.T, w http.ResponseWriter, chunk openAIStreamChunk) {
	t.Helper()
	b, err := json.Marshal(chunk)
	if err != nil {
		t.Fatalf("marshal sse chunk: %v", err)
	}
	_, _ = w.Write([]byte("data: "))
	_, _ = w.Write(b)
	_, _ = w.Write([]byte("\n\n"))
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func writeOpenAISSEDone(w http.ResponseWriter) {
	_, _ = w.Write([]byte("data: [DONE]\n\n"))
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func TestE2E_LocalToolMode_TokenChatWithoutDocker(t *testing.T) {
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
					Role    string `json:"role,omitempty"`
					Content string `json:"content,omitempty"`
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
					Role    string `json:"role,omitempty"`
					Content string `json:"content,omitempty"`
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

	workspace := t.TempDir()
	reqBody := map[string]any{
		"message":       "hello",
		"tool_protocol": "json",
		"tool_ids":      []string{},
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
	if strings.TrimSpace(got) != "OK" {
		t.Fatalf("expected assistant=OK, got %q", got)
	}
}

func TestE2E_MissingRg_FallsBackToGrepDuringToolCall(t *testing.T) {
	grepPath, err := exec.LookPath("grep")
	if err != nil {
		t.Skip("grep not installed")
	}
	toolDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink(grepPath, filepath.Join(toolDir, "grep")); err != nil {
		t.Fatalf("symlink grep: %v", err)
	}
	t.Setenv("PATH", toolDir)

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

	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "a.txt"), []byte("needle\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	var (
		mu       sync.Mutex
		requests [][]byte
	)
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		requests = append(requests, body)
		step := len(requests)
		mu.Unlock()

		w.Header().Set("Content-Type", "text/event-stream")
		if step == 1 {
			toolFinish := "tool_calls"
			writeOpenAISSE(t, w, openAIStreamChunk{
				ID: "cmpl-1",
				Choices: []struct {
					Delta struct {
						Role    string `json:"role,omitempty"`
						Content string `json:"content,omitempty"`
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
						Role    string `json:"role,omitempty"`
						Content string `json:"content,omitempty"`
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
						Content: "",
						ToolCalls: []struct {
							Index    int    `json:"index"`
							ID       string `json:"id,omitempty"`
							Type     string `json:"type,omitempty"`
							Function struct {
								Name      string `json:"name,omitempty"`
								Arguments string `json:"arguments,omitempty"`
							} `json:"function,omitempty"`
						}{{
							Index: 0,
							ID:    "call_1",
							Type:  "function",
							Function: struct {
								Name      string `json:"name,omitempty"`
								Arguments string `json:"arguments,omitempty"`
							}{
								Name:      "rg",
								Arguments: `{"pattern":"needle","path":".","fixed_strings":true}`,
							},
						}},
					},
					FinishReason: &toolFinish,
				}},
			})
			writeOpenAISSEDone(w)
			return
		}

		finalFinish := "stop"
		writeOpenAISSE(t, w, openAIStreamChunk{
			ID: "cmpl-2",
			Choices: []struct {
				Delta struct {
					Role    string `json:"role,omitempty"`
					Content string `json:"content,omitempty"`
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
					Role    string `json:"role,omitempty"`
					Content string `json:"content,omitempty"`
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
					Content: "DONE",
				},
				FinishReason: &finalFinish,
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

	reqBody := map[string]any{
		"message":       "search it",
		"tool_protocol": "json",
		"tool_ids":      []string{"rg"},
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
		t.Fatalf("expected >=2 llm requests, got %d", len(requests))
	}

	var second struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(requests[1], &second); err != nil {
		t.Fatalf("unmarshal llm request: %v", err)
	}
	var toolContent string
	for _, m := range second.Messages {
		if m.Role == "tool" {
			toolContent = m.Content
		}
	}
	if toolContent == "" {
		t.Fatalf("expected tool content in second request")
	}
	if !strings.Contains(toolContent, `"backend":"grep"`) {
		t.Fatalf("expected rg tool fallback backend=grep, got tool content=%s", toolContent)
	}
	if !strings.Contains(toolContent, `"pattern":"needle"`) || !strings.Contains(toolContent, `"a.txt"`) {
		t.Fatalf("expected rg tool output to include match, got tool content=%s", toolContent)
	}
}

func TestE2E_SettingsPersistAcrossRestart_AndNoSecretLeak(t *testing.T) {
	home := t.TempDir()

	setKey := func(baseURL, token string) {
		var resp map[string]any
		mustPutJSON(t, baseURL, token, "/api/bocha/settings", map[string]any{
			"bocha_api_key": "bocha-secret",
		}, &resp)
	}

	createProviderAndModel := func(baseURL, token string) {
		var providerResp createProviderResp
		mustPostJSON(t, baseURL, token, "/api/llm/providers", map[string]any{
			"name":          "persist",
			"provider_type": "openai",
			"base_url":      "http://127.0.0.1:9999",
			"api_key":       "llm-secret",
		}, &providerResp)
		var modelResp createModelResp
		mustPostJSON(t, baseURL, token, "/api/llm/models", map[string]any{
			"provider_id": providerResp.ID,
			"name":        "persist-model",
			"model":       "gpt-test",
			"is_default":  true,
		}, &modelResp)
	}

	start := func() (*oneruntime.Runtime, *httptest.Server) {
		prevCfg := config.AppConfig
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
		router, err := NewRouter(rt)
		if err != nil {
			t.Fatalf("new router: %v", err)
		}
		srv := httptest.NewServer(router)
		t.Cleanup(func() { config.AppConfig = prevCfg })
		return rt, srv
	}

	rt1, srv1 := start()
	setKey(srv1.URL, rt1.AuthToken)
	createProviderAndModel(srv1.URL, rt1.AuthToken)
	srv1.Close()
	_ = rt1.Close()

	rt2, srv2 := start()
	t.Cleanup(func() {
		srv2.Close()
		_ = rt2.Close()
	})

	bochaBody := mustGet(t, srv2.URL, rt2.AuthToken, "/api/bocha/settings")
	if bytes.Contains(bochaBody, []byte("bocha-secret")) {
		t.Fatalf("bocha settings leaked secret: %s", strings.TrimSpace(string(bochaBody)))
	}
	if !bytes.Contains(bochaBody, []byte(`"has_bocha_api_key":true`)) {
		t.Fatalf("expected has_bocha_api_key=true, got %s", strings.TrimSpace(string(bochaBody)))
	}

	providersBody := mustGet(t, srv2.URL, rt2.AuthToken, "/api/llm/providers")
	if bytes.Contains(providersBody, []byte("llm-secret")) {
		t.Fatalf("providers leaked secret: %s", strings.TrimSpace(string(providersBody)))
	}
	if !bytes.Contains(providersBody, []byte(`"has_api_key":true`)) {
		t.Fatalf("expected has_api_key=true, got %s", strings.TrimSpace(string(providersBody)))
	}

	meBody := mustGet(t, srv2.URL, rt2.AuthToken, "/api/me")
	if bytes.Contains(meBody, []byte(rt2.AuthToken)) {
		t.Fatalf("/api/me leaked token: %s", strings.TrimSpace(string(meBody)))
	}
}

func TestE2E_LoginPage_WarnsLANRisk(t *testing.T) {
	staticFS, err := fs.Sub(web.Static, "static")
	if err != nil {
		t.Fatalf("sub static fs: %v", err)
	}

	entries, err := fs.ReadDir(staticFS, "assets")
	if err != nil {
		t.Fatalf("read assets dir: %v", err)
	}

	var loginAsset string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, "Login-") && strings.HasSuffix(name, ".js") {
			loginAsset = name
			break
		}
	}
	if loginAsset == "" {
		t.Fatalf("login asset not found under static/assets")
	}

	b, err := web.Static.ReadFile("static/assets/" + loginAsset)
	if err != nil {
		t.Fatalf("read login asset: %v", err)
	}
	if !bytes.Contains(b, []byte("仅建议在可信局域网内使用")) {
		t.Fatalf("expected login page warning in embedded assets")
	}

	prevCfg := config.AppConfig
	t.Cleanup(func() { config.AppConfig = prevCfg })
	cfg, err := config.Load(config.LoadOptions{
		Home:    t.TempDir(),
		Profile: "local",
	})
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Bind != "0.0.0.0" {
		t.Fatalf("expected default bind=0.0.0.0 for local profile, got %q", cfg.Bind)
	}
}
