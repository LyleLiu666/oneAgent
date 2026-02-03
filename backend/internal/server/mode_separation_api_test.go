package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/liu_y/oneAgent/backend/internal/config"
	oneruntime "github.com/liu_y/oneAgent/backend/internal/runtime"
)

type apiErrResp struct {
	Error     string `json:"error"`
	Code      string `json:"code"`
	Hint      string `json:"hint"`
	RequestID string `json:"request_id"`
}

func doJSON(t *testing.T, method, url, token string, body any) (*http.Response, []byte) {
	t.Helper()
	var r io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		r = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, url, r)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(token) != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	data, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return resp, data
}

func TestServer_ModeSeparation_SessionsAPIRejectsSecretarySession(t *testing.T) {
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

	// Bootstrap canonical secretary session.
	body := mustGet(t, srv.URL, rt.AuthToken, "/api/secretary/state")
	var st struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(body, &st); err != nil {
		t.Fatalf("unmarshal state: %v", err)
	}
	if strings.TrimSpace(st.SessionID) == "" {
		t.Fatalf("expected session_id in state")
	}

	resp, data := doJSON(t, http.MethodGet, srv.URL+"/api/sessions/"+st.SessionID, rt.AuthToken, nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var apiErr apiErrResp
	_ = json.Unmarshal(data, &apiErr)
	if apiErr.Code != "session_module_mismatch" {
		t.Fatalf("expected code=session_module_mismatch, got %+v", apiErr)
	}
	if strings.TrimSpace(apiErr.Hint) == "" {
		t.Fatalf("expected hint for mismatch, got %+v", apiErr)
	}
}

func TestServer_ModeSeparation_ChatAPIRejectsSecretarySession(t *testing.T) {
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
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(body, &st); err != nil {
		t.Fatalf("unmarshal state: %v", err)
	}
	if strings.TrimSpace(st.SessionID) == "" {
		t.Fatalf("expected session_id in state")
	}

	resp, data := doJSON(t, http.MethodPost, srv.URL+"/api/chat", rt.AuthToken, map[string]any{
		"message":    "hi",
		"session_id": st.SessionID,
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var apiErr apiErrResp
	_ = json.Unmarshal(data, &apiErr)
	if apiErr.Code != "session_module_mismatch" {
		t.Fatalf("expected code=session_module_mismatch, got %+v", apiErr)
	}
}

func TestServer_ModeSeparation_SecretaryInboxIgnoresAssistantSessionID(t *testing.T) {
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

	assistantSessionID := uuid.NewString()
	if _, err := rt.Sessions.GetOrCreateSession(assistantSessionID, "local", "assistant", "Assistant"); err != nil {
		t.Fatalf("create assistant session: %v", err)
	}

	var inboxResp struct {
		SessionID string `json:"session_id"`
		MessageID uint   `json:"message_id"`
	}
	mustPostJSON(t, srv.URL, rt.AuthToken, "/api/secretary/inbox/messages", map[string]any{
		"session_id": assistantSessionID,
		"content":    "你好",
	}, &inboxResp)

	if strings.TrimSpace(inboxResp.SessionID) == "" || inboxResp.MessageID == 0 {
		t.Fatalf("unexpected inbox response: %+v", inboxResp)
	}
	if inboxResp.SessionID == assistantSessionID {
		t.Fatalf("expected secretary inbox to ignore assistant session_id, got %q", inboxResp.SessionID)
	}

	// Assistant session must remain untouched.
	body := mustGet(t, srv.URL, rt.AuthToken, "/api/sessions/"+assistantSessionID)
	var sess struct {
		Messages []any `json:"messages"`
	}
	if err := json.Unmarshal(body, &sess); err != nil {
		t.Fatalf("unmarshal assistant session: %v", err)
	}
	if len(sess.Messages) != 0 {
		t.Fatalf("expected assistant session to have 0 messages, got %d", len(sess.Messages))
	}

	// Secretary session should contain the user message.
	resp, data := doJSON(t, http.MethodGet, srv.URL+"/api/secretary/session", rt.AuthToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var sec struct {
		ID       string `json:"id"`
		Messages []struct {
			Role    string `json:"role"`
			Type    string `json:"type"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(data, &sec); err != nil {
		t.Fatalf("unmarshal secretary session: %v", err)
	}
	if strings.TrimSpace(sec.ID) == "" || sec.ID != inboxResp.SessionID {
		t.Fatalf("expected secretary session id=%q, got %+v", inboxResp.SessionID, sec)
	}
	found := false
	for _, m := range sec.Messages {
		if m.Role == "user" && m.Type == "text" && strings.TrimSpace(m.Content) == "你好" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected secretary session to contain the user message, got %+v", sec.Messages)
	}
}
