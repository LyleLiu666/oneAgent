package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/liu_y/oneAgent/backend/internal/config"
	oneruntime "github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/tool"
)

func writeOpenAITextDelta(w http.ResponseWriter, id, content string) {
	escaped, _ := json.Marshal(content)
	_, _ = fmt.Fprintf(w, "data: {\"id\":%q,\"choices\":[{\"delta\":{\"content\":%s}}]}\n\n", id, string(escaped))
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func writeOpenAIDone(w http.ResponseWriter) {
	_, _ = io.WriteString(w, "data: [DONE]\n\n")
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func readSessionIDFromChatStream(t *testing.T, body io.Reader) string {
	t.Helper()
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
		if evt.Type == "session" && strings.TrimSpace(evt.Data) != "" {
			return strings.TrimSpace(evt.Data)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan chat stream: %v", err)
	}
	t.Fatalf("no session id received from stream")
	return ""
}

func fetchSessionMessages(t *testing.T, ctx context.Context, baseURL, token, sessionID string) ([]map[string]any, int) {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/sessions/"+sessionID, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/sessions/:id: %v", err)
	}
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, res.StatusCode
	}

	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode session: %v", err)
	}
	rawMsgs, _ := decoded["messages"].([]any)
	msgs := make([]map[string]any, 0, len(rawMsgs))
	for _, raw := range rawMsgs {
		if m, ok := raw.(map[string]any); ok {
			msgs = append(msgs, m)
		}
	}
	return msgs, res.StatusCode
}

func waitUntilSessionHasAtLeastMessages(t *testing.T, baseURL, token, sessionID string, timeout time.Duration, min int) []map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)

	for {
		select {
		case <-ctx.Done():
			t.Fatalf("timeout waiting for session messages for %s", sessionID)
		default:
		}

		msgs, status := fetchSessionMessages(t, ctx, baseURL, token, sessionID)
		if status != http.StatusOK {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		if len(msgs) >= min {
			return msgs
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestE2E_Chat_AttachAfterDisconnectStreamsToCompletion(t *testing.T) {
	t.Setenv("ONEAGENT_DISABLE_DAILY_LEARNING", "1")

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")

		writeOpenAITextDelta(w, "cmpl-1", "Hello")
		time.Sleep(150 * time.Millisecond)
		writeOpenAITextDelta(w, "cmpl-1", " world")
		time.Sleep(150 * time.Millisecond)
		writeOpenAITextDelta(w, "cmpl-1", "!")
		time.Sleep(150 * time.Millisecond)
		writeOpenAIDone(w)
	}))
	t.Cleanup(mock.Close)

	home := t.TempDir()
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

	data, _ := json.Marshal(map[string]any{
		"message":       "hi",
		"tool_protocol": "json",
		"tool_ids":      []string{},
	})
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
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		t.Fatalf("chat status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	sessionID := readSessionIDFromChatStream(t, resp.Body)
	// Simulate a refresh / disconnect.
	_ = resp.Body.Close()

	attachReq, err := http.NewRequest(http.MethodGet, srv.URL+"/api/sessions/"+sessionID+"/stream", nil)
	if err != nil {
		t.Fatalf("new attach request: %v", err)
	}
	attachReq.Header.Set("Authorization", "Bearer "+rt.AuthToken)

	attachResp, err := http.DefaultClient.Do(attachReq)
	if err != nil {
		t.Fatalf("attach request: %v", err)
	}
	defer attachResp.Body.Close()
	if attachResp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(attachResp.Body)
		t.Fatalf("attach status=%d body=%s", attachResp.StatusCode, strings.TrimSpace(string(b)))
	}

	got := readAssistantTextFromChat(t, attachResp.Body)
	if got != "Hello world!" {
		t.Fatalf("expected attached assistant text %q, got %q", "Hello world!", got)
	}

	msgs := waitUntilSessionHasAtLeastMessages(t, srv.URL, rt.AuthToken, sessionID, 2*time.Second, 2)
	if len(msgs) < 2 {
		t.Fatalf("expected persisted user+assistant messages, got %v", msgs)
	}
	last := msgs[len(msgs)-1]
	if last["role"] != "assistant" {
		t.Fatalf("expected last role=assistant, got %v", last["role"])
	}
	if last["content"] != "Hello world!" {
		t.Fatalf("expected persisted content %q, got %v", "Hello world!", last["content"])
	}
}

func TestE2E_Chat_StopDiscardsAssistantReply(t *testing.T) {
	t.Setenv("ONEAGENT_DISABLE_DAILY_LEARNING", "1")

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")

		parts := []string{"This", " should", " be", " discarded"}
		for i, p := range parts {
			select {
			case <-r.Context().Done():
				return
			default:
			}
			writeOpenAITextDelta(w, fmt.Sprintf("cmpl-%d", i+1), p)
			time.Sleep(200 * time.Millisecond)
		}
		writeOpenAIDone(w)
	}))
	t.Cleanup(mock.Close)

	home := t.TempDir()
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

	data, _ := json.Marshal(map[string]any{
		"message":       "hi",
		"tool_protocol": "json",
		"tool_ids":      []string{},
	})
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
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		t.Fatalf("chat status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	sessionID := readSessionIDFromChatStream(t, resp.Body)

	// Issue stop while the generation is in-flight.
	mustPostJSON(t, srv.URL, rt.AuthToken, "/api/sessions/"+sessionID+"/stop", map[string]any{}, nil)
	_ = resp.Body.Close()

	msgs := waitUntilSessionHasAtLeastMessages(t, srv.URL, rt.AuthToken, sessionID, 2*time.Second, 1)
	if msgs[0]["role"] != "user" {
		t.Fatalf("expected role=user, got %v", msgs[0]["role"])
	}
	if strings.TrimSpace(fmt.Sprint(msgs[0]["content"])) == "" {
		t.Fatalf("expected non-empty user content")
	}

	// Ensure no assistant message is persisted after stop (discard semantics).
	{
		ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
		t.Cleanup(cancel)
	waitLoop:
		for {
			select {
			case <-ctx.Done():
				break waitLoop
			default:
			}

			got, status := fetchSessionMessages(t, ctx, srv.URL, rt.AuthToken, sessionID)
			if status == http.StatusOK && len(got) > 1 {
				t.Fatalf("expected assistant reply discarded after stop, got messages=%v", got)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func TestE2E_Chat_AttachSnapshot_DoesNotIncludeFinishedXMLToolSteps(t *testing.T) {
	t.Setenv("ONEAGENT_DISABLE_DAILY_LEARNING", "1")

	step2Started := make(chan struct{})
	var step2Once sync.Once
	var callCount atomic.Int32

	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")

		switch callCount.Add(1) {
		case 1:
			writeOpenAITextDelta(w, "cmpl-1", "step-0 visible\n")
			writeOpenAITextDelta(w, "cmpl-1", "<tool_data><call><tool_name>does_not_exist</tool_name></call></tool_data>")
			writeOpenAIDone(w)
		case 2:
			step2Once.Do(func() { close(step2Started) })
			// Give the test time to attach while step-1 is in flight.
			time.Sleep(500 * time.Millisecond)
			writeOpenAITextDelta(w, "cmpl-2", "step-1 final\n")
			writeOpenAIDone(w)
		default:
			writeOpenAIDone(w)
		}
	}))
	t.Cleanup(mock.Close)

	home := t.TempDir()
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
	data, _ := json.Marshal(map[string]any{
		"message":       "hi",
		"tool_protocol": "xml",
		"tool_ids":      []string{tool.ToolIDRunCommand},
		"workspace":     workspace,
	})
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
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		t.Fatalf("chat status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	sessionID := readSessionIDFromChatStream(t, resp.Body)
	_ = resp.Body.Close()

	select {
	case <-step2Started:
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for xml tool loop to start step 2")
	}

	attachReq, err := http.NewRequest(http.MethodGet, srv.URL+"/api/sessions/"+sessionID+"/stream", nil)
	if err != nil {
		t.Fatalf("new attach request: %v", err)
	}
	attachReq.Header.Set("Authorization", "Bearer "+rt.AuthToken)

	attachResp, err := http.DefaultClient.Do(attachReq)
	if err != nil {
		t.Fatalf("attach request: %v", err)
	}
	defer attachResp.Body.Close()
	if attachResp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(attachResp.Body)
		t.Fatalf("attach status=%d body=%s", attachResp.StatusCode, strings.TrimSpace(string(b)))
	}

	startIDs := make([]string, 0, 2)
	scanner := bufio.NewScanner(attachResp.Body)
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
		var msg struct {
			Op      string `json:"op"`
			ID      string `json:"id"`
			MsgType string `json:"msg_type,omitempty"`
		}
		if err := json.Unmarshal([]byte(evt.Data), &msg); err != nil {
			continue
		}
		if msg.Op == "start" && msg.MsgType == "text" {
			startIDs = append(startIDs, msg.ID)
			if len(startIDs) >= 2 {
				break
			}
		}
		if msg.Op == "final" && msg.MsgType == "text" && len(startIDs) >= 1 {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan attach stream: %v", err)
	}
	if len(startIDs) != 1 {
		t.Fatalf("expected 1 active text stream in snapshot, got %v", startIDs)
	}
}
