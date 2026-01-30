package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liu_y/oneAgent/backend/internal/config"
	oneruntime "github.com/liu_y/oneAgent/backend/internal/runtime"
	"github.com/liu_y/oneAgent/backend/internal/tool"
)

func TestE2E_Skills_TurnContextInjection_ExplicitSkillName(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	prevCfg := config.AppConfig
	t.Cleanup(func() { config.AppConfig = prevCfg })

	// Seed a global Claude skill.
	skillPath := filepath.Join(home, ".claude", "skills", "code-review-excellence", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o700); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	if err := os.WriteFile(skillPath, []byte("---\nname: code-review-excellence\ndescription: review skill\n---\n"), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	cfg, err := config.Load(config.LoadOptions{
		Home:     t.TempDir(),
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

	var gotReq map[string]any
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotReq)

		w.Header().Set("Content-Type", "text/event-stream")
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

	reqBody := map[string]any{
		"message":       "请使用 code-review-excellence 这个技能帮我 review",
		"tool_protocol": "json",
		"tool_ids":      []string{tool.ToolIDSkillRead},
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
	// Drain the stream so the handler completes and the mock receives the request.
	_, _ = io.ReadAll(resp.Body)

	rawMsgs, _ := gotReq["messages"].([]any)
	if len(rawMsgs) < 2 {
		t.Fatalf("expected at least 2 messages, got %v", gotReq["messages"])
	}

	seen := false
	for _, raw := range rawMsgs {
		m, _ := raw.(map[string]any)
		content, _ := m["content"].(string)
		if strings.Contains(content, "## 技能建议") && strings.Contains(content, "skill_read") {
			seen = true
			break
		}
	}
	if !seen {
		t.Fatalf("expected skills turn context injected, got messages=%v", gotReq["messages"])
	}
}

func TestE2E_Skills_TurnContextInjection_SkipsIneligibleSkill(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	prevCfg := config.AppConfig
	t.Cleanup(func() { config.AppConfig = prevCfg })

	// Seed a global Claude skill with an unmet requirement.
	skillPath := filepath.Join(home, ".claude", "skills", "apple-notes", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o700); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	content := `---
name: apple-notes
description: Manage Apple Notes via memo
requires:
  bins: definitely-not-a-real-bin-12345
---
`
	if err := os.WriteFile(skillPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	cfg, err := config.Load(config.LoadOptions{
		Home:     t.TempDir(),
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

	var gotReq map[string]any
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotReq)

		w.Header().Set("Content-Type", "text/event-stream")
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

	reqBody := map[string]any{
		"message":       "请使用 apple-notes 这个技能帮我记一条 note",
		"tool_protocol": "json",
		"tool_ids":      []string{tool.ToolIDSkillRead},
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
	// Drain the stream so the handler completes and the mock receives the request.
	_, _ = io.ReadAll(resp.Body)

	rawMsgs, _ := gotReq["messages"].([]any)
	if len(rawMsgs) < 2 {
		t.Fatalf("expected at least 2 messages, got %v", gotReq["messages"])
	}

	seen := false
	for _, raw := range rawMsgs {
		m, _ := raw.(map[string]any)
		content, _ := m["content"].(string)
		if strings.Contains(content, "## 技能建议") && strings.Contains(content, "skill_read") {
			seen = true
			break
		}
	}
	if seen {
		t.Fatalf("expected no skills turn context injected for ineligible skill, got messages=%v", gotReq["messages"])
	}
}
